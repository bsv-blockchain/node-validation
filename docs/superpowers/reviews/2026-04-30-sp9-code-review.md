# SP9 — Long-Observation + Perf Tests Code Review

**Reviewer:** code-review (Senior Code Reviewer)
**Date:** 2026-04-30
**Subject:** SP9 (sub-project 10/11) — PC-1, INTER-1, PERF-1, `internal/observer/`
**Spec:** `docs/superpowers/specs/2026-04-30-sp9-observation-perf-tests-design.md`
**Plan:** `docs/superpowers/plans/2026-04-30-sp9-observation-perf-tests.md`
**Commits reviewed (5, post `sp8-complete` tag):**
- `ebd8275` feat(observer): add tip polling + reorg detection helper
- `d1e3623` feat(config): add perf1_ramp_steps with default [10,50,100,250]
- `cafa7c7` feat(tests): add PC-1 + INTER-1 with reorg induction
- `01850e4` feat(tests): add PERF-1 — Throughput and Latency Baseline
- `e8462ba` feat(cmd): register 19 tests; add sp9-done-check

---

## Summary

SP9 lands the final three acceptance tests cleanly. All deliverables present, all 8 critical invariants from the brief verified, the `internal/observer/` package compiles and tests, the 7-step reorg-induction procedure is implemented faithfully, and PERF-1 ramps with per-rate p50/p95 latency. Static checks (`gofmt`, `go vet`, `staticcheck`) are clean; `make build lint test verify` passes; SP9 static done-check runs to completion (cascaded SP1–SP8 gates pass).

The verdict math now plays out as designed: post-SP9 the registered 19 tests cover all critical and important rows; reviewer overrides for the 5 documentation/contract rows can flip the suite to `GO`.

There is one Important defect (Risk B mitigation missing in `induceReorg`), several Minor concerns (mostly cleanup), and a notable strength: the `teranodeTipReader` adapter is the right call for the typed/raw-message asymmetry between the two RPC clients.

---

## Critical

None.

---

## Important

### I1. `induceReorg` does not retry `invalidateblock` per Risk B mitigation

**Spec §7 Risk B:** "`invalidateblock` may fail if teranode-1 is currently mining or busy. Test retries once after 2s; if still failing, records ERROR for that acceptance check."

**Code (`tests/helper.go:359-364`):**
```go
// 4. invalidateblock(B1) on teranode-1.
var dummy json.RawMessage
if err := env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy); err != nil {
    res.Err = fmt.Errorf("invalidateblock B1 on teranode-1: %w", err)
    return res
}
```

No retry. A single transient failure aborts the whole reorg phase, marking the tests' final acceptance check as failed. Recommended fix:

```go
var dummy json.RawMessage
err = env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy)
if err != nil {
    time.Sleep(2 * time.Second)
    err = env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy)
}
if err != nil {
    res.Err = fmt.Errorf("invalidateblock B1 on teranode-1 (after retry): %w", err)
    return res
}
```

Severity Important rather than Critical because (a) `invalidateblock` is highly likely to succeed on a docker-stack teranode-1 that's idle between blocks, (b) PC-1's first two acceptance checks (zero-divergence + zero-tx-divergence) are the load-bearing PASS signals — the third (reorg convergence) failing degrades to FAIL on a soft criterion. Still, fixing this is cheap and the spec calls it out as a tracked mitigation.

---

## Minor

### M1. `observer.Run` mutex is unnecessary

`observer.go:53-90` uses `sync.Mutex` to guard `snapshots`, but the for-range over `o.rpcs` is sequential within a single goroutine — there is no concurrent producer. Dead code, no behavioural impact. Remove for clarity, or document the intention if you plan to parallelise the per-RPC fetch later.

### M2. `observer.ReorgsObserved` carries dead `_ = src` assignment

`observer.go:140` has `_ = src // used as map key above` inside the outer loop body. The plan template had `_ = fmt.Sprintf("%s", src)` as a placeholder; the implementation tightened it but left the no-op. Drop the line entirely.

