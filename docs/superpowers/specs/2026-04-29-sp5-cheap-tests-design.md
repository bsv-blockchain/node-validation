# SP5 — Cheap Probe Tests (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP5 / 11 — Cheap probe tests (OPS-3, PC-3, NEW-NFR11, NEW-NFR13)
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-29
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1, SP2, SP3, SP4, SP4-DOCKER (compose stack must be up for live runs)
**Status:** awaiting user review

---

## 1. Purpose

Land the first four automated tests against the docker stack. All four are short-running (single-digit seconds individually; <2 minutes for the batch) and exercise mostly-read surfaces. After SP5 the report transitions from `INCOMPLETE` to a meaningful verdict reflecting actual measurements: 2 Important rows (OPS-3) and 2 Advisory rows (NEW-NFR11, NEW-NFR13) plus 1 Critical (PC-3).

Tests run against the SP4-DOCKER compose stack via `make compose-test`. They use the SP3 typed clients and (for PC-3) the SP4 transaction generator.

## 2. Scope

### In scope

- `tests/ops3.go` — OPS-3 observability test.
- `tests/pc3.go` — PC-3 message format / wire protocol round-trip.
- `tests/new_nfr11.go` — NEW-NFR11 transport security + authentication probe.
- `tests/new_nfr13.go` — NEW-NFR13 rate-limit discovery.
- `tests/testutil_test.go` — shared test helpers (skip-when-nil patterns, acceptance-check builders).
- `internal/svnode/rpc.go` — extend with `GenerateToAddress(ctx, n int, addr string) ([]string, error)` per Q3.
- `config/config.go` — extend `Limits` with `NFR13MaxProbeRate` (default 1000 req/s) and `NFR13ProbeDuration` (default 5s) per Q2.
- `cmd/teranode-acceptance/register.go` — register the four tests.
- `config.docker.yaml` — set the new rate-limit probe defaults.
- `config.example.yaml` — same.
- `scripts/sp5-done-check.sh` — done-check.
- `internal/testrunner` unit tests — none new (the test files in `tests/` are not run under `go test` of the testrunner; they're run via the CLI's Suite).

### Out of scope

- The other 15 in-scope test cases — SP6, SP7, SP8, SP9.
- Anti-flake retry logic — if a test flakes against the docker stack, the operator restarts the stack. Retries hide bugs.
- Long-running observation (PC-1 stays in SP9; INTER-1 in SP9).
- Mainnet-load gates — these tests don't generate load, but PC-3 broadcasts ~3 transactions and mines blocks via svnode-1.
- Test fixtures for PC-2 / IBD-2 historical scripts — SP8.

## 3. Architecture

```
suite.Run(ctx)
    │
    ├── tests.RunOPS3(ctx, env)          probes metrics + health endpoints, builds Result
    ├── tests.RunPC3(ctx, env)           builds 3 txs (P2PKH/multisig/OP_RETURN), submits,
    │                                    fetches back, verifies round-trip, mines, parses block
    ├── tests.RunNEWNFR11(ctx, env)      probes 6 endpoints for TLS + auth posture
    └── tests.RunNEWNFR13(ctx, env)      ramps GET getbestblockhash up to N rps for D seconds
```

Each test file follows the same shape:

```go
// tests/<name>.go — Source-plan reproduction comment block.
//
// Objective: <verbatim from source plan>
// Method: <verbatim>
// Acceptance criteria: <verbatim>
//
// Implementation notes:
// - <how this test instantiates the method>
// - <which Env fields it uses>
// - <skip conditions>
package tests

import (
    "context"

    "github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunOPS3(ctx context.Context, env *testrunner.Env) testrunner.Result {
    // ... start a Result with ID, Title, StartedAt baked in
    // ... if any required client is nil → return Skipped
    // ... probe, populate AcceptanceChecks, set Status
    // ... return
}
```

The `Result.AcceptanceChecks` slice mirrors the source plan's bullet-list of acceptance criteria; the HTML reporter from SP1 already renders these in `<details>` blocks (added at SP1 closeout polish).

