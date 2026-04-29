package svnode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newRPCStub(t *testing.T, fn func(method string, params []any) any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
			ID     int64  `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": fn(req.Method, req.Params), "error": nil, "id": req.ID})
	}))
}

func TestRPC_GetBestBlockHash(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		if method != "getbestblockhash" {
			t.Errorf("method: %s", method)
		}
		return "feedface"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	h, err := c.GetBestBlockHash(context.Background())
	if err != nil {
		t.Fatalf("GetBestBlockHash: %v", err)
	}
	if h != "feedface" {
		t.Errorf("h: %q", h)
	}
}

func TestRPC_TestMempoolAccept(t *testing.T) {
	srv := newRPCStub(t, func(method string, params []any) any {
		if method != "testmempoolaccept" {
			t.Errorf("method: %s", method)
		}
		return []map[string]any{{"txid": "abc", "allowed": true}}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	raw, err := c.TestMempoolAccept(context.Background(), []string{"01..."})
	if err != nil {
		t.Fatalf("TestMempoolAccept: %v", err)
	}
	if len(raw) == 0 {
		t.Errorf("empty result")
	}
}

func TestRPC_NilOnEmptyURL(t *testing.T) {
	c, err := NewRPCClient("", "", "", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}

func TestRPC_CookieReaderNocrash(t *testing.T) {
	// Exercise readCookie with no bitcoind installed — must not panic or error.
	u, p, ok := readCookie()
	// ok may be true or false depending on whether ~/.bitcoin/.cookie exists.
	// What matters is no crash.
	_ = u
	_ = p
	_ = ok
}

func TestRPC_Call(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return "pong"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	var out string
	if err := c.Call(context.Background(), "ping", nil, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestRPC_GetBlock(t *testing.T) {
	srv := newRPCStub(t, func(method string, params []any) any {
		if method != "getblock" {
			t.Errorf("method: %s", method)
		}
		return map[string]any{"hash": "abc"}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	if _, err := c.GetBlock(context.Background(), "abc", 1); err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
}

func TestRPC_GetBlockHeader(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return map[string]any{"hash": "abc"}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	if _, err := c.GetBlockHeader(context.Background(), "abc", false); err != nil {
		t.Fatalf("GetBlockHeader: %v", err)
	}
}

func TestRPC_GetBlockHash(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return "hashval"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	h, err := c.GetBlockHash(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetBlockHash: %v", err)
	}
	if h != "hashval" {
		t.Errorf("h: %q", h)
	}
}

func TestRPC_GetBlockchainInfo(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return map[string]any{"chain": "test"}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	if _, err := c.GetBlockchainInfo(context.Background()); err != nil {
		t.Fatalf("GetBlockchainInfo: %v", err)
	}
}

func TestRPC_GetRawTransaction(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return "rawtx"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	if _, err := c.GetRawTransaction(context.Background(), "abc", 0); err != nil {
		t.Fatalf("GetRawTransaction: %v", err)
	}
}

func TestRPC_GetRawMempool(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return []string{"txid1"}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	ids, err := c.GetRawMempool(context.Background())
	if err != nil {
		t.Fatalf("GetRawMempool: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("len: %d", len(ids))
	}
}

func TestRPC_SendRawTransaction(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return "newtxid"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	id, err := c.SendRawTransaction(context.Background(), "0100...")
	if err != nil {
		t.Fatalf("SendRawTransaction: %v", err)
	}
	if id != "newtxid" {
		t.Errorf("id: %q", id)
	}
}

func TestRPC_GetMempoolInfo(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return map[string]any{"size": 0}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	if _, err := c.GetMempoolInfo(context.Background()); err != nil {
		t.Fatalf("GetMempoolInfo: %v", err)
	}
}

func TestRPC_EstimateFee(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return 0.0001
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	f, err := c.EstimateFee(context.Background(), 6)
	if err != nil {
		t.Fatalf("EstimateFee: %v", err)
	}
	if f != 0.0001 {
		t.Errorf("f: %v", f)
	}
}
