package teranode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
)

// RPCClient is a Teranode JSON-RPC 1.0 client (port 9292 by default,
// HTTP Basic Auth — discovery: docs/discovery.md §1).
type RPCClient struct {
	caller jsonrpc.Caller
	logger *slog.Logger
}

// NewRPCClient constructs an RPCClient. An empty rawURL returns (nil, nil)
// so callers can skip cleanly when the endpoint is not configured.
func NewRPCClient(rawURL, user, pass string, logger *slog.Logger) (*RPCClient, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("teranode rpc url %q: %w", rawURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("teranode rpc url %q: missing scheme or host", rawURL)
	}
	if logger == nil {
		logger = slog.Default()
	}
	var id atomic.Int64
	return &RPCClient{
		caller: jsonrpc.Caller{
			URL:      rawURL,
			User:     user,
			Pass:     pass,
			HTTP:     &http.Client{Timeout: 30 * time.Second},
			IDSource: &id,
		},
		logger: logger,
	}, nil
}

// Call is a passthrough to jsonrpc.Caller for arbitrary methods.
func (c *RPCClient) Call(ctx context.Context, method string, params []any, out any) error {
	return c.caller.Call(ctx, method, params, out)
}

// BlockchainInfo is the trimmed shape of getblockchaininfo we care about.
type BlockchainInfo struct {
	Chain         string  `json:"chain"`
	Blocks        int64   `json:"blocks"`
	Headers       int64   `json:"headers"`
	BestBlockHash string  `json:"bestblockhash"`
	Difficulty    float64 `json:"difficulty"`
}

func (c *RPCClient) GetBestBlockHash(ctx context.Context) (string, error) {
	var s string
	return s, c.caller.Call(ctx, "getbestblockhash", nil, &s)
}

func (c *RPCClient) GetBlock(ctx context.Context, hash string, verbosity uint32) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getblock", []any{hash, verbosity}, &raw)
}

func (c *RPCClient) GetBlockHeader(ctx context.Context, hash string, verbose bool) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getblockheader", []any{hash, verbose}, &raw)
}

func (c *RPCClient) GetBlockHash(ctx context.Context, height int64) (string, error) {
	var s string
	return s, c.caller.Call(ctx, "getblockhash", []any{height}, &s)
}

func (c *RPCClient) GetBlockchainInfo(ctx context.Context) (BlockchainInfo, error) {
	var info BlockchainInfo
	return info, c.caller.Call(ctx, "getblockchaininfo", nil, &info)
}

func (c *RPCClient) GetRawTransaction(ctx context.Context, txid string, verbose int) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getrawtransaction", []any{txid, verbose}, &raw)
}

func (c *RPCClient) GetRawMempool(ctx context.Context) ([]string, error) {
	var ids []string
	return ids, c.caller.Call(ctx, "getrawmempool", nil, &ids)
}

func (c *RPCClient) SendRawTransaction(ctx context.Context, hexTx string) (string, error) {
	var s string
	return s, c.caller.Call(ctx, "sendrawtransaction", []any{hexTx}, &s)
}

func (c *RPCClient) GetMiningInfo(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getmininginfo", nil, &raw)
}

func (c *RPCClient) GetPeerInfo(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getpeerinfo", nil, &raw)
}

func (c *RPCClient) GetChainTips(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getchaintips", nil, &raw)
}

func (c *RPCClient) Version(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "version", nil, &raw)
}
