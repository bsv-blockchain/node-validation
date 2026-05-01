package tests

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/observer"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

// mempoolReader is satisfied by both *teranode.RPCClient and *svnode.RPCClient,
// which both expose GetRawMempool(ctx) ([]string, error) with the same shape.
type mempoolReader interface {
	GetRawMempool(ctx context.Context) ([]string, error)
}

// Compile-time interface satisfaction check.
var _ mempoolReader = (*teranode.RPCClient)(nil)

// pollMempoolUntil polls rpc.GetRawMempool every 250ms until all wantTxIDs
// are present or the timeout passes. Returns the set of txids that were
// observed (subset of wantTxIDs) and whether the full set was matched.
//
// Usable for both teranode.RPCClient and svnode.RPCClient — both expose
// GetRawMempool() ([]string, error) with the same shape.
func pollMempoolUntil(ctx context.Context, rpc mempoolReader, wantTxIDs []string, timeout time.Duration) (seen map[string]bool, allSeen bool) {
	seen = make(map[string]bool, len(wantTxIDs))
	want := make(map[string]bool, len(wantTxIDs))
	for _, id := range wantTxIDs {
		want[id] = true
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		mempool, err := rpc.GetRawMempool(ctx)
		if err == nil {
			for _, id := range mempool {
				if want[id] {
					seen[id] = true
				}
			}
			if len(seen) == len(want) {
				return seen, true
			}
		}
		select {
		case <-ctx.Done():
			return seen, false
		case <-ticker.C:
		}
	}
	return seen, false
}

// ok returns a passing acceptance check.
func ok(desc, detail string) testrunner.Check {
	return testrunner.Check{Description: desc, Required: true, Pass: true, Detail: detail}
}

// fail returns a failing acceptance check.
func fail(desc, detail string) testrunner.Check {
	return testrunner.Check{Description: desc, Required: true, Pass: false, Detail: detail}
}

// required builds a Check from a boolean.
func required(desc string, pass bool, detail string) testrunner.Check {
	return testrunner.Check{Description: desc, Required: true, Pass: pass, Detail: detail}
}

// skipMissing returns a SKIPPED Result populated with the given reason.
// The caller passes a partially-built Result with ID/Title/Severity already set.
func skipMissing(res testrunner.Result, reason string) testrunner.Result {
	res.Status = testrunner.StatusSkipped
	res.SkipReason = reason
	return res
}

// errorResult marks res as ERROR and stores err.
func errorResult(res testrunner.Result, err error) testrunner.Result {
	res.Status = testrunner.StatusError
	res.Err = err.Error()
	return res
}

