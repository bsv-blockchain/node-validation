# SP5 — Cheap Probe Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land 4 acceptance tests (OPS-3, PC-3, NEW-NFR11, NEW-NFR13) running against the SP4-DOCKER stack via `make compose-test`. After SP5, the report transitions from "all NOT_RUN" to having actual pass/fail data on these four IDs.

**Architecture:** Each test in its own file in flat `package tests`. Shared helpers in `tests/helper.go`. Two new SV Node RPC methods (`GetNewAddress`, `GenerateToAddress`) added to support PC-3's mining trigger. Two new config knobs for the rate-limit probe. Tests are registered in `cmd/teranode-acceptance/register.go`.

**Tech Stack:** Existing — Go 1.22, libsv/go-bt/v2, libsv/go-bk, no new external deps. Live tests run against `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2` and `bitcoinsv/bitcoin-sv:1.1.0` from SP4-DOCKER.

---

### Task 1: SVNode RPC additions (`GetNewAddress`, `GenerateToAddress`)

**Files:**
- Modify: `internal/svnode/rpc.go`
- Modify: `internal/svnode/rpc_test.go`

- [ ] **Step 1: Append to `internal/svnode/rpc.go`** (after the existing convenience-method block):

```go
// GetNewAddress returns a fresh address from the SV Node wallet.
// Requires the node to have wallet support enabled.
func (c *RPCClient) GetNewAddress(ctx context.Context) (string, error) {
	var s string
	return s, c.caller.Call(ctx, "getnewaddress", nil, &s)
}

// GenerateToAddress mines n blocks paying the coinbase to addr.
// Returns the list of mined block hashes. Regtest only.
func (c *RPCClient) GenerateToAddress(ctx context.Context, n int, addr string) ([]string, error) {
	var hashes []string
	return hashes, c.caller.Call(ctx, "generatetoaddress", []any{n, addr}, &hashes)
}
```

- [ ] **Step 2: Append to `internal/svnode/rpc_test.go`**:

```go
func TestRPC_GetNewAddress(t *testing.T) {
	srv := newRPCStub(t, func(method string, _ []any) any {
		if method != "getnewaddress" {
			t.Errorf("method: %s", method)
		}
		return "n3FreshAddr111111111111111111111111"
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	a, err := c.GetNewAddress(context.Background())
	if err != nil {
		t.Fatalf("GetNewAddress: %v", err)
	}
	if a != "n3FreshAddr111111111111111111111111" {
		t.Errorf("addr: %q", a)
	}
}

func TestRPC_GenerateToAddress(t *testing.T) {
	srv := newRPCStub(t, func(method string, params []any) any {
		if method != "generatetoaddress" || len(params) != 2 {
			t.Errorf("method=%s params=%v", method, params)
		}
		if params[0].(float64) != 5 {
			t.Errorf("n: %v", params[0])
		}
		return []string{"hash1", "hash2", "hash3", "hash4", "hash5"}
	})
	defer srv.Close()
	c, _ := NewRPCClient(srv.URL, "u", "p", nil)
	hashes, err := c.GenerateToAddress(context.Background(), 5, "n3...")
	if err != nil {
		t.Fatalf("GenerateToAddress: %v", err)
	}
	if len(hashes) != 5 {
		t.Errorf("hashes: %d want 5", len(hashes))
	}
}
```

- [ ] **Step 3: Run, expect pass**

```bash
go test -race ./internal/svnode/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/svnode/
git commit -m "feat(svnode): add GetNewAddress and GenerateToAddress for regtest mining"
```

---

### Task 2: Config additions (`NFR13MaxProbeRate`, `NFR13ProbeDuration`)

**Files:**
- Modify: `config/config.go`
- Modify: `config/defaults.go`
- Modify: `config/validate.go`
- Modify: `config/validate_test.go`

- [ ] **Step 1: Extend `Limits` struct in `config/config.go`** — add two fields:

```go
type Limits struct {
	PERF1MaxTPS         int           `yaml:"perf1_max_tps"`
	INTER2TxCount       int           `yaml:"inter2_tx_count"`
	CLIENT3TxCount      int           `yaml:"client3_tx_count"`
	FR7ChainDepth       int           `yaml:"fr7_chain_depth"`
	FR10LatencyTargetMs int           `yaml:"fr10_latency_target_ms"`
	FR8PriorityLevels   []string      `yaml:"fr8_priority_levels"`
	NFR13MaxProbeRate   int           `yaml:"nfr13_max_probe_rate"`
	NFR13ProbeDuration  time.Duration `yaml:"nfr13_probe_duration"`
}
```

- [ ] **Step 2: Update `mergeYAML` in `config/config.go`** to copy the new fields:

```go
if src.Limits.NFR13MaxProbeRate != 0 {
    dst.Limits.NFR13MaxProbeRate = src.Limits.NFR13MaxProbeRate
}
if src.Limits.NFR13ProbeDuration != 0 {
    dst.Limits.NFR13ProbeDuration = src.Limits.NFR13ProbeDuration
}
```