### Shared helper: `tests/helper_test.go` → `tests/helper.go`

Small utility kept in the same package:

```go
// pass/fail helpers building Check entries with consistent shape
func ok(desc, detail string) testrunner.Check
func fail(desc, detail string) testrunner.Check
func required(desc string, pass bool, detail string) testrunner.Check

// returns a Result skeleton with Status Skipped + reason
func skip(id, title string, severity matrix.Severity, reason string) testrunner.Result

// computes Status from a slice of required Checks: any required Check false → FAIL
func deriveStatus(checks []testrunner.Check) testrunner.Status
```

## 4. Per-test designs

### 4.1 OPS-3 — Observability and Monitoring

**Source plan §"Operational and Failure-Mode Tests" → OPS-3.** Captures risk R6. Acceptance criteria from NFR-10.

**Method (per source plan):**
1. HTTP-GET metrics_url; verify 200 + parseable Prometheus exposition.
2. Verify presence of metric categories: chain tip height, sync status, mempool size, transaction throughput, block validation latency.
3. HTTP-GET health_url; verify 200 + parseable status body.

**Implementation:**

```go
func RunOPS3(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "OPS-3", Title: "Observability and Monitoring", Severity: matrix.SeverityImportant, StartedAt: env.Now()}
    if env.Teranode == nil || env.Teranode.Metrics == nil || env.Teranode.Health == nil {
        return skipMissing(res, "Teranode metrics or health client not configured")
    }

    // (1) /metrics returns 200 + parseable
    mfs, err := env.Teranode.Metrics.Scrape(ctx)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "Metrics endpoint returns 200 with Prometheus-format body",
        err == nil && len(mfs) > 0,
        fmt.Sprintf("scraped %d metric families; err=%v", len(mfs), err),
    ))

    // (2) Required metric categories
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

    // (3) /health/readiness returns 200 + parseable JSON
    rep, err := env.Teranode.Health.Readiness(ctx)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "Health endpoint returns 200 with JSON-parseable body",
        err == nil && rep.Status != "",
        fmt.Sprintf("status=%q services=%d err=%v", rep.Status, len(rep.Services), err),
    ))

    res.Status = deriveStatus(res.AcceptanceChecks)
    res.Duration = env.Now().Sub(res.StartedAt)
    return res
}
```

The `_count` suffix on histogram metrics is the Prometheus convention; SP3's metric scraper emits these as separate `MetricFamily` entries (`teranode_validator_transactions_count` is parsed alongside `_bucket` and `_sum`).

### 4.2 PC-3 — Message Format and Wire Protocol Verification

**Source plan §"Protocol Correctness Tests" → PC-3.** Captures risk R2. Acceptance criteria from FR-2. Severity Critical.

**Scope (per SP1 spec):** transaction-format scope only — raw P2P packet capture is out of scope.

**Method:**
1. Construct standard BSV transactions in three shapes: P2PKH, multisig (P2MS), OP_RETURN data carrier.
2. Submit via Teranode RPC `sendrawtransaction`; record returned txid.
3. Fetch each tx back via Teranode REST `/tx/{hash}` (binary). Verify byte-exact round-trip — the bytes the node holds match the bytes we sent (or are a canonical re-serialisation that re-parses to the same txid).
4. Mine a block via svnode-1's `generatetoaddress`. Wait for the block to propagate to teranode-1 (poll `getbestblockhash` until it changes, with timeout).
5. Fetch the block via Teranode REST `/block/{hash}`. Re-parse with `bt.NewBlockFromBytes` (libsv). Verify the three test txs are in it.

**Implementation outline:**

