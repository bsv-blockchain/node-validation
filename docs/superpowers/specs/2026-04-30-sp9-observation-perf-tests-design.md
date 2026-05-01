# SP9 — Long-Observation + Perf Tests (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP9 / 11 — PC-1, INTER-1, PERF-1
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-30
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1–SP8
**Status:** awaiting user review

---

## 1. Purpose

Land the final 3 acceptance tests — the two long-observation Critical tests (PC-1 parallel-node comparison, INTER-1 mixed-network consensus) and the Important throughput baseline (PERF-1).

After SP9, all 19 in-scope tests have measurements. With reviewer overrides on the 5 documentation/contractual rows (IBD-1, FR-4, NFR-1, NFR-8, NFR-9), the verdict can flip to `GO`.

## 2. Scope

### In scope

- `tests/pc1.go` — PC-1 (Critical): observe both nodes' tips for `Cfg.Durations.PC1Observation` (7d default, 30min --short); deterministic-batch tx submission; 20-block sliding window; **deliberate reorg induction near the end** via `invalidateblock` + `generatetoaddress` (per Q1=B).
- `tests/inter1.go` — INTER-1 (Critical): same shape over `Cfg.Durations.INTER1Observation` (14d default, 1h --short); track block flow across the 6-node mesh; orphan-rate and propagation-time observations; same reorg-induction sub-phase.
- `tests/perf1.go` — PERF-1 (Important): TPS ramp through `Cfg.Limits.PERF1RampSteps` (default `[10, 50, 100, 250]` on docker, full `[10, 50, 100, 250, 500, 1000]` opt-in via raised `PERF1MaxTPS`).
- `config/config.go` — add `Limits.PERF1RampSteps []int` (per Q2=A).
- `internal/observer/` (new tiny package) — shared block-tail + tip-comparison helper used by PC-1 + INTER-1.
- `cmd/teranode-acceptance/register.go` — register all 19 tests.
- `scripts/sp9-done-check.sh`.

### Out of scope

- Resuming a long-running test from disk-persisted state (build doc §7.6 mentions it; this is YAGNI for a 1h test, would matter for the real 14-day INTER-1 run on testnet).
- 7-day / 14-day full runs against the docker stack — operator runs `--short` for fast feedback; the longer durations target a stable testnet deployment.
- Per-implementation orphan-rate distinction — on docker the only miner is svnode-1; "orphans by implementation" measurement requires Teranode to mine too, which we do briefly during the reorg-induction phase.

## 3. Architecture

```
suite.Run(ctx)
    │
    ├── tests.RunPC1     observe phase (poll tips, compare) → induce-reorg phase (invalidate+regenerate) → verify-convergence
    ├── tests.RunINTER1  observe phase (poll all 6 mesh nodes, track block flow) → induce-reorg phase → verify
    └── tests.RunPERF1   for each rate in ramp: ramp up → submit at rate for D seconds → measure latency → cool down
```

Each follows the SP5–SP8 shape (verbatim source-plan comment block, AcceptanceChecks per criterion, deriveStatus).

### 3.1 Reorg induction (per Q1=B)

Both PC-1 and INTER-1 include a **deliberate reorg induction phase** near the end of their observation window. Mechanism uses RPC methods both backends implement (per SP2 discovery):

1. Capture baseline: all 6 nodes at the same height `H` with hash `B0`.
2. Mine 1 block on svnode-1 → `B1` at height `H+1`. Wait for propagation (poll until all 6 nodes report `B1`).
3. Call `invalidateblock(B1)` on teranode-1 — rolls teranode-1 back to `H`/`B0`.
4. Mine 2 blocks on teranode-1 via `generatetoaddress` → `T1` (H+1), `T2` (H+2). Now teranode-1 has the longer chain.
5. Wait for re-propagation: the rest of the mesh (still at `B1`) observes teranode-1's longer chain and reorgs to `T2`.
6. Verify all 6 nodes converge to `T2` within `Cfg.Durations.DefaultPropagation × 2` (default 20s).
7. Cleanup: optionally call `reconsiderblock(B1)` everywhere — ineffective once `T2` is canonical.

### 3.2 Shared observer package

`internal/observer/observer.go`:

```go
package observer

type TipSnapshot struct {
    Time   time.Time
    Hash   string
    Height int64
    Source string  // "teranode-1", "svnode-1", etc.
}

type Observer struct {
    rpcs   map[string]TipReader  // labelled
    interval time.Duration
    out    chan TipSnapshot
    logger *slog.Logger
}

type TipReader interface {
    GetBestBlockHash(ctx context.Context) (string, error)
    GetBlockchainInfo(ctx context.Context) (json.RawMessage, error)
}

func NewObserver(rpcs map[string]TipReader, interval time.Duration, logger *slog.Logger) *Observer
func (o *Observer) Run(ctx context.Context, until time.Time) []TipSnapshot
func (o *Observer) DivergenceCount(snapshots []TipSnapshot) int
func (o *Observer) ReorgsObserved(snapshots []TipSnapshot) []ReorgEvent
```

