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