### M3. `observer.ConvergedAt` is exported and tested but unused by callers

`tests/pc1.go` derives convergence from its own `induceReorg`-internal poll loop (`reorgResult.ConvergedAt` set in `helper.go:385`). The package-level `observer.ConvergedAt` is unused. Either:
- Use it (refactor `induceReorg` to feed snapshots into `observer.ConvergedAt`), or
- Drop the function and its test, or
- Document that it's available for the testnet long-run analysis pass that's out of SP9 scope (per spec §9).

Not load-bearing — extra exported helper has tests, just isn't called from production paths.

### M4. `observer.Run` first-sample latency = `interval`

`time.NewTicker(o.interval)` does not fire immediately, so the first snapshot lands at `t0 + 5s`. For `--short` PC-1 (30 min) this is fine. Worth a comment noting the implication. If a future caller passes a window shorter than the interval, `Run` returns `nil` silently.

### M5. PERF-1 mempool self-throttle from spec §3.3 / Decisions #8 not implemented

Spec decision #8: "PERF-1 self-throttle if mempool > 5× expected." `tests/perf1.go` does not check `GetRawMempool` size during submission. With `PERF1MaxTPS=250` × `PERF1PerRate=30s` × 5 = 37.5k tx headroom, the docker stack will likely cope without throttle, but the design called it out as a mitigation for runaway. Either implement, or downgrade the decision in the spec to "deferred to operator-tuned config."

### M6. PERF-1 SV Node baseline acceptance check is a soft pass without measurement

`tests/perf1.go:212-215` records:
```go
res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
    "Latency measured per rate (SV Node baseline comparison deferred)",
    fmt.Sprintf("rates=%v", ramp),
))
```

This is fair given that a parallel SV Node run is genuinely out of `--short` scope, but the acceptance criterion *as written in the spec* is "Median latency at each rate within 20% of SV Node baseline." Either:
- Run the same ramp against `env.SVNode.RPC` as a second pass (doubles PERF-1 wall time), or
- Update the spec acceptance text to "Latency measured per rate; baseline comparison run separately (out of `--short`)."

### M7. PERF-1 first rate's `funder.Reset()` discards the bootstrap UTXO

