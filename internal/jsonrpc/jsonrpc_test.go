package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newCaller(t *testing.T, url string) Caller {
	t.Helper()
	var id atomic.Int64
	return Caller{URL: url, HTTP: &http.Client{Timeout: 5 * time.Second}, IDSource: &id}
}

func TestCall_happyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "ping" || len(req.Params) != 1 || req.Params[0].(float64) != 7 {
			t.Errorf("unexpected request: %+v", req)
		}
		_, _ = w.Write([]byte(`{"result":"pong","error":null,"id":` + jsonInt(req.ID) + `}`))
	}))
	defer srv.Close()
	c := newCaller(t, srv.URL)
	var out string
	if err := c.Call(context.Background(), "ping", []any{7}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "pong" {
		t.Errorf("out: %q", out)
	}
}

func TestCall_basicAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"result":null,"error":null,"id":1}`))
	}))
	defer srv.Close()
	c := newCaller(t, srv.URL)
	c.User = "user"
	c.Pass = "pass"
	if err := c.Call(context.Background(), "x", nil, nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestCall_unauthorisedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := newCaller(t, srv.URL)
	if err := c.Call(context.Background(), "x", nil, nil); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("want 401 error, got %v", err)
	}
}

func TestCall_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":null,"error":{"code":-32601,"message":"method not found"},"id":1}`))
	}))
	defer srv.Close()
	c := newCaller(t, srv.URL)
	err := c.Call(context.Background(), "x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var rpcErr *Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32601 {
		t.Errorf("want RPC error code -32601, got %v", err)
	}
	if !IsErrorCode(err, -32601) {
		t.Error("IsErrorCode should be true")
	}
}

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestError_ErrorMethod(t *testing.T) {
	e := &Error{Code: -32600, Message: "invalid request"}
	got := e.Error()
	if !strings.Contains(got, "-32600") || !strings.Contains(got, "invalid request") {
		t.Errorf("Error(): %q", got)
	}
}

func TestIsErrorCode_NonRPCError(t *testing.T) {
	if IsErrorCode(errors.New("plain error"), -1) {
		t.Error("expected false for non-RPC error")
	}
}