```go
func RunPC3(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "PC-3", Title: "Message Format and Wire Protocol Verification", Severity: matrix.SeverityCritical, StartedAt: env.Now()}
    if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil ||
       env.SVNode == nil || env.SVNode.RPC == nil || env.TxGen == nil {
        return skipMissing(res, "client(s) not configured")
    }

    funder := env.TxGen
    builder := funder.Builder()

    // Bootstrap UTXO if needed.
    if funder.Balance() < 100_000_000 {
        if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil {
            res.Status = testrunner.StatusError
            res.Err = "bootstrap: " + err.Error()
            return res
        }
        // Mine to confirm the bootstrap tx.
        if _, err := mineBlocks(ctx, env, 1); err != nil {
            res.Status = testrunner.StatusError; res.Err = err.Error(); return res
        }
    }

    // (1+2+3) Build, submit, fetch back — three shapes.
    addrScript, _ := txgen.P2PKHScript(funder.Address())
    txs := []struct {
        name string
        build func() (txgen.BuildResult, error)
    }{
        {"P2PKH",     func() (txgen.BuildResult, error) { return builder.BuildP2PKH(txgen.BuildRequest{Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}}, FeeRate: 500}) }},
        {"P2MS",      func() (txgen.BuildResult, error) { /* 2-of-3 dummy pubkeys */ }},
        {"OP_RETURN", func() (txgen.BuildResult, error) { return builder.BuildOpReturn(txgen.BuildRequest{Outputs: nil, FeeRate: 500}, []byte("PC-3 round-trip")) }},
    }
    var txids []string
    for _, t := range txs {
        bres, err := t.build()
        if err != nil { /* AcceptanceCheck fail; continue */ }
        // submit
        returnedTxid, err := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
        // confirm = bres.TxID
        // fetch back via REST
        fetched, err := env.Teranode.REST.GetTxBytes(ctx, returnedTxid)
        // parse fetched, recompute txid, compare
        // record AcceptanceCheck "Tx <name> round-trips with matching txid"
        funder.Confirm(bres.Inputs, bres.Change)
        txids = append(txids, returnedTxid)
    }

    // (4) Mine a block.
    minedHashes, err := mineBlocks(ctx, env, 1)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "Block mined and propagated to Teranode",
        err == nil && len(minedHashes) == 1,
        fmt.Sprintf("hashes=%v err=%v", minedHashes, err),
    ))

    // Wait for teranode-1's tip to advance.
    if err := waitForTeranodeTip(ctx, env.Teranode.RPC, minedHashes[0], 30*time.Second); err != nil {
        res.AcceptanceChecks = append(res.AcceptanceChecks, fail("Teranode tip reached mined block within 30s", err.Error()))
    } else {
        res.AcceptanceChecks = append(res.AcceptanceChecks, ok("Teranode tip reached mined block within 30s", "tip="+minedHashes[0]))
    }

    // (5) Fetch the block, parse, verify our txs are present.
    blockBytes, err := env.Teranode.REST.GetBlockBytes(ctx, minedHashes[0])
    parsedBlock, err2 := bt.NewBlockFromBytes(blockBytes)  // exact lib symbol confirmed at impl time
    txidSet := make(map[string]bool, len(parsedBlock.Txs))
    for _, t := range parsedBlock.Txs {
        txidSet[hex.EncodeToString(t.TxIDBytes())] = true
    }
    for i, txid := range txids {
        present := txidSet[txid]
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            fmt.Sprintf("Block contains test tx %d (%s)", i, txid[:10]),
            present,
            fmt.Sprintf("present=%v", present),
        ))
    }

    res.Status = deriveStatus(res.AcceptanceChecks)
    res.Duration = env.Now().Sub(res.StartedAt)
    res.SatisfiesRequirements = []string{"FR-2"}
    return res
}
```

Helpers `mineBlocks` and `waitForTeranodeTip` live in `tests/helper.go`.

**Mining helper:**

```go
// tests/helper.go
func mineBlocks(ctx context.Context, env *testrunner.Env, n int) ([]string, error) {
    addr, err := env.SVNode.RPC.GetNewAddress(ctx)  // small helper; may use Call directly
    if err != nil { return nil, err }
    return env.SVNode.RPC.GenerateToAddress(ctx, n, addr)
}

func waitForTeranodeTip(ctx context.Context, rpc *teranode.RPCClient, want string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        h, err := rpc.GetBestBlockHash(ctx)
        if err == nil && h == want {
            return nil
        }
        select { case <-ctx.Done(): return ctx.Err(); case <-time.After(500*time.Millisecond): }
    }
    return fmt.Errorf("teranode tip never reached %s", want)
}
```