- [ ] **Step 3: Append defaults in `config/defaults.go`**:

```go
if c.Limits.NFR13MaxProbeRate == 0 {
    c.Limits.NFR13MaxProbeRate = 1000
}
if c.Limits.NFR13ProbeDuration == 0 {
    c.Limits.NFR13ProbeDuration = 5 * time.Second
}
```

- [ ] **Step 4: Add validation in `config/validate.go`** (inside the existing `Validate` function, in the limits block):

```go
if c.Limits.NFR13MaxProbeRate < 0 {
    errs = append(errs, "limits.nfr13_max_probe_rate must be ≥ 0")
}
if c.Limits.NFR13ProbeDuration < 0 {
    errs = append(errs, "limits.nfr13_probe_duration must be ≥ 0")
}
```

(Negative is invalid; zero disables the test, which is acceptable.)

- [ ] **Step 5: Append tests to `config/validate_test.go`**:

```go
func TestValidate_NFR13Defaults(t *testing.T) {
	c := validBase()
	if err := Validate(&c); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_NFR13NegativeProbeRate(t *testing.T) {
	c := validBase()
	c.Limits.NFR13MaxProbeRate = -1
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "nfr13_max_probe_rate") {
		t.Errorf("want NFR13MaxProbeRate error, got %v", err)
	}
}

func TestValidate_NFR13NegativeProbeDuration(t *testing.T) {
	c := validBase()
	c.Limits.NFR13ProbeDuration = -1
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "nfr13_probe_duration") {
		t.Errorf("want NFR13ProbeDuration error, got %v", err)
	}
}
```

Add the new fields to `validBase()` so existing tests still pass:

```go
Limits: Limits{
    PERF1MaxTPS: 1, INTER2TxCount: 1, CLIENT3TxCount: 1,
    FR7ChainDepth: 1, FR10LatencyTargetMs: 1,
    FR8PriorityLevels: []string{"standard"},
    NFR13MaxProbeRate: 100,
    NFR13ProbeDuration: time.Second,
},
```

- [ ] **Step 6: Run, expect pass**

```bash
go test -race ./config/...
```

- [ ] **Step 7: Commit**

```bash
git add config/
git commit -m "feat(config): add NFR13 rate-limit probe knobs (max_rate, duration)"
```

---

### Task 3: Update YAML configs

**Files:**
- Modify: `config/testdata/minimal.yaml`
- Modify: `config.example.yaml`
- Modify: `config.docker.yaml`
- Modify: `cmd/teranode-acceptance/testdata/integration.yaml`

- [ ] **Step 1: Update `config/testdata/minimal.yaml`** — add to limits section:

```yaml
  nfr13_max_probe_rate: 100
  nfr13_probe_duration: 1s
```

- [ ] **Step 2: Update `config.example.yaml`** — add to limits section:

```yaml
  nfr13_max_probe_rate: 1000     # NEW-NFR13: max probe rate (req/s); 0 disables
  nfr13_probe_duration: 5s       # NEW-NFR13: how long to ramp; 0 disables
```

- [ ] **Step 3: Update `config.docker.yaml`** — add to limits section:

```yaml
  nfr13_max_probe_rate: 1000
  nfr13_probe_duration: 5s
```

- [ ] **Step 4: Update `cmd/teranode-acceptance/testdata/integration.yaml`** — add to limits section:

```yaml
  nfr13_max_probe_rate: 100
  nfr13_probe_duration: 1s
```

- [ ] **Step 5: Verify**

```bash
make build lint test verify
```

Should exit 0 — the example loader test (`TestExampleYAMLLoads`) and integration tests should still pass.

- [ ] **Step 6: Commit**

```bash
git add config/testdata/ config.example.yaml config.docker.yaml cmd/teranode-acceptance/testdata/
git commit -m "chore(config): wire nfr13 probe knobs into all config YAMLs"
```

---

### Task 4: `tests/helper.go` shared utilities

**Files:**
- Create: `tests/doc.go`
- Create: `tests/helper.go`
- Create: `tests/tests_test.go`

- [ ] **Step 1: Package doc**

```go
// Package tests contains the acceptance-test functions registered with
// the suite by cmd/teranode-acceptance/register.go. Each public Run*
// function follows the testrunner.TestFunc signature and is named
// after the test ID it implements (RunOPS3, RunPC3, etc.).
//
// Tests run against a live Teranode + SV Node pair (the SP4-DOCKER
// compose stack by default) and use the typed clients in env.Teranode,
// env.SVNode, plus the txgen funder in env.TxGen.
package tests
```

- [ ] **Step 2: Implement `tests/helper.go`**

```go
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
```

- [ ] **Step 3: Smoke test for the helper package**

