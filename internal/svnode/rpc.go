// internal/svnode/rpc.go
package svnode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
)

// RPCClient is a bitcoind-compatible JSON-RPC 1.0 client. Wire shape
// matches Teranode's RPC; the method set differs (SV Node has
// estimatefee, getmempoolinfo, testmempoolaccept which Teranode lacks).
type RPCClient struct {
	caller jsonrpc.Caller
	logger *slog.Logger
}

// NewRPCClient. If both user and pass are empty, attempts to read
// "user:pass" from the cookie file at $HOME/.bitcoin/.cookie (the
// bitcoind convention). An empty rawURL returns (nil, nil).
func NewRPCClient(rawURL, user, pass string, logger *slog.Logger) (*RPCClient, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("svnode rpc url %q: %w", rawURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("svnode rpc url %q: missing scheme or host", rawURL)
	}
	if user == "" && pass == "" {
		if cu, cp, ok := readCookie(); ok {
			user, pass = cu, cp
		}
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

// readCookie returns user, pass, ok from the bitcoind cookie file.
func readCookie() (string, string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", false
	}
	for _, p := range []string{
		filepath.Join(home, ".bitcoin", ".cookie"),
		filepath.Join(home, ".bitcoin", "testnet3", ".cookie"),
		filepath.Join(home, ".bitcoin", "regtest", ".cookie"),
	} {
		if b, err := os.ReadFile(p); err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(b)), ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], true
			}
		}
	}
	return "", "", false
}

func (c *RPCClient) Call(ctx context.Context, method string, params []any, out any) error {
	return c.caller.Call(ctx, method, params, out)
}

// Convenience wrappers matching the Teranode RPC client surface.
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
func (c *RPCClient) GetBlockchainInfo(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getblockchaininfo", nil, &raw)
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
func (c *RPCClient) GetMempoolInfo(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "getmempoolinfo", nil, &raw)
}
func (c *RPCClient) EstimateFee(ctx context.Context, numBlocks int64) (float64, error) {
	var f float64
	return f, c.caller.Call(ctx, "estimatefee", []any{numBlocks}, &f)
}
func (c *RPCClient) TestMempoolAccept(ctx context.Context, hexTxs []string) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.caller.Call(ctx, "testmempoolaccept", []any{hexTxs}, &raw)
}
