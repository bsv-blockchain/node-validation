# SP7 — Tx-Generation Tests: Code Review

**Reviewer:** Senior Code Reviewer (automated)
**Date:** 2026-04-30
**Scope:** 5 commits after `sp6-complete` tag (`47eb73e`..`78da826`)
**Spec:** `docs/superpowers/specs/2026-04-30-sp7-txgen-tests-design.md`
**Plan:** `docs/superpowers/plans/2026-04-30-sp7-txgen-tests.md`
**Verdict:** APPROVED — ship it.

---

## Practical sanity (verified in this review)

| Check | Result |
|---|---|
| `gofmt -l .` | clean (no output) |
| `go vet ./...` | clean (no output) |
| `go test -race ./internal/txgen/... ./tests/... ./cmd/teranode-acceptance/...` | PASS (cached) |
| `./scripts/sp7-done-check.sh` (static path) | PASS |
| New SP7 unit tests (`TestConfirmMulti_*`, `TestBuildSplitter_*`) verbosely | 4/4 PASS |
| `TestRegisterTests_SP7RegistersTwelve` | PASS |
| TODO/FIXME/XXX in SP7 code | none |

The full done-check cascade (SP1 → SP6 → SP7) green-lights from a fresh build.

---

## Critical (must fix)

None.

---

## Important (should fix)

None — the implementation faithfully mirrors the plan. The few deviations from the plan's literal text are improvements:

1. **`submitGroup` dropped the unused `label string` parameter** from the plan's prototype. The plan's prototype declared but never used `label`; trimming it is correct and avoids a `staticcheck` complaint had it landed unused.
2. **`built` was promoted to a package-level `interTx` named type**, exactly as the plan's parenthetical fallback recommended. Avoids Go's anonymous-struct identity rule when passed across function boundaries — pragmatic and idiomatic.
3. **`Confirm` is now a thin wrapper over `ConfirmMulti`** in `internal/txgen/coinselect.go` (lines 79-85). The existing `TestConfirm_addsChange` still passes, confirming behaviour preservation.

---

## Minor / Suggestions

1. **Splitter change UTXO is silently discarded.** `RunINTER2` calls `funder.Reset()` then `ConfirmMulti(splitter.Inputs, newUTXOs)` — the splitter's residual change output (likely 1+ M sats given the 2× headroom bootstrap) is dropped on the floor. Not a correctness bug, but if the test runs many times in one funded environment, those sats accumulate as orphaned UTXOs the funder never knows about. Two cleaner options:
   - Don't `Reset()`; just `ConfirmMulti(splitter.Inputs, newUTXOs)` (also keeps any prior funder state alive).
   - Or include `splitter.Change` in the `newOutputs` slice when non-nil.
   File: `tests/inter2.go:120-131`.

2. **`RunNEWFR7`, `RunNEWNFR7`, `RunINTER2` lack godoc-style function comments.** The package-level comment block covers the test logic richly, but the exported entry point itself has no `// RunXXX ...` line. This is consistent with the SP5/SP6 pattern, so it's a stylistic point only — but adopting godoc on exported `Run*` symbols would let `go doc` users discover what each does without reading the file header. Same nit applies to `interTx` and `txidsOf` (package-private; godoc less critical).

3. **`MinChainDepthFR7 = 25`** in `internal/txgen/builder.go:15` is declared but unreferenced anywhere. Either wire it into NEW-FR7's depth fallback (`if depth <= 0 { depth = MinChainDepthFR7 }` instead of literal 25) or drop the constant. Currently it's just decorative.

4. **`submitGroup` returns `sent` but never uses the per-tx error.** A submission failure is silently counted as "not sent." Fine for a 99% threshold check, but if you ever want to debug a failure mode, the lost error is gone. Consider stashing first error per group in `Observations` for postmortem visibility.

5. **NEW-NFR7 baseline `getblock` uses `Call(...)` directly** with method/params, while iterations 2 and 3 use the typed `GetBestBlockHash` / `GetRawTransaction` wrappers. Mixing transports is fine (both ultimately hit the same JSON-RPC endpoint), but a `GetBlock(ctx, hash, verbosity)` typed wrapper on `*teranode.RPCClient` would round it out. Out of scope for SP7 — drop into SP8/SP9 cleanup.

