# SP6 — Discovery-Gated Feature Tests (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP6 / 11 — Five tests for FR-8, FR-9, FR-10, FR-11, plus CLIENT-2
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-29
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1, SP2, SP3, SP4, SP4-DOCKER, SP5
**Status:** awaiting user review

---

## 1. Purpose

Land the next 5 acceptance tests against the docker stack. Per SP2 discovery, several of these surfaces are partially or fully absent in v0.15.0-beta-2; tests are written to record honest findings (FEATURE_NOT_AVAILABLE / partial PASS) rather than fail the run with surprising errors.

After SP6, the report has 9 tests producing real verdicts. Two of three Important rows (PERF-1 still missing; CLIENT-2 ships now) and 6 of 8 Advisory rows have measurements.

## 2. Scope

### In scope

- `tests/client2.go` — CLIENT-2 (Important).
- `tests/new_fr8.go` — NEW-FR8 (Advisory).
- `tests/new_fr9.go` — NEW-FR9 (Advisory).
- `tests/new_fr10.go` — NEW-FR10 (Advisory).
- `tests/new_fr11.go` — NEW-FR11 (Advisory).
- `internal/teranode/p2p_ws.go` — raw `/p2p-ws` WebSocket subscriber for NEW-FR9.
- `internal/teranode/p2p_ws_test.go` — httptest-fake-WS unit tests.
- `internal/teranode/clients.go` — wire P2PWSClient into `Clients`.
- `config/config.go` — add `Teranode.P2PWSURL`.
- `compose/docker-compose.yml` — expose port 9906 on each Teranode.
- `config.docker.yaml` — add `teranode.p2p_ws_url`.
- `config.example.yaml` — add the matching key.
- `cmd/teranode-acceptance/register.go` — register the 5 new tests.
- `cmd/teranode-acceptance/testdata/integration.yaml` — extend with the new key.
- `scripts/sp6-done-check.sh`.

### Out of scope

- Full libp2p subscriber (would consume the actual gossip topic). The `/p2p-ws` raw WebSocket is the lightest viable consumer per SP2 discovery.
- Spending output of an extended-format transaction (the P2MS / P2SH spend paths are exercised by SP9 if needed).
- Address-history pagination depth (FR-10 mentions it; absent on Teranode per SP2 — recorded as `Pass: false, Detail: "no /address/ route per discovery"`).
- 30-day uptime-style metrics (NFR-1 territory, `LONG_TERM_OBSERVATION`).

## 3. Architecture

```
suite.Run(ctx)
    │
    ├── tests.RunCLIENT2     submit standard + extended tx; verify both accepted
    ├── tests.RunNEWFR8      probe estimatefee; expect FEATURE_NOT_AVAILABLE
    ├── tests.RunNEWFR9      subscribe /p2p-ws → submit tx1, then conflicting tx2
    │                        → assert rejected_tx event arrives within 5s
    ├── tests.RunNEWFR10     latency probe of historical REST/RPC reads (p95 ≤ 100ms)
    └── tests.RunNEWFR11     mempool query: list works; entry/ancestors/descendants absent
```

Each test follows the SP5 shape (verbatim source-plan comment block, Result.AcceptanceChecks per criterion, skipMissing when clients are nil, deriveStatus at end).

### `internal/teranode/p2p_ws.go` shape

A small typed client mirroring `notifications.go`:

```go
type P2PWSClient struct {
    url    string
    ws     *websocket.Conn   // gorilla/websocket already transitively pulled by centrifuge-go
    rejectedTxs chan RejectedTxEvent
    blocks      chan P2PBlockEvent
    subtrees    chan P2PSubtreeEvent
    logger *slog.Logger
    closed atomic.Bool
}

type RejectedTxEvent struct {
    Timestamp  string `json:"timestamp"`
    Type       string `json:"type"`        // "rejected_tx" or "rejectedtx" — server-defined
    TxID       string `json:"tx_id"`
    Reason     string `json:"reason"`
    PeerID     string `json:"peer_id"`
    ClientName string `json:"client_name"`
}
type P2PBlockEvent struct {
    Type   string `json:"type"`
    Hash   string `json:"hash"`
    Height uint64 `json:"height"`
}
type P2PSubtreeEvent struct {
    Type string `json:"type"`
    Hash string `json:"hash"`
}

func NewP2PWSClient(rawURL string, logger *slog.Logger) (*P2PWSClient, error)
func (c *P2PWSClient) Connect(ctx context.Context) error
func (c *P2PWSClient) Close() error
func (c *P2PWSClient) RejectedTxs() <-chan RejectedTxEvent
func (c *P2PWSClient) Blocks()      <-chan P2PBlockEvent
func (c *P2PWSClient) Subtrees()    <-chan P2PSubtreeEvent
```

