# SP5 — Cheap Probe Tests — Code Review

**Reviewer:** superpowers:code-reviewer
**Reviewed at:** 2026-04-28
**Sub-project:** SP5 / 11
**Commits reviewed:** 8 commits between `sp4-docker-complete` and `75cd1a5`
**Spec:** `docs/superpowers/specs/2026-04-29-sp5-cheap-tests-design.md`
**Plan:** `docs/superpowers/plans/2026-04-29-sp5-cheap-tests.md`

## Verdict: APPROVE with one minor follow-up

`make build lint test verify` exits 0. `gofmt -l .` clean. `go vet ./...` clean. `./scripts/sp5-done-check.sh` static path exits 0 (SP1–SP4 + SP4-DOCKER static cascades all pass). All four tests register; `TestRegisterTests_SP5RegistersFour` passes. Implementation tracks the spec faithfully.

One minor cleanup item is described under **Minor**.

---

## Critical: none

No critical defects. All ten invariants from the review brief hold:

1. All deliverables exist at the expected paths.
2. Each test follows the specified shape (defer Duration, skip-when-nil, AcceptanceChecks per criterion, deriveStatus, SatisfiesRequirements / CapturedRisks where applicable).
3. PC-3 correctly: builds three shapes via SP4 Builder; submits via Teranode RPC `sendrawtransaction`; verifies returned txid against locally-computed; fetches back via REST and re-parses with `bt.NewTxFromBytes`; mines via svnode-1; waits 60s for tip propagation (per spec §8 risk A); fetches block via REST; parses with `parseStandardBlock` (header + VarInt + `bt.NewTxFromStream` loop); asserts the three test txs are in the block.
4. NEW-NFR11 plain-HTTP findings are recorded as `Pass: true` with the spec-mandated note "regtest plain transport — production deployment must terminate TLS in front" (per Q1=A).
5. NEW-NFR13 is fully config-driven via `Limits.NFR13MaxProbeRate` and `Limits.NFR13ProbeDuration`; either being 0 yields a clean `StatusSkipped` with reason "rate-limit probe disabled in config".
6. OPS-3 metric names match SP2 discovery exactly:
   - `teranode_blockassembly_best_block_height`
   - `teranode_blockchain_fsm_current_state`
   - `teranode_blockassembly_transactions`
   - `teranode_validator_transactions_count`
   - `teranode_blockvalidation_validate_block_count`
7. Pipeline is green; SP1–SP4 + SP4-DOCKER (static) done-checks pass; SP5 done-check static path exits 0.
8. No live network calls in unit tests — `tests/tests_test.go` exercises only `deriveStatus` and `classifyRateLimit`.
9. Helper functions present and correct: `ok`, `fail`, `required`, `skipMissing`, `errorResult`, `deriveStatus`, `mineBlocks`, `waitForTeranodeTip`, `probeTLS`, `tlsInfo`, `classifyRateLimit` — all package-internal.
10. (See Minor #1.)

`readVarInt` correctly handles all four prefix forms (0x00–0xfc, 0xfd, 0xfe, 0xff) with truncation guards. `parseStandardBlock` correctly skips the 80-byte header, reads the tx-count VarInt, and consumes per-tx bytes via `bt.NewTxFromStream` accumulating the `used` byte counter. `submitAndConfirm` uses a value receiver `*txgen.Funder` (simplified per the plan note from the proposed `**Funder` design). The dummy P2MS pubkeys are 33-byte arrays prefixed with `0x02`; `MultisigScript` builds via `bscript.AppendPushData` which doesn't validate curve points, so construction succeeds.

## Important: none

The `goto LimitObserved` in `tests/new_nfr13.go` jumps from inside the `select` case out of the `for` loop to a label positioned immediately after the loop. This is legal Go (no variable scope is jumped over), control flow is correct, and the label is reached either by `goto` or by deadline-fall-through.

The mainnet-load gate in `config/validate.go` correctly lists `NEW-NFR13` in `loadGeneratingTests`, since the probe issues 5,000 calls in 5 seconds — operator must opt in with `--allow-mainnet-load` against mainnet.

## Minor

1. **Stale `var _` staticcheck guards in `tests/helper.go` lines 154–156.** The plan/spec brief explicitly calls these out as items to remove once `probeTLS`, `tlsInfo`, `classifyRateLimit` (and the matrix/svnode/teranode imports) are consumed by the four landed tests. Currently:

   ```go
   var _ matrix.Severity = matrix.SeverityCritical
   var _ *svnode.RPCClient
   var _ *teranode.RPCClient
   ```

   Note that `helper.go` itself only references `teranode.RPCClient` (in `waitForTeranodeTip`'s signature). It does **not** reference `matrix` or `svnode` outside the `var _` guards. So removing the three guards requires also dropping `internal/matrix` and `internal/svnode` from helper.go's import block (the `teranode` import is still load-bearing for `waitForTeranodeTip`). Effectively: drop the three guards, drop two imports, done. Pure cleanup; doesn't affect behaviour.

2. **Doc nitpick (`tests/helper.go:63`)** — `mineBlocks` doc says "asks svnode-1's wallet" but the function dispatches via `env.SVNode.RPC`, which is whichever SV node the config points at. Cosmetic only.

3. **PC-3 chains 3 unconfirmed mempool tx's** (each tx spends the previous tx's change UTXO before any block is mined). BSV regtest with default policy accepts unbounded mempool chains, so this is fine in the docker stack. If the reference SV node is ever swapped for a build with a mempool-chain limit < 3, shapes 2 and 3 could fail with "missing inputs". Worth a one-line note in the spec's risk table; not a code defect.

## Strengths

- **Faithful to spec.** Every section of design spec §3 / §4 / §5 / §6 has a concrete, runnable deliverable. The four test files mirror each other in shape, making the package immediately legible.
- **Comments at the top of each test file are verbatim source-plan reproductions** (Objective / Method / Acceptance criteria / Implementation notes), which is exactly what the build doc demands and what the HTML reporter displays in `<details>` blocks.
- **Skip semantics are clean.** Each test self-skips when its required clients are nil; `TestRegisterTests_SP5RegistersFour` runs all four against a bare `Env` and confirms each one returns a `Result` (not `NOT_RUN`).
- **`parseStandardBlock` is a tight, standalone parser** — no dependency on the libsv block parser (which was confirmed not exported), self-contained, and would round-trip any standard BSV block.
- **Dummy P2MS pubkeys are deterministic** — last byte differs (`0xa1`/`0xa2`/`0xa3`), so the test produces stable txids run-to-run.
- **`scripts/sp5-done-check.sh` cascades through SP1–SP4 + SP4-DOCKER static done-checks** before its own gate, so a regression upstream surfaces immediately.
- **Config defaults applied centrally** in `applyDefaults`; `Validate` rejects negative values, allows 0 (which is the documented "disable" sentinel).
- **All four YAML configs updated consistently:** `config/testdata/minimal.yaml`, `config.example.yaml`, `config.docker.yaml`, `cmd/teranode-acceptance/testdata/integration.yaml` — example-loader test still passes.

## Spec coverage gaps

None observed.

## Recommendation

**Approve and tag `sp5-complete`.** Optionally land a one-commit cleanup removing the three `var _` guards plus the now-unused `matrix` and `svnode` imports in `tests/helper.go` (Minor #1). It's pure tidying — neither blocks the tag nor changes behaviour.