```go
// tests/tests_test.go
package tests

import "testing"

func TestDeriveStatus_passOnAllPass(t *testing.T) {
	checks := []Check{} // alias not needed; use testrunner.Check directly via helper
	_ = checks
	// Build via helpers and invoke deriveStatus.
	// The actual smoke is just that the package compiles + helpers don't panic.
}
```

Replace with the actual tests:

```go
// tests/tests_test.go
package tests

import (
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func TestDeriveStatus_allPass(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		ok("b", ""),
	}
	if got := deriveStatus(c); got != testrunner.StatusPass {
		t.Errorf("got %s want PASS", got)
	}
}

func TestDeriveStatus_anyRequiredFail(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		fail("b", "boom"),
	}
	if got := deriveStatus(c); got != testrunner.StatusFail {
		t.Errorf("got %s want FAIL", got)
	}
}

func TestDeriveStatus_emptyIsError(t *testing.T) {
	if got := deriveStatus(nil); got != testrunner.StatusError {
		t.Errorf("got %s want ERROR", got)
	}
}

func TestClassifyRateLimit_429(t *testing.T) {
	if _, ok := classifyRateLimit(errFromString("HTTP 429 Too Many Requests")); !ok {
		t.Error("want classified as limit")
	}
}

func TestClassifyRateLimit_nilNotLimit(t *testing.T) {
	if _, ok := classifyRateLimit(nil); ok {
		t.Error("nil should not be a limit")
	}
}

// errFromString is a tiny helper so tests don't need to import errors.
type errString string

func (e errString) Error() string { return string(e) }

func errFromString(s string) error { return errString(s) }
```

- [ ] **Step 4: Run, expect pass**

```bash
go test -race ./tests/...
go vet ./tests/...
gofmt -l tests/
```

- [ ] **Step 5: Commit**

```bash
git add tests/
git commit -m "feat(tests): add shared helpers (mineBlocks, probeTLS, deriveStatus, ...)"
```

---

### Task 5: `tests/ops3.go` — OPS-3

**Files:**
- Create: `tests/ops3.go`

- [ ] **Step 1: Implement**

```go
// Package tests — OPS-3 implementation.
//
// Source plan §"Operational and Failure-Mode Tests" → OPS-3.
// Captures risk R6. Acceptance criteria from NFR-10. Severity Important.
//
// Objective:
//   Verify TNG can monitor platform health and performance.
//
// Method:
//   1. HTTP-GET metrics_url; verify 200 + parseable Prometheus exposition.
//   2. Verify presence of metric categories: chain tip height, sync status,
//      mempool size, transaction throughput, block validation latency.
//   3. HTTP-GET health_url; verify 200 + parseable status body.
//
// Acceptance criteria:
//   • Metrics endpoint returns 200, valid format.
//   • All five required metric categories present.
//   • Health endpoint returns 200.
//
// Implementation notes:
//   • Uses env.Teranode.Metrics + env.Teranode.Health from SP3.
//   • Skips with reason if either client is nil.
//   • Metric names sourced from SP2 discovery (commit 11f5fa6a8). If the
//     pinned image v0.15.0-beta-2 has renamed any metric, expect FAIL on
//     that specific check; operator updates the constant set in this file.

package tests

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunOPS3(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "OPS-3", Title: "Observability and Monitoring",
		Severity: matrix.SeverityImportant,
		StartedAt: env.Now(),
		SatisfiesRequirements: []string{"NFR-10"},
		CapturedRisks: []string{"R6"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.Metrics == nil || env.Teranode.Health == nil {
		return skipMissing(res, "Teranode metrics or health client not configured")
	}

	// (1) /metrics returns 200 + parseable.
	mfs, err := env.Teranode.Metrics.Scrape(ctx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Metrics endpoint returns 200 with parseable Prometheus body",
		err == nil && len(mfs) > 0,
		fmt.Sprintf("scraped %d metric families; err=%v", len(mfs), err),
	))

	// (2) Required metric categories.
	requiredMetrics := []struct{ name, category string }{
		{"teranode_blockassembly_best_block_height", "chain tip height"},
		{"teranode_blockchain_fsm_current_state", "sync status"},
		{"teranode_blockassembly_transactions", "mempool size"},
		{"teranode_validator_transactions_count", "transaction throughput"},
		{"teranode_blockvalidation_validate_block_count", "block validation latency"},
	}
	for _, m := range requiredMetrics {
		_, present := mfs[m.name]
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("Metric %q present (%s)", m.name, m.category),
			present,
			fmt.Sprintf("present=%v", present),
		))
	}

	// (3) Health readiness returns 200 + parseable.
	rep, herr := env.Teranode.Health.Readiness(ctx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Health endpoint returns 200 with JSON-parseable body",
		herr == nil && rep.Status != "",
		fmt.Sprintf("status=%q services=%d err=%v", rep.Status, len(rep.Services), herr),
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/ops3.go
git commit -m "feat(tests): add OPS-3 — Observability and Monitoring"
```

