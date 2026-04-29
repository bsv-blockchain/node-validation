package teranode

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
		result := fn(req.Method, req.Params)
		out := map[string]any{"result": result, "error": nil, "id": req.ID}
		_ = json.NewEncoder(w).Encode(out)
	}))
}

func TestRPC_GetBestBlockHash(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		if method != "getbestblockhash" {
			t.Errorf("method: %s", method)
		}
		return "deadbeef"
	})
	defer srv.Close()
	c, err := NewRPCClient(srv.URL, "", "", nil)
	if err != nil {
		t.Fatalf("NewRPCClient: %v", err)
	}
	hash, err := c.GetBestBlockHash(context.Background())
	if err != nil {
		t.Fatalf("GetBestBlockHash: %v", err)
	}
	if hash != "deadbeef" {
		t.Errorf("hash: %q", hash)
	}
}

func TestRPC_GetBlockchainInfoDecoded(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		return map[string]any{"chain": "test", "blocks": 100, "headers": 100, "bestblockhash": "feedface", "difficulty": 1.0}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "", "", nil)
	info, err := c.GetBlockchainInfo(context.Background())
	if err != nil {
		t.Fatalf("GetBlockchainInfo: %v", err)
	}
	if info.Chain != "test" || info.Blocks != 100 || info.BestBlockHash != "feedface" {
		t.Errorf("info: %+v", info)
	}
}

func TestRPC_NilOnEmptyURL(t *testing.T) {
	c, err := NewRPCClient("", "", "", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}

func TestRPC_GetBlockPassesParams(t *testing.T) {
	srv := newRPCStub(t, func(method string, params []any) any {
		if method != "getblock" || len(params) != 2 || params[0].(string) != "abc" || params[1].(float64) != 1 {
			t.Errorf("got method=%s params=%v", method, params)
		}
		return map[string]any{"hash": "abc", "height": 0}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "", "", nil)
	if _, err := c.GetBlock(context.Background(), "abc", 1); err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
}

func TestRPC_SendRawTransaction(t *testing.T) {
	srv := newRPCStub(t, func(method string, params []any) any {
		if method != "sendrawtransaction" || len(params) != 1 {
			t.Errorf("method=%s params=%v", method, params)
		}
		return "newtxid"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "", "", nil)
	id, err := c.SendRawTransaction(context.Background(), "0100...")
	if err != nil {
		t.Fatalf("SendRawTransaction: %v", err)
	}
	if id != "newtxid" {
		t.Errorf("id: %q", id)
	}
}