`Connect` dials the WebSocket and starts a goroutine that reads JSON messages, routes by `type`, and pushes to the matching channel. Buffered channels (64 capacity) drop on full with a logged warning — same pattern as `notifications.go`.

The exact `type` discriminator strings come from upstream `services/p2p/server_helpers.go:48-57,170-178` per SP2 — the client tries both `"rejected_tx"` and `"rejectedtx"` since the agent didn't pin the exact spelling. SP6 implementation can grep upstream once during build-time and pin.

### Config addition

```go
type Teranode struct {
    // ... existing fields
    P2PWSURL string `yaml:"p2p_ws_url"`  // ws://host:19906/p2p-ws
}
```

`teranode.NewClients` constructs `P2PWSClient` from this URL; nil-safe when the field is empty.

### Docker compose patch

Each Teranode service gets one extra port mapping:

```yaml
  teranode-1:
    ports:
      - "127.0.0.1:19292:9292"
      - "127.0.0.1:18090:8090"
      - "127.0.0.1:19091:9091"
      - "127.0.0.1:18000:8000"
      - "127.0.0.1:18444:18444"
      - "127.0.0.1:19905:9905"
      - "127.0.0.1:19906:9906"   # NEW: P2P HTTP / /p2p-ws
```

Plus `29906` for teranode-2 and `39906` for teranode-3.

## 4. Per-test designs

### 4.1 CLIENT-2 — Extended Transaction Format Support (Important)

**Source plan §"Client Integration Tests" → CLIENT-2.** Captures risks R2, R6.

**Method:**
1. Discovery (SP2) determined extended format is **always advertised** in v0.15.0-beta-2 — auto-extension is built in. So the test never skips for "not advertised".
2. Construct an extended-format transaction (txgen.BuildP2PKH then call `tx.ExtendedBytes()` directly via libsv); submit via Propagation HTTP `POST /tx`; verify accepted.
3. Construct a standard-format transaction (`tx.Bytes()`); submit; verify accepted.
4. Mine a block; verify both txs are mined.
5. Fetch the block via REST; verify both txs round-trip with matching txids.

**Acceptance criteria:**
- Extended-format tx accepted (Pass).
- Standard-format tx accepted (backward compat, Pass).
- Both txs mined into the same block (Pass).
- (Note in observations: REST returns non-extended bytes per discovery — not a failure, a documented finding.)

**Implementation note:** the propagation HTTP `POST /tx` endpoint (`http://teranode-1:8833/tx`, mapped to host `28833` — needs adding to compose) is one option. The simpler path is to use Teranode's RPC `sendrawtransaction` which (per SP2 agent-01) accepts both formats since it routes through the same validator pipeline. **Use the RPC path** to avoid exposing yet another port. Drop the propagation HTTP submission from the design.

Updated: tx submission is via `env.Teranode.RPC.SendRawTransaction(ctx, hexBytes)` for both formats.

### 4.2 NEW-FR8 — Fee Estimation (Advisory)

**Source: derived from FR-8.**

**Method per SP1 spec:** for each priority level (`economy`, `standard`, `priority`), call the fee-estimation endpoint, submit 20 transactions at exactly that fee rate, record inclusion latency, compute accuracy (% mined within 1-block horizon for `standard`).

**Discovery says:** `estimatefee` returns `ErrRPCUnimplemented (-1)`. There IS no fee-estimation endpoint. So:

**Implementation:**
1. Try `env.Teranode.RPC.Call(ctx, "estimatefee", []any{1}, &out)`.
2. If error with code -1 → `Status: FEATURE_NOT_AVAILABLE` with note citing discovery.
3. If error with any other code → `Pass: false` for "endpoint exists" check.
4. If success → record returned value, but flag as "unexpected — discovery said unimplemented".

The test won't generate the 20-tx-per-priority-level workload because there's no API to compare against. Skip those criteria with `FEATURE_NOT_AVAILABLE`.

