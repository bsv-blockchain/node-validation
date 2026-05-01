# SP7 — Tx-Generation Tests (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP7 / 11 — NEW-FR7, NEW-NFR7, INTER-2
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-30
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1, SP2, SP3, SP4, SP4-DOCKER, SP5, SP6
**Status:** awaiting user review

---

## 1. Purpose

Land the first heavy-tx-generation tests, including the first new Critical row to flip from `NOT_RUN` to a real result. After SP7, 2 of 8 Critical rows have data (PC-3 from SP5; INTER-2 from SP7).

The three tests share two building blocks: (a) `BuildChain` from SP4 (already used in PC-3), and (b) a new "splitter" pattern — a single tx with N outputs that gives the funder N spendable UTXOs in one mined block.

## 2. Scope

### In scope

- `tests/new_fr7.go` — NEW-FR7 (Advisory): 25-deep unconfirmed chain.
- `tests/new_nfr7.go` — NEW-NFR7 (Advisory): 100 idle read-op repetitions, byte-identical responses; load-condition checks deferred to SP9 with note.
- `tests/inter2.go` — INTER-2 (Critical): 1000 txs across 3 submission patterns (Teranode-only, SVNode-only, both), ≥99% propagation in 10s.
- `internal/txgen/funder.go` — extend with `ConfirmMulti(spent []UTXO, newOutputs []UTXO)`. Existing `Confirm` becomes a thin wrapper.
- `internal/txgen/builder.go` — extend with `BuildSplitter(funder, n int, satsPerOutput uint64, feeRate uint64) (BuildResult, error)` for INTER-2's 1000-UTXO bootstrap.
- `tests/helper.go` — extend with `pollMempoolUntil(ctx, rpc, wantTxIDs, timeout)` returning the set of txids seen.
- `cmd/teranode-acceptance/register.go` — register the 3 new tests (12 total now).
- `scripts/sp7-done-check.sh`.

### Out of scope

- 100 TPS / 500 TPS load conditions for NEW-NFR7 — deferred to SP9 (PERF-1 builds TPS-ramp infrastructure).
- INTER-1 (mixed-network consensus, 14-day observation) — SP9.
- Sustained over-1000-tx workloads — INTER-2 caps at 1000 per source plan.
- Per-tx size variation > 4 size buckets — SP9 PERF-1 covers more variety.

## 3. Architecture

```
suite.Run(ctx)
    │
    ├── tests.RunNEWFR7      build chain (depth ≥25); submit to Teranode; verify SV Node mempool
    ├── tests.RunNEWNFR7     read 3 RPCs × 100 iterations each; assert byte-identical
    └── tests.RunINTER2      splitter mines 1000 UTXOs; 333/333/334 submission split; poll mempools
```

Each test follows the SP5/SP6 shape (verbatim source-plan comment block, `defer Duration`, `skipMissing` when nil, `Result.AcceptanceChecks` per criterion, `deriveStatus` at end).

### 3.1 Splitter pattern

INTER-2 needs ~1000 spendable UTXOs to broadcast 1000 distinct txs without UTXO contention. The simplest path: one "splitter" transaction with N outputs.

```go
// Builder.BuildSplitter constructs a transaction with `n` outputs, each
// paying `satsPerOutput` to the funder's own address. The funder's
// SelectInputs picks enough UTXOs to cover n*satsPerOutput + fee.
//
// On Confirm/ConfirmMulti, the funder's UTXO set is replaced with the
// n new outputs (plus any change).
func (b *Builder) BuildSplitter(n int, satsPerOutput uint64, feeRate uint64) (BuildResult, error)
```

Internally, `BuildSplitter` calls `BuildP2PKH` with `n` identical-script outputs.

### 3.2 ConfirmMulti API

```go
// ConfirmMulti marks `spent` UTXOs as no longer available and registers
// every UTXO in `newOutputs` as spendable. Used by tests that mine
// transactions creating multiple outputs (e.g. the INTER-2 splitter).
//
// Confirm(spent, change) is preserved as a wrapper:
//   ConfirmMulti(spent, []UTXO{}) when change == nil
//   ConfirmMulti(spent, []UTXO{*change}) otherwise.
func (f *Funder) ConfirmMulti(spent []UTXO, newOutputs []UTXO)
```

After the splitter tx is mined, the test fills in the UTXO records:

```go
splitter, _ := builder.BuildSplitter(1000, 100_000, 500)
_, _ = env.Teranode.RPC.SendRawTransaction(ctx, splitter.HexTx)
mineBlocks(ctx, env, 1)
waitForTeranodeTip(...)

// Compose the 1000 new UTXOs.
newUTXOs := make([]UTXO, 1000)
for i := 0; i < 1000; i++ {
    newUTXOs[i] = UTXO{
        TxID:     splitter.TxID,
        Vout:     uint32(i),
        Satoshis: 100_000,
        Script:   addrScript,
    }
}
funder.ConfirmMulti(splitter.Inputs, newUTXOs)
```

## 4. Per-test designs

