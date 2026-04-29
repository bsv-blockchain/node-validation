package teranode

import (
	"encoding/json"
	"testing"
)

func TestNotifications_NilOnEmptyURL(t *testing.T) {
	c, err := NewNotificationClient("", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}

func TestNotifications_DispatchBlock(t *testing.T) {
	c, _ := NewNotificationClient("ws://example.invalid/connection/websocket", nil)
	if c == nil {
		t.Fatal("client unexpectedly nil")
	}
	payload, _ := json.Marshal(BlockEvent{
		Type: "block", Hash: "abc", Height: 99, BaseURL: "http://x", PeerID: "p", ClientName: "test",
	})
	c.dispatch("block", payload)
	select {
	case e := <-c.Blocks():
		if e.Hash != "abc" || e.Height != 99 {
			t.Errorf("block: %+v", e)
		}
	default:
		t.Fatal("no block delivered")
	}
}

func TestNotifications_DispatchSubtree(t *testing.T) {
	c, _ := NewNotificationClient("ws://example.invalid/connection/websocket", nil)
	payload, _ := json.Marshal(SubtreeEvent{Type: "subtree", Hash: "def", PeerID: "p"})
	c.dispatch("subtree", payload)
	select {
	case e := <-c.Subtrees():
		if e.Hash != "def" {
			t.Errorf("subtree: %+v", e)
		}
	default:
		t.Fatal("no subtree delivered")
	}
}

func TestNotifications_DispatchNodeStatus(t *testing.T) {
	c, _ := NewNotificationClient("ws://example.invalid/connection/websocket", nil)
	payload, _ := json.Marshal(NodeStatusEvent{Type: "node_status", PeerID: "p", BestHeight: 42})
	c.dispatch("node_status", payload)
	select {
	case e := <-c.NodeStatus():
		if e.BestHeight != 42 {
			t.Errorf("status: %+v", e)
		}
	default:
		t.Fatal("no node_status delivered")
	}
}

func TestNotifications_UnknownChannelIgnored(t *testing.T) {
	c, _ := NewNotificationClient("ws://example.invalid/connection/websocket", nil)
	c.dispatch("ping", []byte(`{}`))
	c.dispatch("mining_on", []byte(`{}`))
	// No assertions — just confirm dispatch doesn't panic on unknown channels.
}