```go
func RunNEWFR8(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-FR8", ...}
    if env.Teranode == nil || env.Teranode.RPC == nil { return skipMissing(res, ...) }

    var fee float64
    err := env.Teranode.RPC.Call(ctx, "estimatefee", []any{1}, &fee)
    if err == nil {
        // Surprising — record and check accuracy.
        res.AcceptanceChecks = append(res.AcceptanceChecks, ok(...))
        // Run the priority-level probe — but per discovery, only one level
        // exists, so accuracy can't be computed across tiers.
    } else if jsonrpc.IsErrorCode(err, -1) {
        // Expected — feature not available.
        res.Status = testrunner.StatusFeatureNotAvailable
        res.SkipReason = "estimatefee returns ErrRPCUnimplemented per SP2 discovery (handleUnimplemented at services/rpc/Server.go:162)"
        return res
    }
    // ...
    res.Status = deriveStatus(res.AcceptanceChecks)
    return res
}
```

Status `StatusFeatureNotAvailable` flows into the verdict logic same as `StatusNotRun` — Advisory, doesn't change verdict.

### 4.3 NEW-FR9 — Double-Spend Detection (Advisory)

**Source: derived from FR-9.**

**Method:**
1. Connect to `env.Teranode.P2PWS` (raw `/p2p-ws` WebSocket).
2. Construct two transactions spending the same UTXO (different outputs): tx1 and tx2.
3. Submit tx1 via Teranode RPC; expect success.
4. Subscribe to `RejectedTxs()` channel.
5. Submit tx2; expect synchronous error containing `"spent"` or `"conflicting"`.
6. Wait up to 5s for a `rejected_tx` event on the WebSocket carrying tx2's txid.
7. Mine a block (confirms tx1).

**Acceptance criteria:**
- Conflicting tx detected synchronously by RPC (tx2 submission errors).
- Notification delivered on `/p2p-ws` within 5s carrying tx2's txid.
- The "winning" tx (tx1) is the one mined.
- Low-confirmation case (tx mined, then conflicting tx submitted) — best-effort; **deferred** with note since it requires waiting a block for confirmation then submitting a conflicting tx that doesn't have a chance to win, which on regtest is awkward (blocks come on demand). Mark as `Pass: false, Detail: "low-confirmation path deferred per SP6 spec §4.3"`.

```go
func RunNEWFR9(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-FR9", ...}
    if env.Teranode == nil || env.Teranode.RPC == nil ||
       env.Teranode.P2PWS == nil || env.TxGen == nil ||
       env.SVNode == nil {
        return skipMissing(res, "client(s) not configured")
    }

    p2pws := env.Teranode.P2PWS
    if err := p2pws.Connect(ctx); err != nil {
        return errorResult(res, fmt.Errorf("connect /p2p-ws: %w", err))
    }
    defer p2pws.Close()

    funder := env.TxGen
    builder := funder.Builder()
    if funder.Balance() < 100_000_000 {
        if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil { return errorResult(res, err) }
        if _, err := mineBlocks(ctx, env, 1); err != nil { return errorResult(res, err) }
        time.Sleep(2 * time.Second)
    }

    addrScript, _ := txgen.P2PKHScript(funder.Address())

    // Build tx1 spending an explicit UTXO.
    utxos := funder.SnapshotUTXOs()  // expose via Funder if not already
    pinned := utxos[0]
    tx1, err := builder.BuildP2PKH(txgen.BuildRequest{
        Outputs:   []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
        FeeRate:   500,
        SpendUTXO: &pinned,
    })
    if err != nil { return errorResult(res, err) }

    // Build tx2 spending the SAME UTXO with a different output (e.g. different satoshis).
    tx2, err := builder.BuildP2PKH(txgen.BuildRequest{
        Outputs:   []txgen.Output{{Script: addrScript, Satoshis: 2_000}},
        FeeRate:   500,
        SpendUTXO: &pinned,
    })
    if err != nil { return errorResult(res, err) }

    // Submit tx1 — should succeed.
    _, err = env.Teranode.RPC.SendRawTransaction(ctx, tx1.HexTx)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "tx1 (first-seen) accepted by Teranode RPC",
        err == nil, fmt.Sprintf("err=%v", err)))
    if err != nil { res.Status = deriveStatus(res.AcceptanceChecks); return res }

    // Submit tx2 — should fail synchronously with conflicting/spent.
    _, err = env.Teranode.RPC.SendRawTransaction(ctx, tx2.HexTx)
    detected := err != nil && (strings.Contains(strings.ToLower(err.Error()), "spent") ||
        strings.Contains(strings.ToLower(err.Error()), "conflict"))
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "tx2 (conflicting) rejected synchronously by Teranode RPC",
        detected, fmt.Sprintf("err=%v", err)))

    // Wait up to 5s for a rejected_tx event matching tx2's txid.
    expectedTxID := hex.EncodeToString(tx2.TxID[:])
    timer := time.NewTimer(5 * time.Second)
    defer timer.Stop()
    var event *teranode.RejectedTxEvent
loop:
    for {
        select {
        case e := <-p2pws.RejectedTxs():
            if normalizeTxID(e.TxID) == expectedTxID {
                event = &e
                break loop
            }
        case <-timer.C:
            break loop
        case <-ctx.Done():
            return errorResult(res, ctx.Err())
        }
    }
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "Notification on /p2p-ws within 5s carrying tx2's txid",
        event != nil,
        fmt.Sprintf("event=%+v", event)))

    // Mine; verify tx1 is the one mined.
    minedHashes, err := mineBlocks(ctx, env, 1)
    // ... fetch block, verify tx1's txid is present, tx2 is not ...
    
    // Low-confirmation path is deferred.
    res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
        "Low-confirmation double-spend handled (FR-9 criterion 3 part 2)",
        "deferred: regtest mining cadence makes this awkward to exercise; tracked for SP9"))

    res.Status = deriveStatus(res.AcceptanceChecks)
    return res
}
```

