# SP1 Code Review — Reportable Skeleton

**Date:** 2026-04-29
**Reviewer:** Senior Code Reviewer agent (Claude Opus 4.7 1M context)
**Scope:** entire `node-validation` repo at HEAD of branch `main`, 24 commits
**Spec:** `docs/superpowers/specs/2026-04-28-sp1-reportable-skeleton-design.md`
**Plan:** `docs/superpowers/plans/2026-04-29-sp1-reportable-skeleton.md`

---

## Verification run (machine truth)

| Check | Result |
|---|---|
| `make build` | clean |
| `make lint` (vet + gofmt + staticcheck) | clean |
| `make test -race` | all packages pass |
| `make verify` (codegen idempotency gate) | clean |
| `./scripts/sp1-done-check.sh` | exit 0 |
| `./bin/teranode-acceptance --short --config config.yaml` | exit 3, INCOMPLETE |
| `report.json` row counts | 24 / 3 / 24 / 7 (correct) |
| HTML self-contained | no `<script>`, no external CSS, no fonts |
| `go.mod` Teranode dep | none |
| Grep for TODO / FIXME / placeholder in code | none |

All ten invariants in the review brief hold; the harness genuinely does what SP1 promises.

---

## Critical issues

**None.** No invariant is violated, `make verify` is green, and the done-check script
returns 0. Spec scope is met.

---

## Important issues

1. **`internal/testrunner/report.go:334-341` — dead `SortedKeys` helper.**
   It is exported, has 0% coverage, and is unused. Spec §3 cross-cutting choices required
   "Observations map[string]any is marshalled with sorted keys via a custom MarshalJSON" —
   the implementation relies on the standard library's automatic key sorting for
   `map[string]X`, which is correct, so the helper is leftover plumbing. Either
   wire it into a custom `MarshalJSON` (matches spec wording) or delete it. Leaving
   it in place tempts future maintainers to call it from new sites and exposes API
   surface that is not exercised.

2. **`internal/testrunner/verdict.go:71-93` — verdict logic ignores override `Decision: FAIL`.**
   The check is `if !ok || o.Decision != overrides.DecisionPass`. An override that
   explicitly records `decision: FAIL` for IBD-1 / FR-4 silently demotes to
   `INCOMPLETE` with the rationale "documentation review required but not supplied
   via overrides," which is misleading — the reviewer *did* supply it; they
   asserted FAIL. Spec §6 verdict tree treats a Critical FAIL as `NO_GO`. A FAIL
   override against a Critical doc-review row should produce `NO_GO`, not
   `INCOMPLETE`. Worth a one-line fix and a test.

3. **Coverage targets slightly under spec §10:**
   - `config` 78.4% vs target ≥80% (close).
   - `internal/overrides` 72.0% vs target ≥80% (file-not-found and YAML-parse-error
     branches in `Load` are uncovered).
   - `verdict.go` `ComputeVerdict` 95.7% vs target "100% of branches" — the FAIL-override
     branch above is one of the missing arms.