### 4.1 NEW-FR7 — Unconfirmed Tx Chain Acceptance (Advisory)

**Source: derived from FR-7. Captures R1.**

**Method:**
1. Build a chain of `Cfg.Limits.FR7ChainDepth` (default 25) dependent unconfirmed txs via `Builder.BuildChain`.
2. Submit each link via Teranode RPC `sendrawtransaction`; record per-link acceptance.
3. Wait briefly for P2P propagation (5s).
4. Get SV Node `getrawmempool`; verify all 25 chain txids are present.
5. Mine 1 block via svnode-1 `generatetoaddress`.
6. Wait for tip propagation; fetch the block from Teranode REST; verify all 25 chain txs are in it.

**Acceptance criteria (FR-7):**
- Chain of depth ≥25 fully accepted into Teranode mempool.
- Chain visible in SV Node mempool within `default_propagation` seconds (consistency check).
- All chain members eventually mined without intermediate confirmations.
- Behaviour consistent with SV Node (= mempool visibility per Q1=A).

### 4.2 NEW-NFR7 — Deterministic Behaviour (Advisory)

**Source: derived from NFR-7.**

**Method (idle conditions only per Q2=A):**
1. Pick three pure read operations:
   - `getbestblockhash` (no params)
   - `getblock <known hash>` (verbosity=1, returns JSON)
   - `getrawtransaction <known confirmed txid>` (verbose=0, returns hex)
2. For each, capture a baseline response.
3. Repeat each `Cfg.Durations.NewNFR7Iterations` (default 100) times.
4. Verify every iteration's response is byte-identical to the baseline.

**Acceptance criteria (NFR-7):**
- All 3 read ops byte-identical across 100 iterations (Pass).
- No load-induced variation in result shape, timing, or error rate beyond <0.5% variance — **deferred to SP9 with note**: "100/500 TPS load conditions require PERF-1 infrastructure".
- Errors fall into well-defined codes — Pass if no errors observed during the 300 idle calls.

**Implementation notes:**
- Choose the "known hash" by calling `getbestblockhash` once at test start. The choice is deterministic per run.
- Choose the "known txid" by parsing the block's first tx (coinbase). Coinbase txid is stable.
- Comparison: byte-identical via `bytes.Equal` on `json.RawMessage`.

### 4.3 INTER-2 — Cross-Implementation Tx Propagation (Critical)

**Source plan §"Interoperability Tests" → INTER-2. Captures R1, R2.**

**Method:**
1. Bootstrap the funder if balance is low.
2. Build splitter (1000 outputs × 100_000 sats); submit to Teranode; mine 1 block; wait for tip propagation; call `funder.ConfirmMulti` to register the 1000 new UTXOs.
3. Build 1000 simple P2PKH txs at 4 fee-rate buckets (250, 500, 1000, 2000 sat/kB) — 250 txs per bucket — and at 4 sizes (1-output, 2-output, 5-output, 10-output) — verify shape variety per source plan "various sizes and fee rates".
4. Partition: 333 to "SV Node only" group, 333 to "Teranode only" group, 334 to "both" group.
5. Concurrent submission with bounded parallelism (10 goroutines per group).
6. Poll each backend's `getrawmempool` every 250ms (per spec) with 10s timeout per direction.
7. Record per-tx propagation latencies; compute % observed cross-side within 10s.

**Acceptance criteria (INTER-2):**
- ≥99% of "Teranode only" group appears in SV Node mempool within `Cfg.Durations.DefaultPropagation` (default 10s).
- ≥99% of "SV Node only" group appears in Teranode mempool within 10s.
- "Both" group: no duplicates or conflicts (each tx has a unique txid; both backends receive it).
- No permanently lost or stuck txs after the test completes.

**Implementation notes:**
- The 1000-tx submission is the heaviest workload SP7 produces. The Teranode RPC has `rpc_max_clients = 3` (SP2 §1) — the test must serialise per-RPC-conn rather than swamping; 10 goroutines with each holding-and-reusing a single RPC connection works fine since `RPCClient` uses one shared HTTP transport.
- For the "to both" group, submit to Teranode first then SV Node 1ms later; record both responses but assert success for at least one.
- Hash collisions across groups are impossible since each tx spends a different UTXO.
- The test's overall wall time should be 30-60 seconds: ~10s splitter mining + ~30s tx generation + submission + ~10s propagation observation.

## 5. Verification & testing strategy

### 5.1 Unit tests

- `internal/txgen/funder_test.go` — extend with `TestConfirmMulti_addsAllOutputs` and `TestConfirmMulti_marksInputsSpent`.
- `internal/txgen/builder_test.go` — extend with `TestBuildSplitter_outputCount` and `TestBuildSplitter_satsPerOutput`.
- `tests/helper.go` — `pollMempoolUntil` is small enough that its semantics are exercised by the live INTER-2 test; no separate unit test.
- No new unit tests for `tests/new_fr7.go`, `tests/new_nfr7.go`, `tests/inter2.go` directly — they're integration tests run via `make compose-test`.