---

### Task 6: `tests/pc3.go` — PC-3

**Files:**
- Create: `tests/pc3.go`

- [ ] **Step 1: Implement**

```go
// Package tests — PC-3 implementation.
//
// Source plan §"Protocol Correctness Tests" → PC-3.
// Captures risk R2. Acceptance criteria from FR-2. Severity Critical.
//
// Objective:
//   Verify standard BSV transactions round-trip byte-identical through
//   Teranode, and Teranode-emitted blocks parse with a standard parser.
//
// Method:
//   1. Construct standard BSV transactions of three shapes (P2PKH,
//      P2MS multisig, OP_RETURN data carrier) using libsv/go-bt/v2.
//   2. Submit via Teranode RPC sendrawtransaction; record returned txid.
//   3. Fetch each tx back via Teranode REST /tx/{hash}; verify byte-exact
//      round-trip (matching txid).
//   4. Mine a block via svnode-1; wait for Teranode tip to advance.
//   5. Fetch the block; re-parse with libsv parser; verify all three test
//      transactions are in it.
//
// Acceptance criteria:
//   • All transactions round-trip with matching txid.
//   • All test blocks parse without error.
//
// Implementation notes:
//   • Scope is format-only; raw P2P packet capture is out of scope for SP5.
//   • The funder must have a UTXO ≥1.5 BSV; bootstrap if the balance is low.
//   • P2MS uses 2-of-3 with three deterministic dummy compressed pubkeys.
//   • Mining uses svnode-1's wallet via mineBlocks helper.
//   • Wait timeout: 60s for tip propagation (configurable via code).

package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	bt "github.com/libsv/go-bt/v2"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunPC3(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "PC-3", Title: "Message Format and Wire Protocol Verification",
		Severity: matrix.SeverityCritical,
		StartedAt: env.Now(),
		SatisfiesRequirements: []string{"FR-2"},
		CapturedRisks: []string{"R2"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil || env.TxGen == nil {
		return skipMissing(res, "Teranode RPC/REST, SVNode RPC, or TxGen not configured")
	}

	funder := env.TxGen
	builder := funder.Builder()

	// Bootstrap UTXO if needed.
	if funder.Balance() < 100_000_000 {
		if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		// Mine to confirm.
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, fmt.Errorf("confirm bootstrap: %w", err))
		}
		time.Sleep(2 * time.Second) // brief settle for propagation
	}

	addrScript, err := txgen.P2PKHScript(funder.Address())
	if err != nil {
		return errorResult(res, fmt.Errorf("p2pkh script: %w", err))
	}

	// Three deterministic dummy compressed pubkeys for the P2MS shape.
	pubkeys := [][]byte{
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa1},
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa2},
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa3},
	}

	type built struct {
		shape   string
		expected [32]byte
		txid    string
	}
	var txs []built

	// Shape 1 — P2PKH.
	bres, err := builder.BuildP2PKH(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		return errorResult(res, fmt.Errorf("build P2PKH: %w", err))
	}
	if err := submitAndConfirm(ctx, env, &funder, bres, &txs, "P2PKH", &res); err != nil {
		return errorResult(res, err)
	}

	// Shape 2 — P2MS (output paying to a 2-of-3 multisig).
	bres2, err := builder.BuildP2MS(txgen.BuildRequest{Outputs: nil, FeeRate: 500}, 2, pubkeys, 5_000)
	if err != nil {
		return errorResult(res, fmt.Errorf("build P2MS: %w", err))
	}
	if err := submitAndConfirm(ctx, env, &funder, bres2, &txs, "P2MS", &res); err != nil {
		return errorResult(res, err)
	}

	// Shape 3 — OP_RETURN.
	bres3, err := builder.BuildOpReturn(txgen.BuildRequest{Outputs: nil, FeeRate: 500}, []byte("PC-3 round-trip"))
	if err != nil {
		return errorResult(res, fmt.Errorf("build OP_RETURN: %w", err))
	}
	if err := submitAndConfirm(ctx, env, &funder, bres3, &txs, "OP_RETURN", &res); err != nil {
		return errorResult(res, err)
	}

	// (4) Mine a block.
	mined, err := mineBlocks(ctx, env, 1)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block mined via svnode-1",
		err == nil && len(mined) == 1,
		fmt.Sprintf("hashes=%v err=%v", mined, err),
	))
	if err != nil || len(mined) != 1 {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}
	blockHash := mined[0]

	// Wait for Teranode tip to advance.
	terr := waitForTeranodeTip(ctx, env.Teranode.RPC, blockHash, 60*time.Second)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Teranode tip reached mined block within 60s",
		terr == nil,
		fmt.Sprintf("tip=%s err=%v", blockHash, terr),
	))
	if terr != nil {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// (5) Fetch the block, parse, verify our test txs are present.
	blockBytes, err := env.Teranode.REST.GetBlockBytes(ctx, blockHash)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block bytes fetched via Teranode REST",
		err == nil && len(blockBytes) > 0,
		fmt.Sprintf("bytes=%d err=%v", len(blockBytes), err),
	))
	if err != nil {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// Parse with libsv. The block format may be standard Bitcoin (header + tx-count VarInt + txs)
	// or a Teranode-specific shape; if the standard parser fails, that's a finding.
	stdTxids, parseErr := parseStandardBlock(blockBytes)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block parses with standard libsv parser",
		parseErr == nil && len(stdTxids) > 0,
		fmt.Sprintf("txs_in_block=%d err=%v", len(stdTxids), parseErr),
	))

	// Verify each of our txs is in the block.
	idSet := map[string]bool{}
	for _, id := range stdTxids {
		idSet[id] = true
	}
	for _, t := range txs {
		present := idSet[t.txid]
		shortID := t.txid
		if len(shortID) > 10 {
			shortID = shortID[:10]
		}
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("Block contains %s test tx (%s…)", t.shape, shortID),
			present,
			fmt.Sprintf("present=%v", present),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}

// submitAndConfirm submits a built tx via Teranode RPC, fetches it back via REST,
// verifies the round-trip, and (on success) marks the funder's inputs spent and
// the change UTXO available. Appends acceptance checks to res.
func submitAndConfirm(
	ctx context.Context,
	env *testrunner.Env,
	funder **txgen.Funder,
	bres txgen.BuildResult,
	txs *[]struct {
		shape    string
		expected [32]byte
		txid     string
	},
	shape string,
	res *testrunner.Result,
) error {
	returnedTxid, err := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Teranode accepts %s tx via sendrawtransaction", shape),
		err == nil && returnedTxid != "",
		fmt.Sprintf("returned=%q err=%v", returnedTxid, err),
	))
	if err != nil {
		return nil // recorded as failed check, don't error the whole test
	}

	// Verify the returned txid matches the locally-computed one.
	expectedHex := hex.EncodeToString(bres.TxID[:])
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Returned %s txid matches locally-computed", shape),
		returnedTxid == expectedHex,
		fmt.Sprintf("returned=%s expected=%s", returnedTxid, expectedHex),
	))

	// Fetch back via REST.
	fetched, err := env.Teranode.REST.GetTxBytes(ctx, returnedTxid)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Teranode REST returns %s tx body", shape),
		err == nil && len(fetched) > 0,
		fmt.Sprintf("bytes=%d err=%v", len(fetched), err),
	))
	if err != nil {
		return nil
	}

	// Re-parse and verify the txid recomputes to the same value.
	parsed, err := bt.NewTxFromBytes(fetched)
	roundOK := err == nil && hex.EncodeToString(parsed.TxIDBytes()) == returnedTxid
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("%s tx re-parses with matching txid", shape),
		roundOK,
		fmt.Sprintf("err=%v", err),
	))

	(*funder).Confirm(bres.Inputs, bres.Change)
	*txs = append(*txs, struct {
		shape    string
		expected [32]byte
		txid     string
	}{shape: shape, expected: bres.TxID, txid: returnedTxid})
	return nil
}

// parseStandardBlock parses a serialized BSV block and returns the list of
// transaction IDs in order. Uses bt.NewTxFromStream-style iteration since
// libsv/go-bt/v2 does not export a top-level "Block" parser; we read header
// + VarInt + repeated bt.NewTxFromStream.
func parseStandardBlock(blockBytes []byte) ([]string, error) {
	if len(blockBytes) < 81 {
		return nil, fmt.Errorf("block too short: %d bytes", len(blockBytes))
	}
	// Skip the 80-byte header.
	body := blockBytes[80:]
	// Read VarInt for tx count.
	count, n, err := readVarInt(body)
	if err != nil {
		return nil, fmt.Errorf("read tx count: %w", err)
	}
	body = body[n:]
	out := make([]string, 0, count)
	for i := uint64(0); i < count; i++ {
		tx, used, err := bt.NewTxFromStream(body)
		if err != nil {
			return nil, fmt.Errorf("parse tx %d: %w", i, err)
		}
		out = append(out, hex.EncodeToString(tx.TxIDBytes()))
		body = body[used:]
	}
	return out, nil
}

// readVarInt decodes a Bitcoin-style VarInt and returns the value and bytes consumed.
func readVarInt(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("empty input")
	}
	switch b[0] {
	case 0xfd:
		if len(b) < 3 {
			return 0, 0, fmt.Errorf("truncated 0xfd varint")
		}
		return uint64(b[1]) | uint64(b[2])<<8, 3, nil
	case 0xfe:
		if len(b) < 5 {
			return 0, 0, fmt.Errorf("truncated 0xfe varint")
		}
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24, 5, nil
	case 0xff:
		if len(b) < 9 {
			return 0, 0, fmt.Errorf("truncated 0xff varint")
		}
		v := uint64(0)
		for i := 0; i < 8; i++ {
			v |= uint64(b[1+i]) << (8 * i)
		}
		return v, 9, nil
	default:
		return uint64(b[0]), 1, nil
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/pc3.go
git commit -m "feat(tests): add PC-3 — Message Format and Wire Protocol Verification"
```