4. **`config/env.go:91` — fallback to `os.LookupEnv` when caller passed an `environ` slice.**
   `lookupEnv(environ, key)` first scans `environ` and, if not found, calls
   `os.LookupEnv`. That means a unit test passing `environ=[]string{}` can leak
   real process environment (e.g. a developer's exported `TNG_*`). The spec wants
   precedence to be deterministic; the fallback is a hidden source of
   non-determinism. Recommend: drop the `os.LookupEnv` fallback entirely and let
   `main` pass `os.Environ()` explicitly (it already does).

5. **`config/validate.go` — WIF regex check missing.**
   Spec §5 validation table requires `Funding.WIF matches base58/WIF regex if non-empty`.
   Not implemented. Low blast radius for SP1 (funding is unused until SP4) but the
   spec lists it as in scope. Either add the regex or note explicitly in the SP1
   decisions log that this validation is deferred to SP4.

6. **Suite does not enforce ctx cancellation; relies on the test fn to observe it.**
   `suite.go:runOne` wraps the test in `context.WithTimeout` and recovers from
   panics, but it does not race the test fn against `tctx.Done()`. A misbehaving
   future test that ignores ctx will hang forever despite the per-test timeout. SP5+
   tests will trip on this. Consider running the test fn in a goroutine and selecting
   on `tctx.Done()` with a `StatusError, SkipReason: "interrupted"` synthesis. Spec
   §6 explicitly promised "SIGINT cancels the context; the in-flight test produces
   StatusError with SkipReason interrupted" — the runner currently does not
   produce that on its own.

7. **`internal/testrunner/reporter_html.go` — HTML uses tables, not the `<details>` blocks
   the spec called for.** Spec §6 reporter notes "every `<details>` has summary +
   body" as a structural test. The current template chose tables. The structural test
   in `reporter_html_test.go` accommodates this by checking for `<h2>` headers and IDs
   only, which is fine. This is a deliberate deviation that should be recorded in
   the design doc rather than left implicit.

---

## Minor issues / nits

- `internal/testrunner/suite_test.go:130` — `var _ = errors.New` is an unused-import
  workaround. Remove the import.
- `internal/testrunner/report.go:131-132` — `default` arm in the kind switch returns
  an "unknown kind" error, but the manifest validator already enforces the closed
  set. Defensive but unreachable; harmless.
- `internal/testrunner/report.go:117-121` — copying `ovr` into a local then taking its
  address is fine but reads awkwardly: `o := ovr; model.Run.ReviewerOverrides = &o`.
  A direct `&ovr` would alias the parameter, so the copy is intentional — a comment
  would document intent.
- `cmd/teranode-acceptance/main.go:50` — main passes `time.Now` directly into
  `NewEnv`. That is correct for production but the spec called out the fake-clock
  seam as a first-class concern; an `Env` constructor variant
  `testrunner.NewEnvWithClock` for tests would make the seam more obvious. Currently
  callers must remember to pass a stub `func() time.Time`.
- `internal/testrunner/report.go:140-155` — five sequential `if got != X { return ... }`
  blocks duplicate the wording. Extract a small `expect(name, got, want)` helper.
- `cmd/gen-traceability/main.go:69` — `escapeMD(e.PartialNote+" "+e.Notes)` creates a
  leading or trailing space when one of the two is empty. Functionally fine
  (markdown collapses whitespace) but the rendered cells gain a stray space.
- `Makefile:13` — the gofmt check uses `diff -u <(echo -n) <(gofmt -l .)`. Works in
  bash but would silently misreport on a `sh`-only environment. The `SHELL := /bin/bash`
  line is what saves it; OK as-is, with the dependency made explicit.
- `internal/testrunner/reporter_json.go` — comment claims "deterministic key order
  (the struct field order)" but Go's `encoding/json` writes struct fields in
  declaration order *and* maps in sorted key order. The comment under-sells the
  guarantee for `Observations`.
- `internal/testrunner/reporter_text.go:38-39` — `7` is hard-coded as the risk
  count. Use `len(m.Risks)` to keep one source of truth (the manifest, via
  completeness invariant).

---

## Strengths

- **Manifest is the single source of truth.** `internal/matrix/manifest.go` declares
  all 58 entries in one ordered literal, validated by `validate.go`, frozen by
  `golden_test.go`, and re-emitted by `cmd/gen-traceability`. The codegen + `make
  verify` gate makes manifest / README drift impossible — exactly what spec §13
  decision #5 asked for.
- **Severity table is correct.** The 8 Critical, 3 Important, 8 Advisory mapping
  matches the spec letter for letter; IBD-1 carries the unique
  `(Critical, EXCLUDED_DOCUMENTATION)` combination as required.
- **Verdict logic is small, ordered, and table-tested** (`verdict_test.go` covers
  zero-results, all-green-no-overrides, all-green-with-overrides, critical-fail,
  critical-not-run, important-fail, important-skipped, advisory-fail). The decision
  tree is read top-to-bottom in source order — easy to audit.
- **Completeness invariant is enforced and tested both directions** — `BuildReportModel`
  fails on a sabotaged manifest; the happy path is exercised by every other test
  that constructs a model.
- **Configuration precedence is layered cleanly** — defaults → YAML → env → flags →
  short — and validation aggregates errors before returning, matching spec §5.
  The `--only` / `--skip` typo guards are a nice touch (`PC1` vs `PC-1` is exactly
  the kind of footgun the integration test guards).
- **HTML report is genuinely self-contained.** Embedded CSS, no JS, no external
  assets. A reviewer can email it.
- **`scripts/sp1-done-check.sh` is mechanical and brutal.** It builds, lints, tests,
  asserts row counts, asserts the verdict string, and refuses to lie about exit code.
  Same bar SP1 had to clear and now keeps clearing for the lifetime of the project.
- **No global state, no init functions, no ambient registries.** `registerTests`
  in `cmd/teranode-acceptance/register.go` is the explicit registration site. Empty
  in SP1, easily diff-able.

---

## Spec coverage cross-check

| Spec section | Implementation evidence | Status |
|---|---|---|
| §3 architecture | `cmd/teranode-acceptance/main.go` flow matches the diagram exactly | covered |
| §4 matrix | `types.go`, `manifest.go`, `validate.go`, `lookup.go`, `golden_test.go`, `testdata/golden.yaml` | covered |
| §5 config | `config.go`, `flags.go`, `env.go`, `short.go`, `defaults.go`, `validate.go`, `config.example.yaml` | covered (WIF regex missing — see Important #5) |
| §6 testrunner | `types.go` (Status, Result, Env), `suite.go` (Suite), `verdict.go`, `report.go` (BuildReportModel + 3 reporters via `reporter_*.go`) | covered |
| §7 CLI | `main.go` flag set, run flow, `version` ldflag, `register.go` empty body | covered |
| §8 codegen | `cmd/gen-traceability/main.go` with marker block; `make verify` enforces drift | covered |
| §9 Makefile | `build`, `lint`, `test`, `test-short`, `cover`, `gen`, `verify`, `clean` | covered |
| §10 verification | unit tests across all packages, integration tests in `cmd/teranode-acceptance/main_test.go`, lint, `scripts/sp1-done-check.sh` | covered |
| §11 definition of done | done-check script enforces every bullet | covered |

No section is unimplemented.

---

## Practical sanity checks

- `make build lint test verify` exits 0. Confirmed.
- `./scripts/sp1-done-check.sh` exits 0. Confirmed.
- `./bin/teranode-acceptance --short --config config.yaml` produces `report.json`
  with `requirements:24, test_environment:3, test_cases:24, risks:7`, verdict
  `INCOMPLETE`, exit 3. Confirmed.
- No `TODO` / `FIXME` / `placeholder` strings in code. Confirmed.

---

## Overall recommendation

**APPROVE_WITH_MINOR.**

SP1 meets every Critical invariant in the review brief and the design spec. The
implementation is small, readable, and well-aligned with the plan: 58-row manifest
with golden lock, deterministic verdict logic, completeness invariant, codegen drift
gate, self-contained HTML, no Teranode dependency, fake-clock seam, exit code 4
honoured for config errors. The done-check script and integration tests give SP5+
authors a foundation they can trust.

The Important issues are real but small — none of them block SP1 closeout:

- FAIL-decision override branch (Important #2) — fix before the first reviewer ever
  hands in a real overrides file.
- `os.LookupEnv` fallback (Important #4) — fix before flake hunters notice.
- Coverage gaps (Important #3) — bring `config` and `overrides` over 80% in SP2 or
  SP3 housekeeping.
- The dead `SortedKeys` helper (Important #1) — delete or wire up; either is a
  one-line change.

These are all well within the size of "first commit on top of SP1" cleanup. SP2 can
proceed.
