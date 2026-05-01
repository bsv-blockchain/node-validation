# SP10 Final Cross-Cutting Review

**Date:** 2026-05-01
**Reviewer:** Claude Opus 4.7 (1M context), code-reviewer profile
**Scope:** Whole project at `/Users/oskarsson/gitcheckout/node-validation/`, with all 11 specs, plans, and 10 prior reviews as context.
**Outcome:** APPROVE_WITH_MINOR

---

## Executive summary

The project delivers what the build doc and the eleven specs promised. Verify is green, all helper packages are above the 70% coverage threshold, the manifest's 58 rows match what the README and traceability claim, and nothing in `go.sum` references `github.com/bsv-blockchain/teranode`. The pipeline-test additions in SP10 cover the four verdict outcomes plus useful corner cases (Advisory FAIL doesn't demote, override FAIL → NO_GO). Honest reporting of `FEATURE_NOT_AVAILABLE` is correctly localised in NEW-FR8, and NEW-FR11 records absences as positive findings.

What's not great is documentation hygiene: the operator-facing docs that SP10 explicitly delivered to "turn the project from engineer's repo to shippable suite" reference config keys that don't exist, miss the `--reviewer-overrides` step in the canonical `make compose-test` flow, and never tell the operator that PC-2 / IBD-2 are synthetic. None of these is enough to block the `sp10-complete` tag — they're all fixable in one polish commit — but they would absolutely surface during the first real operator handoff.

---

## Critical issues

**None.** I looked hard for one and didn't find anything that blocks tagging.

---

## Important issues

### 1. `make compose-test` cannot reach `GO`; quickstart misleads about this

The quickstart in both README and operator-guide §3 walks the operator through `make compose-up && make compose-test`. The Makefile target hardcodes `./bin/teranode-acceptance --short --config config.docker.yaml || true` with **no `--reviewer-overrides` flag**, so even a perfect run tops out at `INCOMPLETE` (exit 3) because the five doc-review rows are unsatisfied. Nothing in the quickstart explains that the first run is *expected* to be INCOMPLETE and that the operator must rerun the binary directly with `--reviewer-overrides` to land at GO.

This is exactly the kind of "first-run confusion" SP10 was supposed to eliminate. Two fixes either of which is fine:
- Make `compose-test` honour an `OVERRIDES=...` env var: `./bin/teranode-acceptance --short --config config.docker.yaml $${OVERRIDES:+--reviewer-overrides $$OVERRIDES} || true`
- Add a quickstart note: "the first run will print verdict INCOMPLETE; see §5 for how to supply reviewer overrides and reach GO."

### 2. Documentation refers to config keys that don't exist

I cross-checked every config key referenced in the operator-facing docs against `config/config.go` and `config.example.yaml`:

| Doc reference | Actually | Where |
|---|---|---|
| `Limits.PERF1MaxTPS` | `limits.perf1_max_tps` | `README.md:113` (also a stray `PERF1MaxTPS` comment in `config.example.yaml:52`) |
| `centrifuge_url` | `notification_url` (under `teranode:`) | `docs/verdict-interpretation.md:95` |
| `rate_limit_floor` | doesn't exist anywhere in the codebase | `docs/verdict-interpretation.md:246` |
| `test_timeout` (config) | only exists as CLI flag `--test-timeout` | `docs/verdict-interpretation.md:18, 67` |
| `perf1_max_tps` | correct (snake_case YAML) | `docs/verdict-interpretation.md:120, 121` — this one is right |

Three of these (`centrifuge_url`, `rate_limit_floor`, `Limits.PERF1MaxTPS`) will cause the operator to grep the config file fruitlessly. Either fix the doc or add the keys; the fastest fix is the doc.

### 3. Go-version mismatch between `go.mod` and operator guide

`go.mod` declares `go 1.23`. `docs/operator-guide.md:9` says "Go 1.22 (only if rebuilding fixtures)". An operator on 1.22 will get a compile error trying to rebuild. Bump the doc to 1.23.

### 4. Synthetic-fixture provenance is buried

The README's "What this project does NOT do" section says it. The operator guide and verdict-interpretation never mention it. PC-2 and IBD-2 use synthetic regtest fixtures, not real testnet history; that's a deliberate SP8 design decision, but an operator interpreting a PC-2 PASS as "30 historical mainnet transactions agreed across nodes" is being misled. Add one sentence to `docs/operator-guide.md` §4 ("Reading the report") and ideally to the per-test rows in `verdict-interpretation.md` for PC-2 and IBD-2.

### 5. Version-drift risk only documented in README