---

### Task 7: `tests/new_nfr11.go` — NEW-NFR11

**Files:**
- Create: `tests/new_nfr11.go`

- [ ] **Step 1: Implement**

```go
// Package tests — NEW-NFR11 implementation.
//
// Source: derived from NFR-11 (no source-plan test case). Severity Advisory.
//
// Objective:
//   Verify Teranode endpoints support the security posture NFR-11 demands.
//
// Method:
//   1. For each configured Teranode endpoint URL (rpc, rest, notifications,
//      metrics, health) plus svnode RPC: resolve scheme; if https/wss attempt
//      TLS handshake and record version + cipher; if http/ws record as a
//      finding (regtest plain transport, production must terminate TLS).
//   2. Probe authentication: try unauthenticated request to a protected
//      endpoint (Teranode RPC); try authenticated; record both outcomes.
//   3. Rate-limit headers (overlap with NEW-NFR13) — not parsed in this test.
//
// Acceptance criteria (per NFR-11):
//   • TLS 1.2 or higher negotiated where TLS is in use.
//   • Authenticated endpoint rejects unauthenticated requests.
//   • No mandatory plain-text transport for production-relevant endpoints.
//
// Implementation notes:
//   • In docker regtest all transports are plain HTTP/WS — per spec §9 Q1=A,
//     plain HTTP findings record as Pass with a note explaining production
//     posture, not as a failure.
//   • Auth is exercised by constructing a fresh teranode.RPCClient with empty
//     credentials and confirming it's rejected.

package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWNFR11(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-NFR11", Title: "Transport Security and Authentication Probe",
		Severity: matrix.SeverityAdvisory,
		StartedAt: env.Now(),
		SatisfiesRequirements: []string{"NFR-11"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil {
		return skipMissing(res, "Teranode client not configured")
	}

	// (1) URL-by-URL transport probe.
	urls := []struct{ name, raw string }{
		{"teranode.rpc", env.Cfg.Teranode.RPCURL},
		{"teranode.rest", env.Cfg.Teranode.RESTURL},
		{"teranode.notifications", env.Cfg.Teranode.NotificationURL},
		{"teranode.metrics", env.Cfg.Teranode.MetricsURL},
		{"teranode.health", env.Cfg.Teranode.HealthURL},
		{"svnode.rpc", env.Cfg.SVNode.RPCURL},
	}
	for _, u := range urls {
		if u.raw == "" {
			continue
		}
		parsed, err := url.Parse(u.raw)
		if err != nil {
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				fmt.Sprintf("[%s] URL parses", u.name),
				err.Error(),
			))
			continue
		}
		switch parsed.Scheme {
		case "https", "wss":
			info, err := probeTLS(ctx, parsed)
			res.AcceptanceChecks = append(res.AcceptanceChecks, required(
				fmt.Sprintf("[%s] TLS handshake succeeded with version >= 1.2", u.name),
				err == nil && info.Version >= tls.VersionTLS12,
				fmt.Sprintf("version=0x%04x cipher=%s err=%v", info.Version, info.Cipher, err),
			))
		case "http", "ws":
			res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
				fmt.Sprintf("[%s] transport scheme is %q", u.name, parsed.Scheme),
				"regtest plain transport — production deployment must terminate TLS in front",
			))
		case "tcp":
			// SVNode ZMQ — not applicable.
		}
	}

	// (2) Auth probe — Teranode RPC requires Basic Auth.
	if env.Teranode.RPC != nil && env.Cfg.Teranode.RPCURL != "" {
		// Fresh client with empty credentials.
		rawNoAuth, err := teranode.NewRPCClient(env.Cfg.Teranode.RPCURL, "", "", env.Logger)
		if err != nil || rawNoAuth == nil {
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				"Construct unauthenticated client for auth probe",
				fmt.Sprintf("err=%v", err),
			))
		} else {
			_, errNoAuth := rawNoAuth.GetBestBlockHash(ctx)
			isUnauthorised := errNoAuth != nil && (strings.Contains(errNoAuth.Error(), "401") ||
				strings.Contains(strings.ToLower(errNoAuth.Error()), "unauthorized"))
			res.AcceptanceChecks = append(res.AcceptanceChecks, required(
				"Teranode RPC rejects unauthenticated requests with 401",
				isUnauthorised,
				fmt.Sprintf("err=%v", errNoAuth),
			))
		}

		_, errAuth := env.Teranode.RPC.GetBestBlockHash(ctx)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			"Teranode RPC accepts authenticated requests",
			errAuth == nil,
			fmt.Sprintf("err=%v", errAuth),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 2: Verify**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/new_nfr11.go
git commit -m "feat(tests): add NEW-NFR11 — Transport Security and Authentication Probe"
```