PC-1 / INTER-1 both consume `Observer`. The package is small (~200 LoC) but earns its keep by sharing the polling discipline and snapshot analysis.

The interface `TipReader` lets the observer accept both `*teranode.RPCClient` and `*svnode.RPCClient` without coupling to either — same pattern as `pollMempoolUntil` from SP7.

### 3.3 PERF-1 ramp config

```go
// config/config.go — add to Limits
type Limits struct {
    // ... existing fields
    PERF1RampSteps []int `yaml:"perf1_ramp_steps"`  // [10, 50, 100, 250] default on docker
}
```

`config.docker.yaml`:
```yaml
  perf1_ramp_steps: [10, 50, 100, 250]
```

Operator override for full ramp:
```yaml
  perf1_max_tps: 1000
  perf1_ramp_steps: [10, 50, 100, 250, 500, 1000]
```

Test logic filters `RampSteps` to `<= PERF1MaxTPS`.

## 4. Per-test designs

### 4.1 PC-1 — Parallel Node Comparison

**Source plan §"Protocol Correctness Tests" → PC-1.** Captures R2, R3. Severity Critical.

**Method (compressed for `--short` 30min):**

1. **Observe phase** (first 80% of window):
   - Poll all 6 nodes' best tips every 5s using `observer`.
   - Every (window/4) submit a deterministic batch of 5 test txs to both Teranode-1 and SVNode-1; compare per-tx accept/reject via `compare.CompareCategories`.
   - Track block flow: record every observed (height, hash) per node.
2. **Induce-reorg phase** (last 20% of window):
   - Execute the 7-step reorg induction from §3.1.
   - Measure convergence time.
3. **Verify** consistency across the run.

**Acceptance criteria (PC-1):**
- Zero divergence in accepted/rejected blocks during observe phase (= all 6 nodes always agree on the canonical chain at any sample).
- Zero divergence in tx validity decisions across all batches.
- Both Teranode-1 and SVNode-1 converge to the same tip within `Cfg.Durations.DefaultPropagation × 2` of the induced reorg.

### 4.2 INTER-1 — Mixed-Network Consensus

**Source plan §"Interoperability Tests" → INTER-1.** Captures R2, R7. Severity Critical.

**Method (compressed for `--short` 1h):**

1. **Observe phase** (first 80% of window):
   - Poll best-block headers on all 6 nodes every 5s.
   - Track every block by hash; record per-node first-arrival time.
   - Compute cross-arrival latency: time between one node's first-seen and the slowest node's first-seen of the same block.
2. **Induce-reorg phase** (last 20% of window):
   - Same 7-step procedure from §3.1.
   - Measure: do blocks from teranode-1 (T1, T2 mined via `generatetoaddress`) propagate to all SV nodes? Do they get accepted?
3. **Compute orphan rate**: any block-hash observed by *some* nodes but never reaching `getbestblockhash` on others → orphan/stale.

**Acceptance criteria (INTER-1):**
- No persistent forks lasting >1 block (= no two nodes report different best-block hashes for >1 block-time observation).
- Comparable orphan rate (within 2×) — measured during the induce-reorg phase: T1 may end up orphaned on some nodes if T2 races, count how many orphans each implementation generates.
- Blocks from both implementations accepted with comparable frequency: in induce-reorg phase, teranode-1's T2 must reach all 6 nodes; svnode-1's B1 must reach all 6 (before being orphaned).

### 4.3 PERF-1 — Throughput and Latency Baseline

**Source plan §"Performance and Stress Tests" → PERF-1.** Captures R5. Severity Important.

**Method (compressed for `--short`: 30s per rate × 4 rates = 2 minutes default):**

1. For each `rate` in `Cfg.Limits.PERF1RampSteps` filtered to `<= PERF1MaxTPS`:
   1. Bootstrap funder + splitter to ensure ≥`rate × duration` UTXOs available.
   2. Submit txs at the target rate for `Cfg.Durations.PERF1PerRate`.
   3. For each tx, record: submit time, mempool-visible time (poll every 250ms), in-block time (poll for inclusion).
   4. Sample resource usage from `metrics_url` every 10s during the rate step.
   5. Cool down: wait for mempool to drain.
2. Compute per-rate p50, p95 of (submit→in-block) latency.
3. Compare with SVNode baseline: same rate ramp run against svnode-1.
4. Sample block-propagation latency: time from teranode-1 first-seen to svnode-1 first-seen.

**Acceptance criteria (PERF-1):**
- Median latency at each rate within 20% of SV Node baseline.
- p95 at the highest tested rate no more than 5× p95 at 100 TPS.
- Resource usage recorded (absence is a soft fail — Pass with detail rather than Fail).

**Implementation notes:**