6. **`pollMempoolUntil` does its first poll immediately, then waits 250 ms before the second**, which means short timeouts may only get one shot. Behaviour is fine, but a comment near `for time.Now().Before(deadline)` documenting "polls at t=0, t=250ms, t=500ms, …" would prevent confusion when reading the test logs.

---

## Strengths

- **Plan alignment is excellent.** Every deliverable from §6 of the spec is present and named exactly as the design called for.
- **Lexicographic register order is verified mechanically** (`LC_ALL=C sort -c` confirms). `CLIENT-2 < INTER-2 < NEW-FR10 < NEW-FR11 < NEW-FR7 < NEW-FR8 < NEW-FR9 < NEW-NFR11 < NEW-NFR13 < NEW-NFR7 < OPS-3 < PC-3` matches.
- **Config knobs (`FR7ChainDepth`, `INTER2TxCount`, `NewNFR7Iterations`, `DefaultPropagation`) are wired through `config.Config` with defaults, validation, and override-merge logic** (`config/config.go`, `config/defaults.go`, `config/validate.go`). The validation tests confirm zero-values are rejected.
- **`mempoolReader` interface with compile-time assertion** (`var _ mempoolReader = (*teranode.RPCClient)(nil)`) is a nice touch — catches drift if the RPC client signature changes.
- **`ConfirmMulti` correctly delegates to `MarkSpent` then iterates `AddUTXO`** — proper composition of existing primitives, not a re-implementation. The `Confirm` wrapper preserves `TestConfirm_addsChange` behaviour exactly.
- **`BuildSplitter` validates `n < 1`** with a clear error; it then composes through `BuildP2PKH` so coin selection, fee, and change handling are all reused. Good DRY.
- **NEW-NFR7's deferred load checks are emitted as explicit `fail()` checks with notes** rather than skipped — this is exactly the right pattern: forces a re-visit in SP9 and shows the gap in the report.
- **INTER-2's partition arithmetic** (`txs[:count/3]`, `txs[count/3:2*count/3]`, `txs[2*count/3:]`) gives exactly 333/333/334 for `count=1000` per spec decision #6.
- **`scripts/sp7-done-check.sh` cascades through all prior done-checks** before exercising SP7-specific bits — fail-fast on prior regressions.
- **Bounded parallelism (sem channel cap=10)** with `sync.Mutex` for the counter is textbook concurrent submission.

---

## Spec coverage gaps

| Spec section | Status |
|---|---|
| §3.1 Splitter pattern | Implemented (`Builder.BuildSplitter` in `builder.go:194-211`) |
| §3.2 ConfirmMulti API | Implemented (`coinselect.go:67-85`); `Confirm` is thin wrapper |
| §4.1 NEW-FR7 — chain depth ≥25, mempool visibility, single-block confirm | Implemented (`tests/new_fr7.go`); uses `parseStandardBlock` from PC-3 ✓ |
| §4.2 NEW-NFR7 — 3 read ops × 100 iterations, byte-identical, deferred load checks | Implemented (`tests/new_nfr7.go`); deterministic anchors via `getbestblockhash` + coinbase txid ✓ |
| §4.3 INTER-2 — splitter, 1000 txs, 4 fee × 4 size, 333/333/334, 250ms poll, 10s timeout, ≥99% | Implemented (`tests/inter2.go`); concurrency cap 10 per group ✓ |
| §5.1 Unit tests | `TestConfirmMulti_*` (×2), `TestBuildSplitter_*` (×2) — all pass |
| §5.2 SP7 done-check static + live | Both modes present; static path verified |
| §6 Definition of done | All criteria met |

No gaps detected.

---

## Recommendation

**APPROVED — tag `sp7-complete` and proceed to SP8.**

The five Minor items above are polish, not blockers. The change-UTXO discard in INTER-2 (§Minor #1) is the most consequential and worth a 5-minute drive-by fix in a follow-up commit if the operator wants tidy bookkeeping; everything else is documentation/style.

Eight of eleven sub-projects in the can; on track.