---

### Task 8: `tests/new_nfr13.go` — NEW-NFR13

**Files:**
- Create: `tests/new_nfr13.go`

- [ ] **Step 1: Implement**

```go
// Package tests — NEW-NFR13 implementation.
//
// Source: derived from NFR-13. Severity Advisory.
//
// Objective:
//   Verify documented rate limits exist, are consistently enforced, and
//   that 429 / equivalent responses include retry guidance.
//
// Method:
//   1. Issue probe requests at maxRate against getbestblockhash for duration
//      time, OR until the server returns a rate-limit response.
//   2. On first rate-limit response: record status, retry-after header (best
//      effort), body. Wait briefly; verify normal service resumes.
//   3. Report observed limit (or "no_limit_reached").
//
// Acceptance criteria (per NFR-13):
//   • Probing exposes a limit OR documented ceiling reached without one.
//   • If a limit is hit, response includes retry-after-style guidance (best
//     effort given the current RPC client's error type).
//   • Service resumes after the retry period.
//
// Implementation notes:
//   • Configured via Cfg.Limits.NFR13MaxProbeRate (default 1000 req/s) and
//     NFR13ProbeDuration (default 5s). 0 in either disables.
//   • The current RPC client doesn't surface HTTP retry-after headers; this
//     is a known limitation captured by SP10. The test asserts service
//     resumes after a 2-second sleep instead.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWNFR13(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-NFR13", Title: "Rate Limit Discovery and Error Semantics",
		Severity: matrix.SeverityAdvisory,
		StartedAt: env.Now(),
		SatisfiesRequirements: []string{"NFR-13"},
		Observations: map[string]any{},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.RPC == nil {
		return skipMissing(res, "Teranode RPC not configured")
	}
	maxRate := env.Cfg.Limits.NFR13MaxProbeRate
	duration := env.Cfg.Limits.NFR13ProbeDuration
	if maxRate <= 0 || duration <= 0 {
		return skipMissing(res, "rate-limit probe disabled in config")
	}

	deadline := env.Now().Add(duration)
	interval := time.Second / time.Duration(maxRate)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}

	var (
		sent        uint64
		succeeded   uint64
		firstStatus int
		firstErr    error
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for env.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return errorResult(res, ctx.Err())
		case <-ticker.C:
			sent++
			_, err := env.Teranode.RPC.GetBestBlockHash(ctx)
			if err == nil {
				succeeded++
				continue
			}
			if status, isLimit := classifyRateLimit(err); isLimit {
				firstStatus = status
				firstErr = err
				goto LimitObserved
			}
			// Other errors — keep probing; might be transient.
		}
	}

LimitObserved:
	res.Observations["sent"] = sent
	res.Observations["succeeded"] = succeeded
	res.Observations["max_rate_req_per_s"] = maxRate
	res.Observations["probe_duration"] = duration.String()

	if firstErr == nil {
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Rate-limit probe completed without hitting a limit",
			fmt.Sprintf("sent=%d succeeded=%d max_rate=%d duration=%v observed=no_limit_reached",
				sent, succeeded, maxRate, duration),
		))
		res.Observations["limit_observed"] = false
	} else {
		res.Observations["limit_observed"] = true
		res.Observations["limit_status"] = firstStatus
		res.Observations["limit_error"] = firstErr.Error()
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Rate limit observed during probe",
			fmt.Sprintf("sent=%d succeeded=%d firstStatus=%d firstErr=%v",
				sent, succeeded, firstStatus, firstErr),
		))
		// Service resumes after a brief wait.
		time.Sleep(2 * time.Second)
		_, err := env.Teranode.RPC.GetBestBlockHash(ctx)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			"Service resumes after brief wait",
			err == nil,
			fmt.Sprintf("err=%v", err),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 2: Verify**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/new_nfr13.go
git commit -m "feat(tests): add NEW-NFR13 — Rate Limit Discovery and Error Semantics"
```