`svnode.RPCClient.GetNewAddress` and `GenerateToAddress` are added in this sub-project.

### 4.3 NEW-NFR11 — Transport Security and Authentication Probe

**Source: derived from NFR-11.** Severity Advisory.

**Method:**
1. For each Teranode endpoint URL (rpc, rest, notifications, metrics, health) plus svnode-1 RPC:
   - Resolve the scheme.
   - If `https://` or `wss://`: TLS handshake; record version + cipher.
   - If `http://` or `ws://`: record as a finding (plain transport).
2. Probe authentication:
   - Try unauthenticated request to a known protected endpoint (Teranode RPC `getbestblockhash` should require Basic Auth per SP2).
   - Try authenticated request; record which succeed.
3. Parse rate-limit headers if present (overlaps NEW-NFR13).

**Implementation:**

```go
func RunNEWNFR11(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-NFR11", Title: "Transport Security and Authentication Probe", Severity: matrix.SeverityAdvisory, StartedAt: env.Now()}
    if env.Teranode == nil { return skipMissing(res, "Teranode client not configured") }

    // (1) URL-by-URL probe
    urls := []struct{ name, raw string }{
        {"teranode.rpc", env.Cfg.Teranode.RPCURL},
        {"teranode.rest", env.Cfg.Teranode.RESTURL},
        {"teranode.notifications", env.Cfg.Teranode.NotificationURL},
        {"teranode.metrics", env.Cfg.Teranode.MetricsURL},
        {"teranode.health", env.Cfg.Teranode.HealthURL},
        {"svnode.rpc", env.Cfg.SVNode.RPCURL},
    }
    for _, u := range urls {
        if u.raw == "" { continue }
        parsed, _ := url.Parse(u.raw)
        switch parsed.Scheme {
        case "https", "wss":
            // dial with tls.Dial, record version + cipher
            tlsInfo, err := probeTLS(ctx, parsed)
            res.AcceptanceChecks = append(res.AcceptanceChecks, required(
                fmt.Sprintf("[%s] TLS handshake succeeded with version >= 1.2", u.name),
                err == nil && tlsInfo.Version >= tls.VersionTLS12,
                fmt.Sprintf("version=%v cipher=%v err=%v", tlsInfo.Version, tlsInfo.Cipher, err),
            ))
        case "http", "ws":
            // Per Q1=A: record as a finding, mark as Pass with note.
            res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
                fmt.Sprintf("[%s] transport scheme is %q", u.name, parsed.Scheme),
                "regtest plain transport — production deployment must terminate TLS in front of this endpoint",
            ))
        case "tcp":
            // SVNode ZMQ — not in scope for TLS.
        }
    }

    // (2) Auth: unauth attempt should fail; auth attempt should succeed.
    if env.Teranode.RPC != nil {
        // Unauth — strip credentials by constructing a fresh client with empty user/pass.
        rawNoAuth, _ := teranode.NewRPCClient(env.Cfg.Teranode.RPCURL, "", "", env.Logger)
        _, errNoAuth := rawNoAuth.GetBestBlockHash(ctx)
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            "Teranode RPC rejects unauthenticated requests with 401",
            errNoAuth != nil && (strings.Contains(errNoAuth.Error(), "401") || strings.Contains(errNoAuth.Error(), "unauthorized")),
            fmt.Sprintf("err=%v", errNoAuth),
        ))

        _, errAuth := env.Teranode.RPC.GetBestBlockHash(ctx)
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            "Teranode RPC accepts authenticated requests",
            errAuth == nil,
            fmt.Sprintf("err=%v", errAuth),
        ))
    }

    res.Status = deriveStatus(res.AcceptanceChecks)
    res.Duration = env.Now().Sub(res.StartedAt)
    res.SatisfiesRequirements = []string{"NFR-11"}
    return res
}
```

