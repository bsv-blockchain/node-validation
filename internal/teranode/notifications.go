package teranode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge-go"
)

// NotificationClient subscribes to Teranode's Centrifuge channels:
// block, subtree, node_status. Discovery: docs/discovery.md §3.
type NotificationClient struct {
	url    string
	client *centrifuge.Client
	logger *slog.Logger

	blocks   chan BlockEvent
	subtrees chan SubtreeEvent
	statuses chan NodeStatusEvent

	mu        sync.Mutex
	connected bool
	closed    bool
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
	cl := centrifuge.NewJsonClient(rawURL, centrifuge.Config{})
	cl.OnConnected(func(_ centrifuge.ConnectedEvent) {
		nc.mu.Lock()
		nc.connected = true
		nc.mu.Unlock()
	})
	cl.OnDisconnected(func(_ centrifuge.DisconnectedEvent) {
		nc.mu.Lock()
		nc.connected = false
		nc.mu.Unlock()
	})
	cl.OnPublication(func(e centrifuge.ServerPublicationEvent) {
		nc.dispatch(e.Channel, e.Data)
	})
	nc.client = cl
	return nc, nil
}

func (c *NotificationClient) Connect(ctx context.Context) error {
	if c == nil {
		return errors.New("nil notification client")
	}
	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("centrifuge connect: %w", err)
	}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		c.mu.Lock()
		ok := c.connected
		c.mu.Unlock()
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("centrifuge connect timeout")
		case <-tick.C:
		}
	}
}

func (c *NotificationClient) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	_ = c.client.Disconnect()
	return nil
}

func (c *NotificationClient) Blocks() <-chan BlockEvent          { return c.blocks }
func (c *NotificationClient) Subtrees() <-chan SubtreeEvent      { return c.subtrees }
func (c *NotificationClient) NodeStatus() <-chan NodeStatusEvent { return c.statuses }

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
