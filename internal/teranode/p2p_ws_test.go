package teranode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewP2PWSClient_NilOnEmptyURL(t *testing.T) {
	c, err := NewP2PWSClient("", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}

func TestNewP2PWSClient_RejectsHTTP(t *testing.T) {
	if _, err := NewP2PWSClient("http://x/p2p-ws", nil); err == nil {
		t.Fatal("want error for http scheme")
	}
}

func TestP2PWS_DispatchRejectedTx(t *testing.T) {
	c, _ := NewP2PWSClient("ws://x/p2p-ws", nil)
	payload, _ := json.Marshal(RejectedTxEvent{Type: "rejected_tx", TxID: "abc", Reason: "spent"})
	c.dispatch(payload)
	select {
	case e := <-c.RejectedTxs():
		if e.TxID != "abc" || e.Reason != "spent" {
			t.Errorf("event: %+v", e)
		}
	default:
		t.Fatal("no event delivered")
	}
}

func TestP2PWS_DispatchAlternateTypeName(t *testing.T) {
	c, _ := NewP2PWSClient("ws://x/p2p-ws", nil)
	c.dispatch([]byte(`{"type":"rejectedtx","tx_id":"def","reason":"conflict"}`))
	select {
	case e := <-c.RejectedTxs():
		if e.TxID != "def" {
			t.Errorf("event: %+v", e)
		}
	default:
		t.Fatal("no event delivered")
	}
}

func TestP2PWS_DispatchBlock(t *testing.T) {
	c, _ := NewP2PWSClient("ws://x/p2p-ws", nil)
	payload, _ := json.Marshal(P2PBlockEvent{Type: "block", Hash: "h1", Height: 42})
	c.dispatch(payload)
	select {
	case e := <-c.Blocks():
		if e.Height != 42 {
			t.Errorf("event: %+v", e)
		}
	default:
		t.Fatal("no event delivered")
	}
}

func TestP2PWS_LiveConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"type":"rejected_tx","tx_id":"livetx","reason":"spent"}`))
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/p2p-ws"
	c, err := NewP2PWSClient(wsURL, nil)
	if err != nil {
		t.Fatalf("NewP2PWSClient: %v", err)
	}
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
	select {
	case e := <-c.RejectedTxs():
		if e.TxID != "livetx" {
			t.Errorf("event: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event in 2s")
	}
}