// waitForMempoolEntries polls rpc.GetRawMempool every 500ms until all
// wantTxIDs are observed in the mempool, or the timeout passes.
func waitForMempoolEntries(ctx context.Context, rpc mempoolReader, wantTxIDs []string, timeout time.Duration) error {
	want := map[string]bool{}
	for _, id := range wantTxIDs {
		want[id] = true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mempool, err := rpc.GetRawMempool(ctx)
		if err == nil {
			seen := 0
			present := map[string]bool{}
			for _, id := range mempool {
				if want[id] {
					present[id] = true
				}
			}
			for id := range want {
				if present[id] {
					seen++
				}
			}
			if seen == len(want) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("only some of %d expected txs reached mempool within %v", len(wantTxIDs), timeout)
}

// deriveStatus computes Status from the acceptance checks. Any required
// false → FAIL. All true → PASS. No checks → ERROR (unconfigured test).
func deriveStatus(checks []testrunner.Check) testrunner.Status {
	if len(checks) == 0 {
		return testrunner.StatusError
	}
	for _, c := range checks {
		if c.Required && !c.Pass {
			return testrunner.StatusFail
		}
	}
	return testrunner.StatusPass
}

// mineBlocks asks svnode-1's wallet for a fresh address and mines n blocks
// to it. Returns the list of mined block hashes. Used by tests that need
// to advance the chain.
func mineBlocks(ctx context.Context, env *testrunner.Env, n int) ([]string, error) {
	if env.SVNode == nil || env.SVNode.RPC == nil {
		return nil, errors.New("svnode RPC not configured")
	}
	addr, err := env.SVNode.RPC.GetNewAddress(ctx)
	if err != nil {
		return nil, fmt.Errorf("getnewaddress: %w", err)
	}
	hashes, err := env.SVNode.RPC.GenerateToAddress(ctx, n, addr)
	if err != nil {
		return nil, fmt.Errorf("generatetoaddress: %w", err)
	}
	return hashes, nil
}

// waitForTeranodeTip polls Teranode RPC until its chain tip matches want
// or the deadline passes. Returns nil on success.
func waitForTeranodeTip(ctx context.Context, rpc *teranode.RPCClient, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h, err := rpc.GetBestBlockHash(ctx)
		if err == nil && h == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("teranode tip never reached %s within %v", want, timeout)
}

// tlsInfo describes a successful TLS handshake.
type tlsInfo struct {
	Version uint16
	Cipher  string
}

// probeTLS dials u as TCP+TLS and returns the negotiated version + cipher.
func probeTLS(ctx context.Context, u *url.URL) (tlsInfo, error) {
	host := u.Host
	if !strings.Contains(host, ":") {
		switch u.Scheme {
		case "https":
			host += ":443"
		case "wss":
			host += ":443"
		default:
			return tlsInfo{}, fmt.Errorf("no port for scheme %q", u.Scheme)
		}
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	rawConn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return tlsInfo{}, fmt.Errorf("dial: %w", err)
	}
	defer rawConn.Close()
	tlsConn := tls.Client(rawConn, &tls.Config{ServerName: u.Hostname()})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return tlsInfo{}, fmt.Errorf("handshake: %w", err)
	}
	state := tlsConn.ConnectionState()
	return tlsInfo{Version: state.Version, Cipher: tls.CipherSuiteName(state.CipherSuite)}, nil
}

// measureLatency runs probeFn for each item in inputs sequentially,
// records elapsed time, and returns the p95 latency (or 0 if inputs empty).
// Errors from probeFn are still timed (the latency includes the
// error-discovery time) but are also counted via the optional errCount
// pointer.
func measureLatency(ctx context.Context, _ string, inputs []string, probeFn func(string) error) time.Duration {
	if len(inputs) == 0 {
		return 0
	}
	durations := make([]time.Duration, 0, len(inputs))
	for _, in := range inputs {
		select {
		case <-ctx.Done():
			goto done
		default:
		}
		start := time.Now()
		_ = probeFn(in)
		durations = append(durations, time.Since(start))
	}
done:
	if len(durations) == 0 {
		return 0
	}
	// Sort ascending and pick p95.
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	idx := int(float64(len(durations)) * 0.95)
	if idx >= len(durations) {
		idx = len(durations) - 1
	}
	return durations[idx]
}

// intRange returns ["start", "start+1", ..., "start+n-1"] as strings (for
// measureLatency callers that walk a numeric range).
func intRange(start, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%d", start+i))
	}
	return out
}

// classifyRateLimit inspects err for rate-limit-shaped indicators.
// Returns the HTTP status (or 0) and whether it was a limit.
func classifyRateLimit(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "429"):
		return 429, true
	case strings.Contains(strings.ToLower(s), "rate limit"):
		return 429, true
	case strings.Contains(strings.ToLower(s), "too many requests"):
		return 429, true
	case strings.Contains(s, "503"):
		return 503, true
	}
	return 0, false
}

// jsonUnmarshalLooseImpl is a thin wrapper around encoding/json.Unmarshal.
// Used by client1.go (and any future callers) to parse REST JSON responses
// without importing encoding/json directly in the caller file.
func jsonUnmarshalLooseImpl(b []byte, v any) error {
	return json.Unmarshal(b, v)
}

// tailBlocks ranges over notif.Blocks() until ctx is cancelled.
// Used by CLIENT-1 so each notification goroutine has its own cancellable
// context; calling cancel() on that context stops the goroutine promptly
// when the client is disconnected and replaced by a fresh one.
func tailBlocks(ctx context.Context, notif *teranode.NotificationClient, mu *sync.Mutex, seen map[string]bool, count *int) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-notif.Blocks():
			mu.Lock()
			seen[e.Hash] = true
			*count++
			mu.Unlock()
		}
	}
}

// tailBlockHeights ranges over notif.Blocks() and appends block heights until
// ctx is cancelled. Used by CLIENT-3 for the same per-goroutine cancel pattern.
func tailBlockHeights(ctx context.Context, notif *teranode.NotificationClient, mu *sync.Mutex, heights *[]uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-notif.Blocks():
			mu.Lock()
			*heights = append(*heights, e.Height)
			mu.Unlock()
		}
	}
}