The "v0.15.0-beta-2 image may have been built from a different commit than the SP2 SHA" caveat sits in README's §"Version note". The operator guide §6 ("Reporting findings to BSVA") tells the operator to capture the image tag and the SP2 commit but doesn't explain *why* — that the commit may not match the image. An operator hitting OPS-3 metric drift won't know to read the README's troubleshooting unless told to. Cross-link from operator-guide §6 and §7 into the README version note.

### 6. The "live" path of `sp10-done-check.sh` self-references the EXAMPLE override file

`scripts/sp10-done-check.sh:55-57` runs the live path with `--reviewer-overrides docs/operator-guide-overrides-example.yaml`. The example file's first lines are `# EXAMPLE — DO NOT USE FOR REAL OPERATIONS`. For the harness to self-test it's fine, but a production operator who copies the done-check verbatim will be using the example artefacts as if they were real. Either rename the file (e.g. `…-template.yaml`) so the warning is unambiguous, or have the live path use a separate `done-check-overrides.yaml` that's not pitched as the operator template.

---

## Minor / nits

### 7. Test-file naming inconsistency

`tests/pc1.go` vs `tests/new_fr7.go`. `client1.go`, `inter1.go` follow the no-separator convention; the `NEW-` family uses underscores. Internal-only, no functional impact, but a cosmetic asymmetry that hurts grep-ability (`grep -l "RunPC" tests/` works; `grep -l "RunNEWFR" tests/` requires you to know the naming).

### 8. Mixed error-creation style across `internal/`

`internal/txgen/types.go` defines sentinels (`ErrInsufficientFunds`, `ErrNoWallet`); other packages use ad-hoc `errors.New` and `fmt.Errorf`. Acceptable for a test harness, but if any future caller wants to do `errors.Is` on a teranode-client error, they can't. Not worth refactoring at this point.

### 9. `make build lint test` documented; `verify` not

Operator guide §2 says "make build lint test    # static checks must pass" but the project's actual definition-of-done is `make build lint test verify` (build doc §13). Add `verify` to the operator-guide example. The done-check script gets it right; only the operator guide is incomplete.

### 10. `make compose-test` depends on `compose-up`

The README quickstart shows
```
make compose-up
make compose-test
make compose-down
```
which runs `compose-up` twice (since `compose-test` declares it as a dep). `compose-up` is roughly idempotent (docker compose is), but the second invocation will still re-mine 110 blocks, which is wasted work and may fail if the wallet is already past block 110. Drop `compose-up` from the quickstart, or split `compose-test` into a `compose-test-only` variant without the dep.

### 11. SP1 deferred "HTML field-level golden tests" never landed

SP1 spec §"Out of scope" said field-level HTML rendering golden tests would be SP10. SP10 added end-to-end pipeline tests for the verdict but did not add a golden-HTML test. The `reporter_html_test.go` does structural assertions only. Low priority — the HTML template is small and rarely changes — but the deferral was named explicitly.

### 12. SP3 noted "removed by SP10 from `go.mod`" — that referred to a never-added dep

`docs/superpowers/specs/2026-04-29-sp3-backend-clients-design.md:431` mentions removal "by SP10 from go.mod"; in fact no such dep was ever added, so SP10 doesn't need to act. Keep this in mind only as a documentation cross-reference.

### 13. SP4-DOCKER deferred "No CI integration in SP4-DOCKER" to SP10

SP10 did not add CI. The done-check scripts are the closest thing. Acceptable — CI was never explicitly delivered as an SP10 promise — but worth noting that the deferred-to-SP10 item is not addressed by SP10. If the project ever lands in a CI environment, someone will have to wire the done-checks.

### 14. SP4-txgen deferred "lint check could enforce — defer to SP10" for fixture-WIF leak prevention

The risk was that the test fixture WIF in `internal/txgen/testdata/fixtures.go` could be imported into `cmd/`. SP10 did not add such a lint check. The package boundary (`testdata/`) prevents this in practice, so the risk is theoretical, but the deferral is unaddressed.

### 15. SP5 deferred "PC-3 mining wait may be config-driven" and "TLS sidecar"

SP10 spec §2 lists "Changes to test logic (only doc comments…)" as out of scope, so these were correctly deferred again. They remain as latent debt.

### 16. SP6 deferred "NEW-FR9 low-confirmation criterion" to SP9

SP9 didn't add it either. The test still records it as a deferred-with-note path. Honest, but the deferral has been kicked twice now.

### 17. `report.html` is shown to operators but never validated against a goldenfile

Same root cause as nit 11. If the template breaks in a way that still parses as HTML, no test fails.

---

## Strengths

