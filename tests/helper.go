package tests

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/svnode"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

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

// Compile-time guards: ensure the helper types depend on the right packages so
// imports stay live in builds where some helpers aren't called.
var _ matrix.Severity = matrix.SeverityCritical
var _ *svnode.RPCClient
var _ *teranode.RPCClient
var _ = probeTLS
var _ tlsInfo
