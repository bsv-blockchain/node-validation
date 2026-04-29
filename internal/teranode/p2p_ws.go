// Raw /p2p-ws WebSocket subscriber for the P2P service. Used by
// NEW-FR9 to observe rejected-tx events that aren't surfaced via
// the Centrifuge channels.
//
// Wire format per SP2 discovery (services/p2p/HandleWebsocket.go +
// services/p2p/server_helpers.go):
//
//	{"type":"block","hash":"...","height":N,...}
//	{"type":"subtree","hash":"..."}
//	{"type":"rejected_tx","tx_id":"...","reason":"..."}   (or "rejectedtx" — both accepted)
package teranode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RejectedTxEvent struct {
	Timestamp  string `json:"timestamp,omitempty"`
	Type       string `json:"type"`
	TxID       string `json:"tx_id,omitempty"`
	Reason     string `json:"reason,omitempty"`
	PeerID     string `json:"peer_id,omitempty"`
	ClientName string `json:"client_name,omitempty"`
}

type P2PBlockEvent struct {
	Type   string `json:"type"`
	Hash   string `json:"hash,omitempty"`
	Height uint64 `json:"height,omitempty"`
}

type P2PSubtreeEvent struct {
	Type string `json:"type"`
	Hash string `json:"hash,omitempty"`
}

// P2PWSClient subscribes to a Teranode P2P service's /p2p-ws endpoint.
type P2PWSClient struct {
	url    string
	logger *slog.Logger

	rejected chan RejectedTxEvent
	blocks   chan P2PBlockEvent
	subtrees chan P2PSubtreeEvent

	mu     sync.Mutex
	conn   *websocket.Conn
	closed bool
}

// NewP2PWSClient. Empty URL → (nil, nil) for nil-safety.
func NewP2PWSClient(rawURL string, logger *slog.Logger) (*P2PWSClient, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("teranode p2p-ws url %q: %w", rawURL, err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("teranode p2p-ws url %q: scheme must be ws or wss", rawURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("teranode p2p-ws url %q: missing host", rawURL)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &P2PWSClient{
		url:      rawURL,
		logger:   logger,
		rejected: make(chan RejectedTxEvent, 64),
		blocks:   make(chan P2PBlockEvent, 64),
		subtrees: make(chan P2PSubtreeEvent, 64),
	}, nil
}

// Connect dials the WebSocket and starts reading messages.
func (c *P2PWSClient) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	hdr := http.Header{}
	conn, _, err := dialer.DialContext(ctx, c.url, hdr)
	if err != nil {
		return fmt.Errorf("dial p2p-ws %s: %w", c.url, err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	go c.pump()
	return nil
}

// Close terminates the read loop and the WebSocket.
func (c *P2PWSClient) Close() error {
	c.mu.Lock()
	c.closed = true
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (c *P2PWSClient) RejectedTxs() <-chan RejectedTxEvent { return c.rejected }
func (c *P2PWSClient) Blocks() <-chan P2PBlockEvent        { return c.blocks }
func (c *P2PWSClient) Subtrees() <-chan P2PSubtreeEvent    { return c.subtrees }

func (c *P2PWSClient) pump() {
	for {
		c.mu.Lock()
		conn := c.conn
		closed := c.closed
		c.mu.Unlock()
		if closed || conn == nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !errors.Is(err, websocket.ErrCloseSent) {
				c.logger.Debug("p2p-ws read error", "err", err)
			}
			return
		}
		c.dispatch(data)
	}
}

// dispatch decodes one JSON message and routes by `type`. Unknown types
// are silently dropped.
func (c *P2PWSClient) dispatch(raw []byte) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		c.logger.Debug("p2p-ws dispatch: bad JSON", "err", err)
		return
	}
	switch probe.Type {
	case "rejected_tx", "rejectedtx", "rejected":
		var e RejectedTxEvent
		if err := json.Unmarshal(raw, &e); err == nil {
			select {
			case c.rejected <- e:
			default:
				c.logger.Warn("p2p-ws rejected channel full; dropping", "tx_id", e.TxID)
			}
		}
	case "block":
		var e P2PBlockEvent
		if err := json.Unmarshal(raw, &e); err == nil {
			select {
			case c.blocks <- e:
			default:
				c.logger.Warn("p2p-ws blocks channel full; dropping", "hash", e.Hash)
			}
		}
	case "subtree":
		var e P2PSubtreeEvent
		if err := json.Unmarshal(raw, &e); err == nil {
			select {
			case c.subtrees <- e:
			default:
				c.logger.Warn("p2p-ws subtrees channel full; dropping", "hash", e.Hash)
			}
		}
	}
}