Note `Funder.SnapshotUTXOs()` — that's a public accessor we need to expose. SP4 has it as `snapshotUTXOs` (lowercase, internal). SP6 promotes it to public. (Trivial change.)

`normalizeTxID` is a tiny helper handling LE/BE byte-order convention if upstream emits the txid in either form.

### 4.4 NEW-FR10 — Historical Data Access Latency (Advisory)

**Source: derived from FR-10.** Acceptance: p95 ≤ 100ms for tx-by-id, block-by-height, block-by-hash.

**Method:** Adapt sample size to available regtest history (per Q2=A).

```go
func RunNEWFR10(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-FR10", ...}
    if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil { return skipMissing(res, ...) }

    // Discover available history.
    info, err := env.Teranode.RPC.GetBlockchainInfo(ctx)
    if err != nil { return errorResult(res, err) }

    // Sample size: min(50, info.Blocks).
    sampleN := 50
    if info.Blocks < int64(sampleN) { sampleN = int(info.Blocks) }
    if sampleN < 5 {
        return skipMissing(res, fmt.Sprintf("regtest only has %d blocks; need ≥5 for meaningful latency sample", info.Blocks))
    }

    // Collect block hashes.
    blockHashes := make([]string, 0, sampleN)
    for h := int64(1); h <= int64(sampleN); h++ {
        hash, err := env.Teranode.RPC.GetBlockHash(ctx, h)
        if err == nil { blockHashes = append(blockHashes, hash) }
    }

    // Collect tx ids from those blocks (coinbase txids are easiest).
    txids := make([]string, 0, sampleN)
    for _, bh := range blockHashes {
        var blk struct {
            Tx []string `json:"tx"`
        }
        if err := env.Teranode.RPC.Call(ctx, "getblock", []any{bh, 1}, &blk); err == nil && len(blk.Tx) > 0 {
            txids = append(txids, blk.Tx[0]) // coinbase
        }
    }

    res.Observations = map[string]any{
        "block_hashes_sample_size": len(blockHashes),
        "txids_sample_size":        len(txids),
    }

    // Measure latency for each query type.
    txP95 := measureLatency(ctx, "tx-by-id", txids, func(id string) error {
        _, err := env.Teranode.REST.GetTxBytes(ctx, id); return err
    })
    blockHashP95 := measureLatency(ctx, "block-by-hash", blockHashes, func(h string) error {
        _, err := env.Teranode.REST.GetBlockBytes(ctx, h); return err
    })
    // block-by-height — REST doesn't expose this per SP2 §2 gap 1; use search?q=<height>.
    blockHeightP95 := measureLatency(ctx, "block-by-height", intRange(1, sampleN), func(h string) error {
        _, err := env.Teranode.REST.Search(ctx, h); return err
    })

    target := time.Duration(env.Cfg.Limits.FR10LatencyTargetMs) * time.Millisecond
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        fmt.Sprintf("tx-by-id p95 ≤ %v", target),
        txP95 <= target, fmt.Sprintf("p95=%v target=%v", txP95, target)))
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        fmt.Sprintf("block-by-hash p95 ≤ %v", target),
        blockHashP95 <= target, fmt.Sprintf("p95=%v target=%v", blockHashP95, target)))
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        fmt.Sprintf("block-by-height p95 ≤ %v (via /search)", target),
        blockHeightP95 <= target, fmt.Sprintf("p95=%v target=%v", blockHeightP95, target)))

    // Address history: per SP2 absent.
    res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
        "Address history queries supported with pagination",
        "absent in v0.15.0-beta-2 per SP2 discovery §2 gap 1; no /address/ route"))

    res.Status = deriveStatus(res.AcceptanceChecks)
    return res
}
```

