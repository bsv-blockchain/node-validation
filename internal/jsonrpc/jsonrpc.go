// Package jsonrpc provides shared JSON-RPC 1.0 framing used by both the
// Teranode and SV Node RPC clients. Both backends speak the same wire
// format (request envelope, response shape, Bitcoin-style error codes).
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// Request is the JSON-RPC 1.0 request envelope.
type Request struct {
	JSONRPC string `json:"jsonrpc,omitempty"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int64  `json:"id"`
}

// Response is the JSON-RPC 1.0 response shape.
type Response struct {
	Result json.RawMessage `json:"result"`
	Error  *Error          `json:"error"`
	ID     int64           `json:"id"`
}

// Error is the typed JSON-RPC error. Tests can branch via errors.As.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message) }

// Caller carries the per-client state needed to issue a Call.
type Caller struct {
	URL      string
	User     string
	Pass     string
	HTTP     *http.Client
	IDSource *atomic.Int64
}

// Call issues one JSON-RPC request and decodes the result into out.
// Returns *Error if the server returned a JSON-RPC error; a network or
// decoding error otherwise.
func (c Caller) Call(ctx context.Context, method string, params []any, out any) error {
	if params == nil {
		params = []any{}
	}
	req := Request{Method: method, Params: params, ID: c.IDSource.Add(1)}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.User != "" || c.Pass != "" {
		httpReq.SetBasicAuth(c.User, c.Pass)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http %s %s: %w", method, c.URL, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized (HTTP 401) for method %s", method)
	}
	var parsed Response
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("decode response (status %d, body %q): %w", resp.StatusCode, string(respBody), err)
	}
	if parsed.Error != nil {
		return parsed.Error
	}
	if out != nil {
		if err := json.Unmarshal(parsed.Result, out); err != nil {
			return fmt.Errorf("decode result for %s: %w", method, err)
		}
	}
	return nil
}

// IsErrorCode is a helper for branching on a specific RPC error code.
func IsErrorCode(err error, code int) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}