Note on Q1=A: plain-HTTP transport schemes are recorded as `Pass: true` with a detail note explaining the regtest posture. The criterion that's still strictly checked is auth-rejects-unauth on the RPC endpoint, which DOES work in our docker stack (Basic Auth is required).

`probeTLS` is a tiny helper using `tls.Dial`:

```go
func probeTLS(ctx context.Context, u *url.URL) (struct{ Version uint16; Cipher string }, error) {
    addr := u.Host
    if !strings.Contains(addr, ":") { addr += ":443" }
    d := &net.Dialer{Timeout: 5*time.Second}
    rawConn, err := d.DialContext(ctx, "tcp", addr)
    if err != nil { return struct{...}{}, err }
    defer rawConn.Close()
    cfg := &tls.Config{ServerName: u.Hostname()}
    tlsConn := tls.Client(rawConn, cfg)
    if err := tlsConn.HandshakeContext(ctx); err != nil { return struct{...}{}, err }
    s := tlsConn.ConnectionState()
    return struct{ Version uint16; Cipher string }{s.Version, tls.CipherSuiteName(s.CipherSuite)}, nil
}
```

### 4.4 NEW-NFR13 — Rate Limit Discovery and Error Semantics

**Source: derived from NFR-13.** Severity Advisory.

**Method:**
1. Issue probe requests at slowly increasing rate against `getbestblockhash` until server returns a rate-limit response or the configured ceiling is reached.
2. On first rate-limit response: record status, retry-after header, body.
3. Wait the indicated retry-after period; verify normal service resumes.
4. Report observed limit.

**Implementation (per Q2=B):**

```go
func RunNEWNFR13(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-NFR13", Title: "Rate Limit Discovery and Error Semantics", Severity: matrix.SeverityAdvisory, StartedAt: env.Now()}
    if env.Teranode == nil || env.Teranode.RPC == nil { return skipMissing(res, "Teranode RPC not configured") }

    maxRate := env.Cfg.Limits.NFR13MaxProbeRate          // default 1000 req/s
    duration := env.Cfg.Limits.NFR13ProbeDuration         // default 5s
    if maxRate <= 0 || duration <= 0 {
        return skipMissing(res, "rate-limit probe disabled in config")
    }

    deadline := env.Now().Add(duration)
    interval := time.Second / time.Duration(maxRate)

    var (
        sent       uint64
        succeeded  uint64
        firstLimit error
        firstStatus int
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
            // Look for rate-limit-shaped errors.
            if rl, ok := classifyRateLimit(err); ok {
                firstLimit = err
                firstStatus = rl
                break
            }
        }
        if firstLimit != nil {
            break
        }
    }

    if firstLimit == nil {
        // No limit hit — observed: no_limit_reached.
        res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
            "Rate-limit probe completed without hitting a limit",
            fmt.Sprintf("sent=%d succeeded=%d max_rate=%d duration=%v observed=no_limit_reached",
                sent, succeeded, maxRate, duration),
        ))
        if res.Observations == nil { res.Observations = map[string]any{} }
        res.Observations["sent"] = sent
        res.Observations["succeeded"] = succeeded
        res.Observations["limit_observed"] = false
    } else {
        // Hit a limit — record diagnostics.
        res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
            "Rate limit observed",
            fmt.Sprintf("sent=%d succeeded=%d firstStatus=%d firstErr=%v",
                sent, succeeded, firstStatus, firstLimit),
        ))
        if res.Observations == nil { res.Observations = map[string]any{} }
        res.Observations["limit_observed"] = true
        res.Observations["limit_status"] = firstStatus
        res.Observations["limit_error"] = firstLimit.Error()

        // Retry-after waits — for SP5 we only assert that a non-error response eventually comes back
        // within a reasonable cap (60s). The test does not parse retry-after headers itself; the
        // RPC client's error type doesn't surface them. SP10 can revisit.
        time.Sleep(2 * time.Second)
        _, err := env.Teranode.RPC.GetBestBlockHash(ctx)
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            "Service resumes after brief wait",
            err == nil,
            fmt.Sprintf("err=%v", err),
        ))
    }

    res.Status = deriveStatus(res.AcceptanceChecks)
    res.Duration = env.Now().Sub(res.StartedAt)
    res.SatisfiesRequirements = []string{"NFR-13"}
    return res
}

func classifyRateLimit(err error) (status int, isLimit bool) {
    // HTTP 429, or RPC error code that smells like a limit.
    s := err.Error()
    switch {
    case strings.Contains(s, "429"):
        return 429, true
    case strings.Contains(s, "rate limit"), strings.Contains(s, "too many requests"):
        return 429, true
    case strings.Contains(s, "503"):
        return 503, true
    }
    return 0, false
}
```