`measureLatency` takes a list and a probe-function; runs each, records elapsed; returns p95.

### 4.5 NEW-FR11 — Mempool Query (Advisory)

**Source: derived from FR-11.**

Per SP2 §10:
- `getrawmempool` works (returns `[]string` of hashes).
- `getmempoolentry` is in `rpcUnimplemented` (handler absent).
- `getmempoolancestors` / `getmempooldescendants` not registered.
- `getmempoolinfo` is `handleUnimplemented`.

**Method:**
1. Submit a known set of transactions covering varying fee rates and one explicit ancestor/descendant chain (e.g. depth 3 from txgen.BuildChain).
2. Call `getrawmempool` — verify the submitted txids appear.
3. Call `getmempoolentry <txid>` — expect ErrUnimplemented → record FEATURE_NOT_AVAILABLE.
4. Call `getmempoolancestors`, `getmempooldescendants` — both expected absent (unknown command); record FEATURE_NOT_AVAILABLE.
5. Call `getmempoolinfo` — expect ErrUnimplemented → record FEATURE_NOT_AVAILABLE.

```go
func RunNEWFR11(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "NEW-FR11", ...}
    if env.Teranode == nil || env.Teranode.RPC == nil || env.TxGen == nil || env.SVNode == nil { return skipMissing(res, ...) }

    funder := env.TxGen
    builder := funder.Builder()
    // Bootstrap if needed; mine a confirmation block.
    // ... (same as PC-3)

    addrScript, _ := txgen.P2PKHScript(funder.Address())

    // Submit a chain of 3 dependent txs (parent → child1 → child2).
    chain, err := builder.BuildChain(txgen.BuildRequest{
        Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
        FeeRate: 500,
    }, 3)
    if err != nil { return errorResult(res, err) }
    var chainTxIDs []string
    for _, c := range chain {
        if _, err := env.Teranode.RPC.SendRawTransaction(ctx, c.HexTx); err != nil {
            return errorResult(res, fmt.Errorf("submit chain tx: %w", err))
        }
        chainTxIDs = append(chainTxIDs, hex.EncodeToString(c.TxID[:]))
    }

    // (1) getrawmempool — verify presence.
    mempool, err := env.Teranode.RPC.GetRawMempool(ctx)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "getrawmempool returns []string",
        err == nil, fmt.Sprintf("err=%v len=%d", err, len(mempool))))
    seen := map[string]bool{}
    for _, id := range mempool { seen[id] = true }
    for i, id := range chainTxIDs {
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            fmt.Sprintf("Chain tx %d (%s…) appears in getrawmempool", i, id[:10]),
            seen[id], fmt.Sprintf("present=%v", seen[id])))
    }

    // (2) getmempoolentry — expected absent.
    var entry json.RawMessage
    err = env.Teranode.RPC.Call(ctx, "getmempoolentry", []any{chainTxIDs[0]}, &entry)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "getmempoolentry: per SP2 absent (FEATURE_NOT_AVAILABLE recorded)",
        err != nil,  // we EXPECT an error per discovery
        fmt.Sprintf("err=%v", err)))

    // (3,4) getmempoolancestors / getmempooldescendants — also expected absent.
    for _, m := range []string{"getmempoolancestors", "getmempooldescendants"} {
        var raw json.RawMessage
        err := env.Teranode.RPC.Call(ctx, m, []any{chainTxIDs[1]}, &raw)
        res.AcceptanceChecks = append(res.AcceptanceChecks, required(
            fmt.Sprintf("%s: absent per SP2 discovery", m),
            err != nil,
            fmt.Sprintf("err=%v", err)))
    }

    // (5) getmempoolinfo — expected unimplemented.
    var info json.RawMessage
    err = env.Teranode.RPC.Call(ctx, "getmempoolinfo", nil, &info)
    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "getmempoolinfo: per SP2 unimplemented",
        err != nil, fmt.Sprintf("err=%v", err)))

    // Mine to clean up.
    _, _ = mineBlocks(ctx, env, 1)

    res.Status = deriveStatus(res.AcceptanceChecks)
    return res
}
```

