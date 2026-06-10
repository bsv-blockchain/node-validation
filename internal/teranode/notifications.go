package teranode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// NotificationClient subscribes to Teranode's Centrifuge channels:
// block, subtree, node_status. Discovery: docs/discovery.md §3.
//
// Transport note (root cause of the historical CLIENT-1/CLIENT-3 skips):
// Teranode's Asset Centrifuge endpoint is hardcoded as UNIDIRECTIONAL
// (services/asset/centrifuge_impl, Unidirectional()=true). On
// /connection/websocket the server accepts our connect command but replies
// with a Centrifuge *push*-format connect envelope that carries NO command
// id ({"connect":{"client":…,"subs":{…},"ping":25,"session":…}}), then it
// server-side-subscribes us and streams data pushes
// ({"channel":"block","pub":{"data":{…}}}). A bidirectional
// centrifuge-go client waits for a reply carrying id:1, never sees one, and
// times out after 15s. We therefore speak the wire protocol directly over a
// raw gorilla/websocket connection: send the connect command, treat the
// unsolicited connect push as "connected", and route the subscription pushes
// through dispatch(). The public API of this type is unchanged so the
// CLIENT-1/CLIENT-3 tests keep working across fresh Connect/Close cycles.
type NotificationClient struct {
	url    string
	logger *slog.Logger

	blocks   chan BlockEvent
	subtrees chan SubtreeEvent
	statuses chan NodeStatusEvent

	mu        sync.Mutex
	conn      *websocket.Conn
	connected bool
	closed    bool

	writeMu sync.Mutex
}

