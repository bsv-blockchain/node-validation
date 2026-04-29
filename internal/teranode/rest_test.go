package teranode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRESTStub(t *testing.T, paths map[string]struct {
	status int
	body   string
}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		full := r.URL.Path
		if r.URL.RawQuery != "" {
			full += "?" + r.URL.RawQuery
		}
		entry, ok := paths[full]
		if !ok {
			http.Error(w, "not in stub: "+full, http.StatusNotFound)
			return
		}
		w.WriteHeader(entry.status)
		_, _ = w.Write([]byte(entry.body))
	}))
}

func TestREST_GetTxBytes(t *testing.T) {
	srv := newRESTStub(t, map[string]struct {
		status int
		body   string
	}{"/api/v1/tx/abc": {200, "binary-bytes"}})
	defer srv.Close()
	c, err := NewRESTClient(srv.URL+"/api/v1", nil)
	if err != nil {
		t.Fatalf("NewRESTClient: %v", err)
	}
	b, err := c.GetTxBytes(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetTxBytes: %v", err)
	}
	if string(b) != "binary-bytes" {
		t.Errorf("body: %q", b)
	}
}

func TestREST_404IsRESTError(t *testing.T) {
	srv := newRESTStub(t, map[string]struct {
		status int
		body   string
	}{})
	defer srv.Close()
	c, _ := NewRESTClient(srv.URL+"/api/v1", nil)
	_, err := c.GetTxBytes(context.Background(), "nope")
	var rerr *RESTError
	if !errors.As(err, &rerr) || rerr.Status != http.StatusNotFound {
		t.Fatalf("want RESTError 404, got %v", err)
	}
}

func TestREST_NilOnEmptyURL(t *testing.T) {
	c, err := NewRESTClient("", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}

func TestREST_SearchEncodesQuery(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c, _ := NewRESTClient(srv.URL+"/api/v1", nil)
	if _, err := c.Search(context.Background(), "1234 abc"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(seen, "search?q=1234+abc") && !strings.Contains(seen, "search?q=1234%20abc") {
		t.Errorf("query encoding: %q", seen)
	}
}

func TestREST_ListBlocksPagination(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c, _ := NewRESTClient(srv.URL+"/api/v1", nil)
	if _, err := c.ListBlocks(context.Background(), 10, 5); err != nil {
		t.Fatalf("ListBlocks: %v", err)
	}
	if !strings.Contains(seen, "offset=10") || !strings.Contains(seen, "limit=5") {
		t.Errorf("query: %q", seen)
	}
}

func TestREST_WrapperMethods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c, _ := NewRESTClient(srv.URL+"/api/v1", nil)
	ctx := context.Background()
	if _, err := c.GetTxJSON(ctx, "abc"); err != nil {
		t.Errorf("GetTxJSON: %v", err)
	}
	if _, err := c.GetBlockBytes(ctx, "abc"); err != nil {
		t.Errorf("GetBlockBytes: %v", err)
	}
	if _, err := c.GetBlockJSON(ctx, "abc"); err != nil {
		t.Errorf("GetBlockJSON: %v", err)
	}
	if _, err := c.GetBlockHeaderBytes(ctx, "abc"); err != nil {
		t.Errorf("GetBlockHeaderBytes: %v", err)
	}
	if _, err := c.GetBlockHeaderJSON(ctx, "abc"); err != nil {
		t.Errorf("GetBlockHeaderJSON: %v", err)
	}
	if _, err := c.GetBestBlockHeaderJSON(ctx); err != nil {
		t.Errorf("GetBestBlockHeaderJSON: %v", err)
	}
	if _, err := c.GetUTXOJSON(ctx, "abc:0"); err != nil {
		t.Errorf("GetUTXOJSON: %v", err)
	}
}

func TestREST_ErrorMethodAndTruncate(t *testing.T) {
	e := &RESTError{Status: 503, Path: "/api/v1/tx/x", Body: strings.Repeat("x", 200)}
	msg := e.Error()
	if !strings.Contains(msg, "503") {
		t.Errorf("error missing status: %s", msg)
	}
	// truncate should kick in at 160 chars
	if !strings.Contains(msg, "...") {
		t.Errorf("expected truncation in: %s", msg)
	}
}