The "expected absent" checks pass when the call errors (positive assertion of negative behaviour). This makes the test report honest: features absent are explicitly recorded as findings, not silently passed.

## 5. Verification & testing strategy

### 5.1 Unit tests

- `internal/teranode/p2p_ws_test.go` — `httptest`-based fake WebSocket server emits `{"type":"rejected_tx", "tx_id":"abcd"}` JSON; client decodes to channel; coverage target ≥70%.
- `tests/tests_test.go` — extended with helpers for the FR-10 percentile math (small `measureLatency` unit test using a synthetic timing source).
- No new tests for FR-8/9/10/11 directly (live integration only).

### 5.2 SP6 done-check (`scripts/sp6-done-check.sh`)

```bash
make build lint test verify
./scripts/sp{1,2,3,4}-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh

go test -race ./tests/... ./internal/teranode/...

# Verify the 5 new tests register
go test -race ./cmd/teranode-acceptance/... -run TestRegisterTests_SP6RegistersNine

if [ "${SP6_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only CLIENT-2,NEW-FR8,NEW-FR9,NEW-FR10,NEW-FR11 || true
    test -s report.json
    for id in CLIENT-2 NEW-FR8 NEW-FR9 NEW-FR10 NEW-FR11; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi
echo "SP6 done-check passed."
```

## 6. Definition of done

- All 5 test files exist with verbatim source-plan comment block.
- `internal/teranode/p2p_ws.go` exists with `httptest`-based unit tests.
- `internal/teranode/clients.go` constructs `P2PWSClient` from `cfg.Teranode.P2PWSURL`.
- `compose/docker-compose.yml` exposes port 9906 on each Teranode (host `19906/29906/39906`).
- `config.docker.yaml`, `config.example.yaml`, `cmd/teranode-acceptance/testdata/integration.yaml` all include the new `p2p_ws_url` key.
- `cmd/teranode-acceptance/register.go` registers all 9 tests in alphabetical order.
- `Funder.SnapshotUTXOs()` is now public (was private).
- `scripts/sp6-done-check.sh` static path exits 0.
- Code review approves.

## 7. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | The `/p2p-ws` `type` discriminator string may differ from agent-08's report | Client tries both `"rejected_tx"` and `"rejectedtx"`; SP6 implementation greps upstream once and pins. |
| B | NEW-FR9's `expected_txid` byte-order mismatch (Bitcoin LE display vs server-emitted format) | `normalizeTxID` helper accepts both forms. |
| C | Teranode v0.15.0-beta-2 may have *implemented* `estimatefee` / `getmempoolentry` since SP2 (built from `11f5fa6a8…`) | NEW-FR8 / NEW-FR11 detect this and record positive findings rather than fail. |
| D | NEW-FR9's "low-confirmation" criterion is deferred | Documented in test detail; SP9 picks it up. |
| E | NEW-FR10 has small sample size on regtest (~10-20) — p95 noisier | Acceptable for `Pass/Fail` against a 100ms target since regtest latency is sub-millisecond on local docker; report observation includes sample sizes. |
| F | `/p2p-ws` may also publish unrelated channels at high volume, congesting the rejected-tx channel | Buffered channels (64); `dispatch()` drops on full and logs warning. NEW-FR9 only needs ONE event matching expected txid. |
| G | Port 9906 exposure may collide if operator already has something on host:19906 | Standard 127.0.0.1: binding lets the operator override via env if needed; document in `compose.md`. |

## 8. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | NEW-FR9 subscribes to `/p2p-ws` raw WebSocket (not full libp2p) | per user correction |
| 2 | NEW-FR10 sample size adapts to available regtest history | per user (Q2=A) |
| 3 | Add `internal/teranode/p2p_ws.go` (small client, ~150 LoC) | drafter |
| 4 | Expose port 9906 on each Teranode in compose | drafter, required for #1 |
| 5 | `Funder.SnapshotUTXOs()` promoted to public | drafter, needed by NEW-FR9 |
| 6 | `StatusFeatureNotAvailable` used in NEW-FR8 + parts of NEW-FR11 | drafter, matches SP1's enum |
| 7 | NEW-FR9 low-confirmation criterion deferred | drafter, tracked as risk D |
| 8 | CLIENT-2 uses RPC `sendrawtransaction` (not Propagation HTTP) | drafter — avoids exposing yet another port |

## 9. Out-of-scope reminders

SP6 doesn't ship: full libp2p subscriber, Propagation HTTP submission, address-history pagination, fee-rate market measurement (no upstream surface), multi-block reorg double-spend testing.