func NewNotificationClient(rawURL string, logger *slog.Logger) (*NotificationClient, error) {
	if rawURL == "" {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	nc := &NotificationClient{
		url:      rawURL,
		logger:   logger,
		blocks:   make(chan BlockEvent, 64),
		subtrees: make(chan SubtreeEvent, 64),
		statuses: make(chan NodeStatusEvent, 64),
	}
	return nc, nil
}

// wsEnvelope is the subset of the Centrifuge JSON protocol we parse. Frames
// may carry a reply (id set), a connect push (Connect set), or a data push
// (Channel + Pub set). An empty object {} is a server ping that must be
// answered with an empty pong to keep the bidirectional socket alive.
type wsEnvelope struct {
	ID      uint32          `json:"id,omitempty"`
	Connect *connectPush    `json:"connect,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Pub     *publicationMsg `json:"pub,omitempty"`
}

type connectPush struct {
	Client  string                     `json:"client"`
	Subs    map[string]json.RawMessage `json:"subs"`
	Ping    int                        `json:"ping"`
	Session string                     `json:"session"`
}

type publicationMsg struct {
	Data json.RawMessage `json:"data"`
}

// connectCommand is the bidirectional connect command we send first. The
// server accepts it even though it answers in unidirectional push format.
const connectCommand = `{"id":1,"connect":{}}`

func (c *NotificationClient) Connect(ctx context.Context) error {
	if c == nil {
		return errors.New("nil notification client")
	}
	if err := c.dialAndHandshake(ctx); err != nil {
		return err
	}
	go c.readLoop()
	return nil
}

// dialAndHandshake opens the websocket, sends the connect command, and blocks
// until the server's connect push arrives (or the deadline fires). It returns
// errors whose text preserves the "bad handshake" / "centrifuge connect
// timeout" markers the CLIENT tests historically branched on, so behaviour is
// stable if the transport ever regresses.
func (c *NotificationClient) dialAndHandshake(ctx context.Context) error {
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("centrifuge connect: %w (status %s)", err, resp.Status)
		}
		return fmt.Errorf("centrifuge connect: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(connectCommand)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("centrifuge connect write: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetReadDeadline(deadline)

	for {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return ctx.Err()
		default:
		}
		_, msg, rerr := conn.ReadMessage()
		if rerr != nil {
			_ = conn.Close()
			if errors.Is(rerr, context.Canceled) || errors.Is(rerr, context.DeadlineExceeded) {
				return rerr
			}
			return errors.New("centrifuge connect timeout: " + rerr.Error())
		}
		gotConnect := false
		for _, frame := range splitFrames(msg) {
			if isPing(frame) {
				c.writeRaw(conn, []byte("{}"))
				continue
			}
			var env wsEnvelope
			if json.Unmarshal(frame, &env) != nil {
				continue
			}
			if env.Connect != nil {
				gotConnect = true
				continue
			}
			// Data pushes can arrive in the same frame as / right after the
			// connect push; route them rather than dropping them.
			c.route(env)
		}
		if gotConnect {
			break
		}
	}

	_ = conn.SetReadDeadline(time.Time{})
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()
	return nil
}

// readLoop consumes server pushes for the life of the client, transparently
// reconnecting if the socket drops (unless the client was explicitly closed).
func (c *NotificationClient) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return
		}
		if conn == nil {
			if !c.reconnect() {
				return
			}
			continue
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			c.connected = false
			wasClosed := c.closed
			c.mu.Unlock()
			if wasClosed {
				return
			}
			_ = conn.Close()
			c.logger.Warn("notification stream read error; reconnecting", "err", err)
			if !c.reconnect() {
				return
			}
			continue
		}

		for _, frame := range splitFrames(msg) {
			if isPing(frame) {
				c.writeRaw(conn, []byte("{}"))
				continue
			}
			var env wsEnvelope
			if json.Unmarshal(frame, &env) != nil {
				continue
			}
			c.route(env)
		}
	}
}

// reconnect re-establishes the stream with capped exponential backoff. It
// returns false if the client was closed or all attempts are exhausted.
func (c *NotificationClient) reconnect() bool {
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < 12; attempt++ {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := c.dialAndHandshake(ctx)
		cancel()
		if err == nil {
			c.logger.Info("notification stream reconnected")
			return true
		}
		time.Sleep(backoff)
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
	return false
}

func (c *NotificationClient) route(env wsEnvelope) {
	if env.Channel != "" && env.Pub != nil {
		c.dispatch(env.Channel, env.Pub.Data)
	}
}

func (c *NotificationClient) writeRaw(conn *websocket.Conn, data []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (c *NotificationClient) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.connected = false
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	return nil
}

func (c *NotificationClient) Blocks() <-chan BlockEvent          { return c.blocks }
func (c *NotificationClient) Subtrees() <-chan SubtreeEvent      { return c.subtrees }
func (c *NotificationClient) NodeStatus() <-chan NodeStatusEvent { return c.statuses }

// splitFrames splits a websocket message into individual Centrifuge JSON
// frames. The JSON protocol batches multiple commands/pushes in a single
// websocket message separated by newlines.
func splitFrames(msg []byte) [][]byte {
	parts := bytes.Split(msg, []byte("\n"))
	out := parts[:0]
	for _, p := range parts {
		if len(bytes.TrimSpace(p)) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// isPing reports whether a frame is the Centrifuge empty-object server ping.
func isPing(frame []byte) bool {
	return bytes.Equal(bytes.TrimSpace(frame), []byte("{}"))
}

func (c *NotificationClient) dispatch(channel string, data []byte) {
	switch channel {
	case "block":
		var e BlockEvent
		if err := json.Unmarshal(data, &e); err == nil {
			select {
			case c.blocks <- e:
			default:
				c.logger.Warn("block channel full; dropping event", "hash", e.Hash)
			}
		}
	case "subtree":
		var e SubtreeEvent
		if err := json.Unmarshal(data, &e); err == nil {
			select {
			case c.subtrees <- e:
			default:
				c.logger.Warn("subtree channel full; dropping event", "hash", e.Hash)
			}
		}
	case "node_status":
		var e NodeStatusEvent
		if err := json.Unmarshal(data, &e); err == nil {
			select {
			case c.statuses <- e:
			default:
				c.logger.Warn("node_status channel full; dropping event", "peer_id", e.PeerID)
			}
		}
	}
}

// BlockEvent matches the JSON shape from services/p2p/server_helpers.go:48-57.
type BlockEvent struct {
	Timestamp  string `json:"timestamp"`
	Type       string `json:"type"`
	Hash       string `json:"hash"`
	Height     uint64 `json:"height"`
	BaseURL    string `json:"base_url"`
	PeerID     string `json:"peer_id"`
	ClientName string `json:"client_name"`
}

// SubtreeEvent matches services/p2p/server_helpers.go:170-178.
type SubtreeEvent struct {
	Timestamp  string `json:"timestamp"`
	Type       string `json:"type"`
	Hash       string `json:"hash"`
	BaseURL    string `json:"base_url"`
	PeerID     string `json:"peer_id"`
	ClientName string `json:"client_name"`
}

// NodeStatusEvent matches services/p2p/Server.go:929-956.
type NodeStatusEvent struct {
	Type          string  `json:"type"`
	PeerID        string  `json:"peer_id"`
	Version       string  `json:"version"`
	BestBlockHash string  `json:"best_block_hash"`
	BestHeight    uint64  `json:"best_height"`
	TxCount       uint64  `json:"tx_count"`
	SubtreeCount  uint64  `json:"subtree_count"`
	FSMState      string  `json:"fsm_state"`
	Uptime        float64 `json:"uptime"`
	ClientName    string  `json:"client_name"`
	MinerName     string  `json:"miner_name"`
}