- The "resource usage" check uses `MetricsScraper.Scrape(ctx)` (SP3) and reports CPU/memory if those metrics exist; per SP2 §5 they're partial — `teranode_blockassembly_*` give tx-volume; CPU/memory aren't exposed directly. Test reports what it can find with detail noting the gap.
- Hard-cap absolute submission rate at `Cfg.Limits.PERF1MaxTPS` per build doc §7.5 ("Refuse mainnet without `--allow-mainnet-load`"). The mainnet-load gate already lives in `config.Validate` (SP1).
- Self-throttle: if either node's mempool grows beyond a threshold (e.g. 10× rate × duration), pause submission to avoid runaway. Threshold is `Cfg.Limits.PERF1MaxTPS × Cfg.Durations.PERF1PerRate × 5`.

## 5. Verification & testing strategy

### 5.1 Unit tests

- `internal/observer/observer_test.go` — table tests for `DivergenceCount`, `ReorgsObserved` with synthetic snapshots.
- `tests/tests_test.go` — extended for any new helpers.
- The 3 SP9 tests run live only.

### 5.2 SP9 done-check (`scripts/sp9-done-check.sh`)

```bash
make build lint test verify
./scripts/sp{1..8}-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

go test -race ./internal/observer/... ./tests/...

go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP9_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only PC-1,INTER-1,PERF-1 || true
    test -s report.json
    for id in PC-1 INTER-1 PERF-1; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
    done
    make compose-down
fi
echo "SP9 done-check passed."
```

Note: `--short` SP9 live run is ~1.5h (PC-1 30min + INTER-1 1h + PERF-1 ~3min) on the docker stack. CI shouldn't run this; operator does.

## 6. Definition of done

- All 3 test files exist with verbatim source-plan comment blocks.
- `internal/observer/` package exists with passing unit tests.
- `Limits.PERF1RampSteps` configured with default + per-config-file overlays.
- All 4 YAML configs (`testdata/minimal`, `config.example`, `config.docker`, `integration testdata`) have the new key.
- `cmd/teranode-acceptance/register.go` registers all 19 tests in alphabetical order.
- `make build lint test verify` exits 0; SP1–SP8 done-checks pass; SP9 static done-check exits 0.
- Code review approves.

## 7. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | Reorg induction race-condition: teranode-1's `T1` block may propagate before `T2`, causing partial reorg observation | Test waits ≤30s for full convergence; reports the actual observed reorg path, not just final state |
| B | `invalidateblock` may fail if teranode-1 is currently mining or busy | Test retries once after 2s; if still failing, records ERROR for that acceptance check |
| C | PERF-1 ramp on local docker may saturate before reaching higher rates | Configurable via `PERF1RampSteps`; default ramps stop at 250 TPS; operator opts into 500/1000 |
| D | Reorg-induction phase's `generatetoaddress` requires Teranode wallet/coinbase address — Teranode supports it (SP2 §1) but needs an address to mine to | Use `funder.Address()` (txgen WIF address) as the mining destination |
| E | INTER-1's "orphan rate" calculation requires distinguishing per-implementation orphans, but only svnode-1 mines normally — orphans only appear in the reorg-induction phase | Acknowledge in Detail; report orphans-during-reorg as the only data |
| F | PC-1's "deterministic batch" of test txs requires reproducible UTXO set | Each batch uses fresh splitter outputs from a per-batch sub-bootstrap; the determinism is "same shape and quantity", not byte-identical |
| G | Long-running tests (30min+) may hit per-test timeout (default 30 min from SP1) | PC-1 obs window is 30min; needs 5-min headroom for setup + reorg phase. Set `Cfg.TestTimeout` to 60 min when running with `--only PC-1,INTER-1,PERF-1`, OR raise per-test timeout default for SP9. |

## 8. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Reorg induction via invalidateblock + generatetoaddress | per user (Q1=B) |
| 2 | PERF-1 ramp configurable via `Limits.PERF1RampSteps`; default `[10, 50, 100, 250]` on docker | per user (Q2=A) |
| 3 | PC-1 / INTER-1 run sequentially in the suite | per user (Q3=A) |
| 4 | Shared `internal/observer/` package | drafter — DRY |
| 5 | Reorg induction phase = last 20% of observation window | drafter — leaves 80% for pure observation |
| 6 | PC-1 deterministic batches every (window/4) — 4 batches per run | drafter |
| 7 | INTER-1 orphan-rate measurement during reorg phase only | drafter — see risk E |
| 8 | PERF-1 self-throttle if mempool > 5× expected | drafter |
| 9 | --short test timeout raised in CLI when PC-1/INTER-1 selected | drafter — 60min via `--test-timeout 1h` operator-supplied |

## 9. Out-of-scope reminders

SP9 doesn't ship: 7-day / 14-day actual long runs (operator does these against testnet); per-implementation orphan-rate discrimination outside the reorg phase; CPU/memory metrics in PERF-1 (require Teranode metrics that don't exist per SP2 §5); resume-from-disk for paused observations.

After SP9 + reviewer overrides, verdict math:
- Critical: 8/8 PASS (assuming live run is green).
- Important: 3/3 PASS.
- Advisory: 8/8 measured.
- Documentation-review rows (IBD-1, FR-4, NFR-1, NFR-8, NFR-9) PASS via overrides.

→ `GO` (exit code 0). Acceptance suite finally green-lights.