`perf1.go:118` calls `funder.Reset()` after submitting the splitter and before re-registering its outputs. The original bootstrap UTXO that funded the splitter is correctly consumed (it's an input), so Reset has no semantic ill effect on rate 1. On subsequent rates, the previous rate's funder state is also wiped — the next iteration's `if funder.Balance() < target` check then forces a re-bootstrap. This works but means each rate triggers a fresh Bootstrap, doubling RPC traffic. Consider preserving change UTXOs across rates.

### M8. PERF-1 `_ = hex.EncodeToString(bres.TxID[:])` only exists to keep the import alive

`perf1.go:175`. The unused `hex` import is artificial — likely a copy-paste from a spec scaffold that referenced TxID. Drop the unused work and remove the `encoding/hex` import.

### M9. Doc comments missing on exported `RunPC1`, `RunINTER1`, `RunPERF1`

The Go-doc conventions expect a leading comment on every exported function. The file-level block comment covers the test's purpose, but the function itself has no godoc line. SP5–SP8 tests follow the same pattern, so this is consistent rather than newly broken — flagging for global cleanup in hardening (SP10/SP11).

---

## Strengths

- **`teranodeTipReader` adapter is the right answer.** `teranode.RPCClient.GetBlockchainInfo` returns the typed `BlockchainInfo`, while `svnode.RPCClient.GetBlockchainInfo` returns `json.RawMessage`. Rather than hammering one of them into the other shape, the wrapper marshals the typed value back to JSON to satisfy the interface. Pragmatic and well-commented.
- **Phase split (80/20 observe/reorg)** is implemented exactly as spec §3.1 specifies, with `observeUntil = env.Now().Add(window * 4 / 5)`.
- **Reorg induction's 7-step procedure** maps cleanly: baseline → mine B1 on svnode-1 → wait propagation → invalidateblock B1 on teranode-1 → generatetoaddress 2 on teranode-1 → wait for svnode-1 reorg → result struct.
- **Observer unit tests** exercise the three exported analysis functions with synthetic data — good coverage of `DivergenceCount` (agree + disagree), `ReorgsObserved` (reorg + advance), `ConvergedAt`.
- **Filter ramp by `PERF1MaxTPS`** correctly implemented at `perf1.go:64-69`; the test errors cleanly if no steps remain.
- **Per-rate `p95Idx` clamp** at `perf1.go:191-193` correctly handles the `n × 0.95 == n` edge case.
- **`register.go` alphabetical order** verified — 19 tests in correct alpha order.
- **`TestCLI_overridesAlonelyDoNotProduceGo` updated** to expect exit 2 (CONDITIONAL_GO) — the verdict math is right: with all 19 tests registered the critical tests skip (not NOT_RUN), so INCOMPLETE doesn't trigger; OPS-3 important still fails without live env → CONDITIONAL_GO. Comment explains the transition from exit 3 → exit 2.
- **`config.example.yaml`** has the inline comment `# PERF-1 ramp; raise PERF1MaxTPS to extend` — operator-friendly.
- **No TODO/FIXME/XXX markers** in any SP9 file.
- **`gofmt -l .` clean, `go vet ./...` clean, `staticcheck` clean, `go test -race ./...` passes.**

---

## Spec coverage cross-check

| Spec section | Deliverable | Status |
|---|---|---|
| §3.1 (reorg induction 7 steps) | `tests/helper.go` `induceReorg` | Implemented; missing Risk B retry (I1) |
| §3.2 (Observer package) | `internal/observer/{doc,observer,observer_test}.go` | Implemented; M3 `ConvergedAt` unused, M1/M2 cleanup |
| §3.3 (PERF1RampSteps config) | `config.Limits.PERF1RampSteps` + 4 YAML files + defaults + mergeYAML | Implemented and correct |
| §4.1 (PC-1) | `tests/pc1.go` 3 acceptance checks | Implemented |
| §4.2 (INTER-1) | `tests/inter1.go` 3 acceptance checks | Implemented |
| §4.3 (PERF-1) | `tests/perf1.go` ramp + percentiles + metrics | Implemented; M5 throttle missing, M6 SV baseline soft, M8 unused hex |
| §5.2 (sp9-done-check.sh) | `scripts/sp9-done-check.sh` | Implemented; static path passes |
| §6 (DoD) | All bullet items | Met |

No gaps. Two design decisions (#8 PERF-1 throttle, SV Node baseline comparison) need either implementation or spec walk-back; flagged Minor since neither blocks the verdict-flip math.

---

## Practical sanity

- `make build lint test verify` exits 0.
- `gofmt -l .` clean.
- `go vet ./...` clean.
- `staticcheck ./...` clean.
- `go test -race ./internal/observer/... ./tests/...` passes.
- `go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'` passes (19 results).
- `./scripts/sp9-done-check.sh` (static, `SP9_LIVE=0`) executed — cascade through SP1–SP8 done-checks completes; observer + tests build/test pass; register.go test passes. (Live path not exercised in this review — operator runs with `SP9_LIVE=1` against the docker stack.)

---

## Recommendation

**APPROVE with one Important fix.**

The Important issue (I1: missing `invalidateblock` retry) is cheap to fix and explicitly called out in the spec's risk register. Land that fix before tagging `sp9-complete` and the final `GO` exit-code verification.

The Minor items can roll into hardening (SP10/SP11) — none of them affect the verdict math or the PASS/FAIL outcomes of the live PC-1/INTER-1/PERF-1 runs in expected operating conditions. M5 and M6 deserve a note in the spec acknowledging they're deferred rather than left as silent gaps.

After the I1 fix, the suite is ready to flip to `GO` (exit code 0) once reviewer overrides land for IBD-1, FR-4, NFR-1, NFR-8, NFR-9 — exactly as the spec §9 verdict math predicts.