---

### Task 9: Register tests + done-check

**Files:**
- Modify: `cmd/teranode-acceptance/register.go`
- Modify: `cmd/teranode-acceptance/register_test.go`
- Create: `scripts/sp5-done-check.sh`

- [ ] **Step 1: Update `register.go`**

```go
// cmd/teranode-acceptance/register.go
package main

import (
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/tests"
)

func registerTests(suite *testrunner.Suite) {
	// Alphabetical by ID.
	suite.Register("NEW-NFR11", tests.RunNEWNFR11)
	suite.Register("NEW-NFR13", tests.RunNEWNFR13)
	suite.Register("OPS-3", tests.RunOPS3)
	suite.Register("PC-3", tests.RunPC3)
}
```

- [ ] **Step 2: Update `register_test.go`** to expect 4 tests now

```go
// cmd/teranode-acceptance/register_test.go (replace prior assertion)
func TestRegisterTests_SP5RegistersFour(t *testing.T) {
	cfg := config.Config{TestTimeout: time.Minute}
	env := testrunner.NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
	suite := testrunner.NewSuite(env)
	registerTests(suite)
	results := suite.Run(testContext(t))
	// 4 registered tests — each runs once. Without env.Teranode/SVNode/TxGen,
	// each test should self-skip (or error gracefully).
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	wantIDs := map[string]bool{"NEW-NFR11": false, "NEW-NFR13": false, "OPS-3": false, "PC-3": false}
	for _, r := range results {
		if _, ok := wantIDs[r.ID]; ok {
			wantIDs[r.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing result for %s", id)
		}
	}
}
```

