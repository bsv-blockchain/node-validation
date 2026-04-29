package teranode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RESTClient talks to the Teranode Asset HTTP API.
// Discovery: docs/discovery.md §2.
type RESTClient struct {
	base   *url.URL
	http   *http.Client
	logger *slog.Logger
}

// NewRESTClient constructs a RESTClient. The rawURL must include any
// route prefix (e.g. "http://host:8090/api/v1"). Empty rawURL → (nil, nil).
func NewRESTClient(rawURL string, logger *slog.Logger) (*RESTClient, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("teranode rest url %q: %w", rawURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("teranode rest url %q: missing scheme or host", rawURL)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RESTClient{base: u, http: &http.Client{Timeout: 30 * time.Second}, logger: logger}, nil
}

// RESTError carries the HTTP status code so callers can branch via errors.As.
type RESTError struct {
	Status int
	Path   string
	Body   string
}

func (e *RESTError) Error() string {
	return fmt.Sprintf("teranode rest %s: HTTP %d (%s)", e.Path, e.Status, truncate(e.Body, 160))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (c *RESTClient) get(ctx context.Context, p string) ([]byte, error) {
	full := *c.base
	pathPart, queryPart, _ := strings.Cut(p, "?")
	full.Path = strings.TrimRight(full.Path, "/") + "/" + strings.TrimLeft(pathPart, "/")
	if queryPart != "" {
		full.RawQuery = queryPart
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", full.String(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &RESTError{Status: resp.StatusCode, Path: full.Path, Body: string(body)}
	}
	return body, nil
}

func (c *RESTClient) GetTxBytes(ctx context.Context, hash string) ([]byte, error) {
	return c.get(ctx, "tx/"+hash)
}
func (c *RESTClient) GetTxJSON(ctx context.Context, hash string) (json.RawMessage, error) {
	b, err := c.get(ctx, "tx/"+hash+"/json")
	return json.RawMessage(b), err
}
func (c *RESTClient) GetBlockBytes(ctx context.Context, hash string) ([]byte, error) {
	return c.get(ctx, "block/"+hash)
}
func (c *RESTClient) GetBlockJSON(ctx context.Context, hash string) (json.RawMessage, error) {
	b, err := c.get(ctx, "block/"+hash+"/json")
	return json.RawMessage(b), err
}
func (c *RESTClient) GetBlockHeaderBytes(ctx context.Context, hash string) ([]byte, error) {
	return c.get(ctx, "header/"+hash)
}
func (c *RESTClient) GetBlockHeaderJSON(ctx context.Context, hash string) (json.RawMessage, error) {
	b, err := c.get(ctx, "header/"+hash+"/json")
	return json.RawMessage(b), err
}
func (c *RESTClient) GetBestBlockHeaderJSON(ctx context.Context) (json.RawMessage, error) {
	b, err := c.get(ctx, "bestblockheader/json")
	return json.RawMessage(b), err
}
func (c *RESTClient) GetUTXOJSON(ctx context.Context, utxoHash string) (json.RawMessage, error) {
	b, err := c.get(ctx, "utxo/"+utxoHash+"/json")
	return json.RawMessage(b), err
}
func (c *RESTClient) Search(ctx context.Context, q string) (json.RawMessage, error) {
	b, err := c.get(ctx, "search?q="+url.QueryEscape(q))
	return json.RawMessage(b), err
}
func (c *RESTClient) ListBlocks(ctx context.Context, offset, limit int) (json.RawMessage, error) {
	b, err := c.get(ctx, fmt.Sprintf("blocks?offset=%d&limit=%d", offset, limit))
	return json.RawMessage(b), err
}