// teranodeTipReader wraps *teranode.RPCClient to satisfy observer.TipReader.
// teranode.RPCClient.GetBlockchainInfo returns (BlockchainInfo, error) rather
// than (json.RawMessage, error), so this adapter marshals the response.
type teranodeTipReader struct {
	rpc *teranode.RPCClient
}

func (t *teranodeTipReader) GetBestBlockHash(ctx context.Context) (string, error) {
	return t.rpc.GetBestBlockHash(ctx)
}

func (t *teranodeTipReader) GetBlockchainInfo(ctx context.Context) (json.RawMessage, error) {
	info, err := t.rpc.GetBlockchainInfo(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

// reorgResult captures the outcome of a reorg induction sub-phase.
type reorgResult struct {
	BaselineHash   string
	BaselineHeight int64
	WinnerHash     string
	WinnerHeight   int64
	ConvergedAt    time.Time
	Reorged        bool
	Err            error
}

// induceReorg manually creates competing chains and verifies convergence.
//
// Procedure (per SP9 spec §3.1):
//  1. Capture baseline (svnode-1 best-block-hash).
//  2. Mine 1 block on svnode-1 → B1.
//  3. Wait for B1 to propagate to teranode-1.
//  4. invalidateblock(B1) on teranode-1 (rolls teranode-1 back).
//  5. generatetoaddress 2 blocks on teranode-1 → T1, T2 (longer chain).
//  6. Wait for svnode-1 to reorg to T2.
//  7. Return success/failure.
//
// Returns reorgResult with details. If any step fails, sets Err and returns.
func induceReorg(ctx context.Context, env *testrunner.Env, _ []observer.TipSnapshot) reorgResult {
	res := reorgResult{}
	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil {
		res.Err = fmt.Errorf("required clients not configured")
		return res
	}

	// 1. Baseline.
	baseline, err := env.SVNode.RPC.GetBestBlockHash(ctx)
	if err != nil {
		res.Err = fmt.Errorf("baseline: %w", err)
		return res
	}
	res.BaselineHash = baseline

	// 2. Mine 1 block on svnode-1.
	addr, err := env.SVNode.RPC.GetNewAddress(ctx)
	if err != nil {
		res.Err = fmt.Errorf("getnewaddress: %w", err)
		return res
	}
	b1Hashes, err := env.SVNode.RPC.GenerateToAddress(ctx, 1, addr)
	if err != nil || len(b1Hashes) != 1 {
		res.Err = fmt.Errorf("mine B1: err=%v hashes=%v", err, b1Hashes)
		return res
	}
	b1 := b1Hashes[0]

	// 3. Wait for propagation: poll teranode-1's tip until == b1.
	if err := waitForTeranodeTip(ctx, env.Teranode.RPC, b1, 30*time.Second); err != nil {
		res.Err = fmt.Errorf("B1 propagation: %w", err)
		return res
	}

	// 4. invalidateblock(B1) on teranode-1, with one retry after 2s
	// per SP9 spec §7 risk B (transient busy/mining races).
	var dummy json.RawMessage
	invalErr := env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy)
	if invalErr != nil {
		select {
		case <-ctx.Done():
			res.Err = ctx.Err()
			return res
		case <-time.After(2 * time.Second):
		}
		invalErr = env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy)
	}
	if invalErr != nil {
		res.Err = fmt.Errorf("invalidateblock B1 on teranode-1 (after retry): %w", invalErr)
		return res
	}

	// 5. generatetoaddress 2 blocks on teranode-1.
	teranodeAddr := env.TxGen.Address()
	var teranodeMined []string
	if err := env.Teranode.RPC.Call(ctx, "generatetoaddress", []any{2, teranodeAddr}, &teranodeMined); err != nil {
		res.Err = fmt.Errorf("generatetoaddress on teranode-1: %w", err)
		return res
	}
	if len(teranodeMined) != 2 {
		res.Err = fmt.Errorf("teranode-1 mined %d blocks, want 2", len(teranodeMined))
		return res
	}
	t2 := teranodeMined[1]
	res.WinnerHash = t2

	// 6. Wait for svnode-1 to reorg to T2.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		current, err := env.SVNode.RPC.GetBestBlockHash(ctx)
		if err == nil && current == t2 {
			res.ConvergedAt = time.Now()
			res.Reorged = true
			return res
		}
		select {
		case <-ctx.Done():
			res.Err = ctx.Err()
			return res
		case <-time.After(500 * time.Millisecond):
		}
	}
	res.Err = fmt.Errorf("svnode-1 did not reorg to T2=%s within 30s", t2)
	return res
}