### 5.2 SP7 done-check (`scripts/sp7-done-check.sh`)

```bash
make build lint test verify
./scripts/sp{1,2,3,4,5,6}-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

go test -race ./tests/... ./internal/txgen/...

# Verify register.go has 12 tests
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP7_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only NEW-FR7,NEW-NFR7,INTER-2 || true
    test -s report.json
    for id in NEW-FR7 NEW-NFR7 INTER-2; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi
echo "SP7 done-check passed."
```

### 5.3 Coverage targets

- `internal/txgen` coverage stays ≥80% (existing 84.5%; ConfirmMulti and BuildSplitter add ~30 LoC + tests).
- No coverage requirement on `tests/` since the 3 SP7 tests run live only.

## 6. Definition of done

- All 3 test files exist with verbatim source-plan comment block.
- `Funder.ConfirmMulti` and `Builder.BuildSplitter` exist with passing unit tests.
- `tests/helper.go` `pollMempoolUntil` exists.
- `cmd/teranode-acceptance/register.go` registers all 12 tests in alphabetical order.
- `scripts/sp7-done-check.sh` static path exits 0.
- `make build lint test verify` exits 0; SP1-SP6 done-checks pass.
- Code review approves.

## 7. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | Splitter tx with 1000 outputs may hit Teranode policy limits (max output count, max tx size) | 1000 outputs × ~34 bytes ≈ 34 KB — well under any reasonable max-tx-size. Spec confirms via SP2 discovery; INTER-2 reports a clean Pass:false with detail if it surfaces. |
| B | 1000-tx workload may overwhelm shared Aerospike+Postgres+Kafka in docker | Default INTER2TxCount=1000 per Q3=A; if observed flakiness, operator drops to 500 via config override. |
| C | Teranode `rpc_max_clients=3` serialises submission | Test uses 10 goroutines but each share one HTTP transport; backpressure is invisible to test, just slower. Worst case: 1000-tx submission takes longer (5-10 min). |
| D | NEW-FR7's chain may exceed any future depth policy in Teranode | Default depth 25 per spec; configurable via `Cfg.Limits.FR7ChainDepth`. If Teranode rejects after some depth, test records the rejection point. |
| E | NEW-NFR7's "byte-identical" assertion may fail due to JSON field ordering nondeterminism | Teranode's RPC responses use Go's `json.Marshal` which sorts map keys; should be deterministic. If not, normalise via `json.Decode → json.Marshal` round-trip before comparison. |
| F | The "to both" submission group's race-condition between Teranode and SV Node | Test asserts at least one succeeds; if both error, that's a real finding. |
| G | UTXO confusion if INTER-2 runs after a previous test that consumed UTXOs | Each test bootstraps if balance is low; INTER-2 explicitly does the splitter step before the 1000-tx generation. Idempotent. |

## 8. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | `Funder.ConfirmMulti(spent, newOutputs)` API; `Confirm` becomes thin wrapper | per user (gap=A) |
| 2 | NEW-FR7 SV-Node parity = mempool visibility (single chain, Teranode-submitted) | per user (Q1=A) |
| 3 | NEW-NFR7 idle iterations only; load conditions deferred to SP9 | per user (Q2=A) |
| 4 | INTER-2 default 1000 txs per source plan | per user (Q3=A) |
| 5 | Splitter pattern: `Builder.BuildSplitter(n, satsPerOutput, feeRate)` | drafter |
| 6 | INTER-2 partitions: 333 / 333 / 334 (Teranode-only / SVNode-only / both) | drafter — matches source plan "1/3 each" |
| 7 | INTER-2 fee-rate buckets: 250/500/1000/2000 sat/kB | drafter — covers source plan "various fee rates" |
| 8 | INTER-2 size variations: 1/2/5/10 outputs | drafter — covers source plan "various sizes" |
| 9 | INTER-2 mempool poll: every 250ms per source plan; 10s timeout | source plan |
| 10 | NEW-NFR7 read ops: `getbestblockhash`, `getblock <hash>`, `getrawtransaction <txid>` | source plan |
| 11 | Test files alphabetical in `register.go`: INTER-2 → NEW-FR7 → NEW-NFR7 (lexicographic) | drafter |

## 9. Out-of-scope reminders

SP7 doesn't ship: 100/500 TPS NEW-NFR7 (SP9), INTER-1 mixed-network 14-day observation (SP9), full PERF-1 throughput ramp (SP9), CLIENT-1 1-hour observation (SP8).

The first live `make compose-test` after SP7 will, in the best case, produce:
- Critical: PC-3 PASS, INTER-2 PASS, 6 NOT_RUN → still INCOMPLETE.
- Important: OPS-3 PASS, CLIENT-2 PASS, PERF-1 NOT_RUN.
- Advisory: 8 of 8 with real measurements.

So `make compose-test` exit code: still 3 (INCOMPLETE) until SP8 lands the remaining Critical tests (PC-1, PC-2, IBD-2, INTER-1, CLIENT-1, CLIENT-3).