**Config additions:**

```go
// config/config.go — add to Limits struct
type Limits struct {
    // ... existing fields
    NFR13MaxProbeRate  int           `yaml:"nfr13_max_probe_rate"`     // requests/second (default 1000)
    NFR13ProbeDuration time.Duration `yaml:"nfr13_probe_duration"`     // total probe duration (default 5s)
}
```

Defaults applied in `applyDefaults`:

```go
if c.Limits.NFR13MaxProbeRate == 0 {
    c.Limits.NFR13MaxProbeRate = 1000
}
if c.Limits.NFR13ProbeDuration == 0 {
    c.Limits.NFR13ProbeDuration = 5 * time.Second
}
```

`config.example.yaml` and `config.docker.yaml` get the matching keys (commented in the example, set in docker config).

## 5. SVNode RPC additions

`internal/svnode/rpc.go` extends with:

```go
func (c *RPCClient) GetNewAddress(ctx context.Context) (string, error) {
    var s string
    return s, c.caller.Call(ctx, "getnewaddress", nil, &s)
}

func (c *RPCClient) GenerateToAddress(ctx context.Context, n int, addr string) ([]string, error) {
    var hashes []string
    return hashes, c.caller.Call(ctx, "generatetoaddress", []any{n, addr}, &hashes)
}
```

Plus tests covering both (using the existing `httptest.Server` + canned-response pattern).

## 6. Verification & testing strategy

### 6.1 Unit tests (under `make test`)

- The four `tests/*.go` files have no separate unit tests — they're integration tests run against the docker stack. A small `tests/tests_test.go` runs `go vet` and confirms the package compiles.
- `internal/svnode/rpc_test.go` extended with two new test cases (`TestRPC_GetNewAddress`, `TestRPC_GenerateToAddress`) using httptest fakes.
- `config/validate_test.go` extended with a case for the new NFR-13 fields.

### 6.2 Live integration (under `make compose-test`)

Operator runs:

```bash
make compose-up        # boots stack, runs bootstrap
make compose-test      # runs the suite against the stack
make compose-down      # tears down
```

`make compose-test` exit code:
- After SP5: `2` (CONDITIONAL_GO) likely — Critical PC-3 should pass and contributes; Important OPS-3 should pass; Advisory NEW-NFR11/13 don't affect verdict; remaining Critical tests (PC-1, PC-2, IBD-2, INTER-1, INTER-2, CLIENT-1, CLIENT-3) are still NOT_RUN → INCOMPLETE.
- Realistically: still `3` (INCOMPLETE) until SP8 lands the rest of the Critical tests, OR until reviewer overrides cover IBD-1 and the doc-review rows.

### 6.3 SP5 done-check (`scripts/sp5-done-check.sh`)

```bash
make build lint test verify
./scripts/sp{1,2,3,4}-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

# Live: requires compose stack up
if [ "${SP5_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml --only OPS-3,PC-3,NEW-NFR11,NEW-NFR13 || true
    test -s report.json
    # Verify each test produced a Result (not NOT_RUN)
    for id in OPS-3 PC-3 NEW-NFR11 NEW-NFR13; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        [ "$status" != "NOT_RUN" ] && [ -n "$status" ] || { echo "FAIL: $id status=$status"; exit 1; }
    done
    make compose-down
fi
echo "SP5 done-check passed."
```

`SP5_LIVE=1` runs the full live path; without it, only static checks run.