Replace any prior `TestRegisterTests_SP1IsEmpty` test — that assertion is now stale.

- [ ] **Step 3: Run, expect pass**

```bash
go test -race ./cmd/teranode-acceptance/...
```

- [ ] **Step 4: Create `scripts/sp5-done-check.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP4 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

echo "==> tests/ package builds and unit tests pass"
go test -race ./tests/...

echo "==> register.go registers all 4 tests (verified by TestRegisterTests_SP5RegistersFour)"
go test -race ./cmd/teranode-acceptance/... -run TestRegisterTests_SP5RegistersFour

if [ "${SP5_LIVE:-0}" = "1" ]; then
    echo "==> live: bringing up compose stack"
    make compose-up
    echo "==> running the 4 SP5 tests against live stack"
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only NEW-NFR11,NEW-NFR13,OPS-3,PC-3 || true
    test -s report.json
    for id in NEW-NFR11 NEW-NFR13 OPS-3 PC-3; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status (expected non-NOT_RUN)"
            exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi

echo "==> SP5 done-check passed."
```

- [ ] **Step 5: Make executable, run static path**

```bash
chmod +x scripts/sp5-done-check.sh
./scripts/sp5-done-check.sh   # static; SP5_LIVE not set
```

- [ ] **Step 6: Commit**

```bash
git add cmd/teranode-acceptance/ scripts/sp5-done-check.sh
git commit -m "feat(cmd): register 4 SP5 tests; add sp5-done-check"
```

---

### Task 10: Code review and closeout

- [ ] **Step 1: Run `superpowers:code-reviewer`**

Verify:
- 4 tests in `tests/<name>.go`, each with the verbatim Objective/Method/Acceptance source-plan comment block.
- Each test populates `Result.AcceptanceChecks` per source-plan bullet.
- Each test skips cleanly (`StatusSkipped` + reason) when its required clients are nil.
- `tests/helper.go` provides `mineBlocks`, `waitForTeranodeTip`, `probeTLS`, `classifyRateLimit`, `deriveStatus`, and the check builders.
- `internal/svnode/rpc.go` has `GetNewAddress` and `GenerateToAddress` with passing tests.
- `config/config.go` has the two new NFR-13 fields with defaults applied.
- All 4 YAML configs (`testdata/minimal.yaml`, `config.example.yaml`, `config.docker.yaml`, `cmd/teranode-acceptance/testdata/integration.yaml`) include the new fields.
- `cmd/teranode-acceptance/register.go` registers all 4 tests in alphabetical order.
- `scripts/sp5-done-check.sh` static path exits 0; live path (`SP5_LIVE=1`) is documented.
- `make build lint test verify` exits 0; SP1–SP4 + SP4-DOCKER (static) done-checks all pass.

- [ ] **Step 2: Address findings inline; commit per fix**

- [ ] **Step 3: Capture review report**

```bash
mkdir -p docs/superpowers/reviews
$EDITOR docs/superpowers/reviews/2026-04-29-sp5-code-review.md
git add docs/superpowers/reviews/
git commit -m "docs: capture SP5 code-review report"
```

- [ ] **Step 4: Tag SP5 complete**

```bash
git tag -a sp5-complete -m "SP5 — Cheap Probe Tests complete"
```

---

## Self-review checklist (planner)

- [x] Spec coverage — every section of `2026-04-29-sp5-cheap-tests-design.md` is implemented.
- [x] No placeholders — every code block contains real, runnable code (parseStandardBlock implemented inline; no `// TODO` lines).
- [x] All 4 tests follow the same shape (defer Duration, skip-when-nil, AcceptanceChecks per criterion, deriveStatus).
- [x] PC-3's mining wait is 60s (per spec §8 risk A).
- [x] NEW-NFR11's plain-HTTP findings record as Pass with note (per Q1=A).
- [x] NEW-NFR13 is config-driven (per Q2=B).
- [x] register.go alphabetical, register_test.go updated.