1. **Verdict pipeline tests are correctly scoped.** `pipeline_test.go` exercises Suite.Run → BuildReportModel → ComputeVerdict end-to-end. The four core cases (GO, NO_GO, CONDITIONAL_GO, INCOMPLETE) are covered, plus three real-world nuances (Advisory FAIL doesn't demote, Important SKIPPED is acceptable, override FAIL produces NO_GO).
2. **Coverage is genuinely high.** Every helper package measured: `compare` 100%, `overrides` 96%, `config` 94.1%, `testrunner` 92.7%, `observer` 92.2%, `matrix` 90.9%, `txgen` 84.1%, `jsonrpc` 82.9%, `svnode` 80.9%, `teranode` 79.7%. Most are well above 70%; nothing required spot-fixes.
3. **Build doc §13 mechanically checked.** `sp10-done-check.sh` enforces `make build lint test verify`, all 10 prior done-checks executable + green, coverage thresholds, and the no-`teranode` go.sum invariant.
4. **`FEATURE_NOT_AVAILABLE` is honest.** NEW-FR8 returns the dedicated status. NEW-FR11 records expected-absent endpoints as positive findings rather than silent skips. This matches the SP2 discovery's intent.
5. **Verdict-interpretation reference is comprehensive.** ~315 lines, one section per test, multiple plausible status outcomes per test with concrete actions. Better than the spec asked for.
6. **Manifest invariants enforced.** `golden_test.go` freezes the 58-row manifest; `make verify` enforces traceability/README sync and fixture-YAML sync.
7. **Doc-comment audit is wired into `make verify`.** Every test file has `Objective:` / `Method:` / `Acceptance criteria:` markers, and the audit catches drift.
8. **Reviewer-override semantics are well-tested.** `TestPipeline_OverrideFail_NO_GO` proves an explicit FAIL override fails the doc-review row; this is exactly the behaviour an auditor wants.

---

## Spec coverage gaps

(Items that specs deferred to "SP10" or "later" and were not delivered. None blocking; all listed for transparency.)

| Source | Deferred item | SP10 state |
|---|---|---|
| SP1 §"Out of scope" | HTML field-level golden tests | NOT delivered |
| SP1 funding-WIF regex check | Full regex check deferred to SP4 | Believed delivered in SP4 |
| SP3 §"…removed by SP10 from go.mod" | Removal of a dep | N/A — dep never added |
| SP4-DOCKER nit 12 | CI integration | NOT delivered (out of explicit SP10 scope) |
| SP4-txgen nit F | Lint check preventing fixture-WIF import into cmd/ | NOT delivered |
| SP5 risk A | PC-3 mining wait → config | NOT delivered |
| SP5 risk D | TLS sidecar for NEW-NFR11 | NOT delivered |
| SP6 risk D | NEW-FR9 low-confirmation criterion | NOT delivered (deferred to SP9, also not delivered) |
| SP8 risk B | Testnet-fixture sourcing path | NOT delivered (explicitly out of SP10 scope) |
| SP8 OOS reminders | ≥50 PC-2 fixtures | NOT delivered (explicitly out of SP10 scope) |

The *ones the operator will actually feel* are: TLS sidecar absence (NEW-NFR11 always reports "regtest plain HTTP" — which is documented as a known finding) and the synthetic-fixture provenance for PC-2/IBD-2. Both are documented somewhere; neither is blocking.

---

## Operator readiness assessment

A first-time operator can follow `make compose-up && make compose-test && make compose-down` and produce `report.html` and `report.json`. They will see verdict `INCOMPLETE`, and unless they read the operator guide §5 carefully, they will not know that this is expected on a fresh run and that they need to re-invoke the binary with `--reviewer-overrides` to reach GO. Three of the troubleshooting tips (Limits.PERF1MaxTPS, centrifuge_url, rate_limit_floor) point at config keys that don't exist; an operator who hits the corresponding failure mode and tries to act on the doc will lose 10-30 minutes confused. The version-drift caveat is real and is documented in README; the operator guide doesn't pull it forward, so an operator using only the operator guide will miss it. None of these are showstoppers — the operator can succeed by reading both docs and the troubleshooting section — but the SP10 promise of "operator-ready" is one polish commit away from being kept honestly.

---

## Overall recommendation

**APPROVE_WITH_MINOR.** Tag `sp10-complete` is appropriate. None of the issues block the tag. A follow-up "operator-doc polish" commit fixing items 1, 2, 3, and 4 above (~30 minutes of work) would close the most-likely first-operator confusion paths. Items 5-17 can be addressed lazily as they surface or in a future SP11-style polish pass.