## 7. Definition of done

- All four `tests/<id>.go` files exist with the verbatim Objective/Method/Acceptance source-plan comment block at the top.
- `tests/helper.go` contains `mineBlocks`, `waitForTeranodeTip`, `ok`, `fail`, `required`, `skipMissing`, `deriveStatus`.
- `internal/svnode/rpc.go` has `GetNewAddress` + `GenerateToAddress` with passing tests.
- `config/config.go` has `Limits.NFR13MaxProbeRate` and `Limits.NFR13ProbeDuration` with defaults.
- `config.docker.yaml` and `config.example.yaml` have the matching keys (set in docker; commented in example).
- `cmd/teranode-acceptance/register.go` registers all four tests in alphabetical order.
- `scripts/sp5-done-check.sh` exists; static path passes.
- `make build lint test verify` exits 0; SP1-SP4 + SP4-DOCKER static done-checks all pass.
- `superpowers:code-reviewer` approves.

## 8. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | PC-3's mining wait (30s) is too short on a slow host | Increase to 60s; the test's `waitForTeranodeTip` timeout is configurable in code. SP10 may move it to config. |
| B | Teranode v0.15.0-beta-2 may have renamed an OPS-3 metric since SP2 discovery (commit `11f5fa6a8…`) | OPS-3 uses 5 metric-name lookups — any rename surfaces as a test FAIL. Operator inspects `/metrics` and updates the constant set. Document in `docs/compose.md`'s version-note section. |
| C | NEW-NFR13 may DOS-style hammer the docker stack at 1000 req/s for 5s | 5000 RPC calls is well within Teranode's `rpc_max_clients=3` × per-call timeout 30s ceiling. If observed flakiness, drop default to 100 req/s. |
| D | NEW-NFR11's TLS probe always reports "regtest plain HTTP" — the test never exercises a TLS code path | Acceptable for SP5; the TLS probe code is reachable, just not driven. SP10 (or a future dedicated TLS sub-project) can stand up an stunnel sidecar for proof. |
| E | PC-3's P2MS variant may fail validation on Teranode if bare multisig is non-standard in regtest | Build doc CLIENT-2 already says CLIENT-2 may skip if extended format not advertised; PC-3 tests *standard* P2MS which Teranode accepts per SP2 discovery (`getrawmempool` + `sendrawtransaction` work). If P2MS rejects, mark that one of the three round-trip checks Pass=false; PC-3 overall may degrade. |
| F | The bootstrap funder address derived from the privkey=1 mainnet WIF won't be recognised by SV node's wallet | Bootstrap shell already does `sendtoaddress` to the derived string; the SV node accepts any well-formed address. Confirmed in SP4-DOCKER design. |

## 9. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Plain HTTP in NEW-NFR11 → Pass with note | per user (Q1=A) |
| 2 | Rate-limit probe ceiling configurable via `Limits.NFR13MaxProbeRate` and `Limits.NFR13ProbeDuration` | per user (Q2=B) |
| 3 | `GenerateToAddress` extends `*svnode.RPCClient` | per user (Q3=A) |
| 4 | Each test in its own file in `tests/` package | drafter — matches build doc §4 |
| 5 | Shared helpers in `tests/helper.go` (not a sub-package) | drafter — keeps tests/ flat |
| 6 | Test files have a verbatim Objective/Method/Acceptance source-plan comment block | drafter — build doc §10 demands |
| 7 | `Result.AcceptanceChecks` populated per source-plan bullet | drafter — HTML reporter renders these |
| 8 | Skip when sub-clients are nil (SP3 pattern) | drafter |
| 9 | `mineBlocks` and `waitForTeranodeTip` live in `tests/helper.go` (no `internal/regtest/` sub-package) | drafter |

## 10. Out-of-scope reminders

SP5 is four tests. Testing the testing of those tests (meta-meta) — golden-file diffs of expected reports — is YAGNI. Verification is empirical: the live path against the docker stack tells us whether the tests work. SP6 (5 more tests) and SP7-SP9 are unaffected by SP5's choices.
