# SP10 — Hardening Pass (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP10 / 11 — Final hardening, docs, verdict-pipeline tests
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-05-01
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1–SP9
**Status:** awaiting user review

---

## 1. Purpose

Land the last sub-project — no new tests; tighten the project so an operator can take it and run with confidence. Every item in the original build-doc §13 ("Definition of done") and SP1 spec §13 ("Definition of done") gets explicitly verified. Documentation moves from skeleton to operator-ready.

After SP10, the project is shippable: `make compose-up && SP10_LIVE=1 ./scripts/sp10-done-check.sh` produces a complete report that interpretation guidance covers.

## 2. Scope

### In scope

- `internal/testrunner/pipeline_test.go` — end-to-end verdict-pipeline tests with synthetic Result fixtures (all 4 verdict outcomes: GO / CONDITIONAL_GO / NO_GO / INCOMPLETE).
- `scripts/check-test-docs.go` — doc-comment audit; each `tests/<id>.go` must contain `Objective:`, `Method:`, `Acceptance criteria:` markers in its top-of-file comment block.
- `scripts/check-test-docs_test.go` — unit tests for the audit.
- `Makefile` — extend `verify` to run `check-test-docs`.
- Coverage spot-fixes: any helper package below 70% gets a few targeted tests. (Most are already above per prior reviews; this is mostly a verification step.)
- **README full rewrite** — quickstart, sub-project map, exit-code interpretation, troubleshooting, version note, links to operator guide.
- `docs/operator-guide.md` — step-by-step for first-time operator (prerequisites → setup → run → interpret).
- `docs/verdict-interpretation.md` — table of "if you see X status on test Y, here's what it means and what to do".
- Final `superpowers:code-reviewer` pass over the whole project (cross-cutting issues from SP1–SP9).
- `scripts/sp10-done-check.sh` — meta-check: all 10 prior done-checks exist, executable, and exit 0; build-doc §13 items mechanically verified.
- Captured review report at `docs/superpowers/reviews/2026-05-01-sp10-final-review.md`.

### Out of scope

- New acceptance tests. (Suite stays at 19.)
- Changes to compose stack, docker-compose.yml, or fixture generators.
- Changes to test logic (only doc comments, only if a finding surfaces drift).
- Live `make compose-up` end-to-end run — operator action; SP10 documents expected outcomes but doesn't run it.
- Migration off the v0.15.0-beta-2 image. The version-note in README documents how to upgrade later.

## 3. Architecture

```
SP10 deliverables
    │
    ├── internal/testrunner/pipeline_test.go    end-to-end verdict tests
    │
    ├── scripts/check-test-docs.go              audit tool
    ├── scripts/sp10-done-check.sh              meta-check
    ├── Makefile                                 verify ⊇ check-test-docs
    │
    ├── README.md                                full rewrite (operator-facing)
    ├── docs/operator-guide.md                   step-by-step
    └── docs/verdict-interpretation.md           per-test interpretation table
```

### 3.1 Verdict-pipeline tests

Existing `internal/testrunner/verdict_test.go` exercises `ComputeVerdict` directly with synthetic Result slices. SP10 adds **end-to-end pipeline tests** that exercise:

```
Suite.Run(ctx) → BuildReportModel → ComputeVerdict → exit code
```

i.e. they go through the same code path as the CLI but with synthetic test functions injected at registration time.

```go
// internal/testrunner/pipeline_test.go
package testrunner

func TestPipeline_AllGreenWithOverrides_GO(t *testing.T) {
    env := newPipelineEnv(t)
    suite := NewSuite(env)
    registerAllPassing(suite)  // helper that registers each in-scope test ID returning Status: PASS
    results := suite.Run(context.Background())
    overrides := overrides.File{
        Reviewer: "test", Overrides: map[string]overrides.Override{
            "IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
            "FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
            "NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
            "NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
            "NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
        },
    }
    model, err := BuildReportModel(env, results, overrides, time.Now(), time.Now(), "v")
    if err != nil { t.Fatal(err) }
    if model.Verdict.Decision != "GO" {
        t.Errorf("got %s, want GO", model.Verdict.Decision)
    }
    if model.Verdict.ExitCode != 0 {
        t.Errorf("got exit %d, want 0", model.Verdict.ExitCode)
    }
}

// + parallel tests for CONDITIONAL_GO (Important fail), NO_GO (Critical fail),
//   INCOMPLETE (no overrides), INCOMPLETE (Critical NOT_RUN), etc.
```

The helper `registerAllPassing` programmatically registers each in-scope ID from `matrix.Load().InScopeTestIDs()` with a no-op test function returning a synthetic PASS Result. Variants for different verdict outcomes register the same set with different injected results.

### 3.2 Doc-comment audit (`scripts/check-test-docs.go`)

Each `tests/<id>.go` must have a top-of-file comment block containing the three markers. Tool:

```go
// scripts/check-test-docs.go
//
//   $ go run ./scripts/check-test-docs.go --tests-dir tests/
//
// Asserts every tests/<id>.go file has Objective: + Method: + Acceptance:
// markers in its top-of-file comment block. Skips fixtures.go, helper.go,
// tests_test.go, doc.go, and any *_test.go files.

package main

func main() {
    // ... walk tests/ for *.go files matching test ID patterns
    // ... read each, check for markers
    // ... emit violations to stderr; exit 1 if any found
}
```

The check is heuristic but useful: if a test file's spec drifts from what the source plan says, this catches the missing markers (a common drift).

### 3.3 Coverage spot-fixes

Per most-recent reviews:

| Package | Last measured |
|---|---|
| `config` | 94.6% |
| `internal/matrix` | 90%+ |
| `internal/overrides` | 96.0% |
| `internal/jsonrpc` | 82.9% |
| `internal/teranode` | 80.1% |
| `internal/svnode` | 80.3% |
| `internal/compare` | 100% |
| `internal/txgen` | 84.5% |
| `internal/testrunner` | 80%+ |
| `internal/observer` | (new in SP9, has unit tests) |

All currently above 70%. SP10 re-runs coverage and bumps anything below; if everything is already above, no test additions are needed — record current numbers in `docs/superpowers/reviews/2026-05-01-sp10-final-review.md`.

### 3.4 README rewrite (Q1=A)

Sections:

1. **One-paragraph project summary** (preserved from SP1)
2. **Quickstart** — copy-paste sequence:
   ```bash
   git clone ...
   cd node-validation
   make build
   make compose-up           # ~2 min: pulls images, starts stack, mines 110 blocks
   make compose-test         # ~30 min: runs --short suite
   make compose-down
   ```
3. **Sub-project map** — table of all 11 sub-projects with status (all complete) and what each delivered.
4. **Exit codes** — keep SP1's table; add commentary on what each verdict means in practice.
5. **Test catalogue** — table of all 19 tests with severity, source plan ID, what they measure, where to look in the report.
6. **Reviewer overrides** — how to mark IBD-1 / FR-4 / NFR-1 / NFR-8 / NFR-9 as PASS via documentation review; example YAML; how the runner consumes it.
7. **Troubleshooting** — common failure modes with explanations:
   - "OPS-3 fails because metric X not found" → likely Teranode-version drift; update constant set.
   - "PC-1 reorg phase fails" → check teranode-1 has wallet / can mine.
   - "INTER-2 < 99% propagation" → check legacy mesh peering.
   - "NEW-FR9 no notification on /p2p-ws" → check port 19906 is exposed.
   - etc.
8. **Version note** — Teranode v0.15.0-beta-2 vs SP2 discovery SHA `11f5fa6a8…`; how to refresh discovery if upgrading.
9. **Traceability matrix** (auto-generated by gen-traceability — preserved).
10. **What this project does NOT do** — preserved from SP1.
11. **License placeholder**.

The full README sits ~400-500 lines.

### 3.5 `docs/operator-guide.md`

Step-by-step deeper than README's quickstart. Sections:

1. Prerequisites (Docker ≥24, Compose v2, ~8GB RAM, ~30GB disk, Go 1.22 if rebuilding fixtures)
2. Initial setup (clone, build binaries, verify static checks)
3. First live run (compose-up, observe bootstrap output, verify all 6 nodes converge, run the suite)
4. Reading the report (HTML walkthrough, JSON schema reference, key fields)
5. Reviewer overrides workflow (when to use them, how to populate the YAML, how the verdict shifts)
6. Reporting findings to BSVA (which fields matter, how to attach logs, version evidence)
7. Refreshing the suite when upgrading Teranode (re-run SP2 discovery, regenerate traceability, validate fixtures still match)
8. Common operations (rerunning a single test, comparing two runs, isolating a flaky test)

### 3.6 `docs/verdict-interpretation.md`

Tabular reference: per-test, per-status, what it means and what to do. Example row:

| Test | Status | Means | Action |
|---|---|---|---|
| OPS-3 | PASS | Metrics + health endpoints work, all 5 categories present | None |
| OPS-3 | FAIL ("Metric `teranode_blockassembly_best_block_height` absent") | Metric was renamed in this Teranode version | Update the metric-name set in `tests/ops3.go`; commit; rerun |
| NEW-FR9 | FEATURE_NOT_AVAILABLE | Double-spend WS notification not surfaced; expected per SP2 | None — expected for current Teranode |
| INTER-2 | FAIL ("only 91% propagation") | Mesh peering may be partial | Check `make compose-logs SERVICE=teranode-1` for connection drops |
| ... | ... | ... | ... |

One row per (test, plausible-status) combination. Maybe 80-100 rows total.

### 3.7 Final code review

Single `superpowers:code-reviewer` agent run scoped to the entire project. Reviewer is given:
- The 10 prior review reports under `docs/superpowers/reviews/`
- The build doc + all 11 spec docs
- The full repo

Asked to find:
- Cross-cutting issues that don't fit one sub-project (e.g. naming inconsistencies across the suite)
- Spec gaps that are still open (tracked in any of SP1-SP9 as "deferred to SP10")
- Final spot-checks on every spec's definition-of-done

Findings go to `docs/superpowers/reviews/2026-05-01-sp10-final-review.md`.

### 3.8 Done-check

`scripts/sp10-done-check.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> All prior done-checks present + executable + exit 0"
for sp in 1 2 3 4 4-docker 5 6 7 8 9; do
    script="./scripts/sp${sp}-done-check.sh"
    [ -x "$script" ] || { echo "FAIL: $script missing or not executable"; exit 1; }
done
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh
./scripts/sp7-done-check.sh
./scripts/sp8-done-check.sh
./scripts/sp9-done-check.sh

echo "==> Test doc-comment audit"
go run ./scripts/check-test-docs.go --tests-dir tests/

echo "==> Coverage thresholds"
for pkg in config internal/matrix internal/overrides internal/jsonrpc \
           internal/teranode internal/svnode internal/compare \
           internal/txgen internal/testrunner internal/observer; do
    pct=$(go test -race -cover ./$pkg/... 2>/dev/null | grep -oE 'coverage: [0-9.]+%' | grep -oE '[0-9.]+' | head -1)
    if [ -z "$pct" ]; then continue; fi
    awk -v p="$pct" -v t=70 'BEGIN { if (p+0 < t+0) exit 1 }' \
        || { echo "FAIL: $pkg coverage $pct% < 70%"; exit 1; }
    echo "    $pkg: $pct%"
done

echo "==> Build-doc §13 mechanical checks"
# (a) Manifest matches README + traceability.md (codegen invariant)
make verify
# (b) Project compiles without bsv-blockchain/teranode in go.sum
if grep -q "github.com/bsv-blockchain/teranode " go.sum 2>/dev/null; then
    echo "FAIL: bsv-blockchain/teranode is a dependency"; exit 1
fi
# (c) Every test file has source-plan markers (already covered by check-test-docs)

echo "==> Documentation present"
test -s README.md
test -s docs/operator-guide.md
test -s docs/verdict-interpretation.md

if [ "${SP10_LIVE:-0}" = "1" ]; then
    echo "==> Live: full --short run"
    make compose-up
    ./bin/teranode-acceptance --short --test-timeout 90m \
        --config config.docker.yaml \
        --reviewer-overrides docs/operator-guide-overrides-example.yaml || true
    test -s report.json
    test -s report.html
    decision=$(jq -r '.verdict.decision' report.json)
    echo "==> Live verdict: $decision"
    make compose-down
fi
echo "==> SP10 done-check passed."
```

The example reviewer-overrides file lives at `docs/operator-guide-overrides-example.yaml` (referenced by the operator guide).

## 4. Verification & testing strategy

### 4.1 Unit tests

- `internal/testrunner/pipeline_test.go` — 4+ verdict-pipeline scenarios
- `scripts/check-test-docs_test.go` — table-driven cases (file with all markers, file missing one, etc.)

### 4.2 SP10 done-check

`./scripts/sp10-done-check.sh` exits 0. The static path runs without Docker (just like every prior done-check). The live path requires Docker.

## 5. Definition of done

- All 4 SP10 deliverable files exist (pipeline_test.go, check-test-docs.go, sp10-done-check.sh, README rewrite, operator-guide.md, verdict-interpretation.md).
- `make build lint test verify` exits 0; SP1–SP9 done-checks pass; SP10 done-check exits 0.
- All 10 prior `sp{N}-done-check.sh` scripts exist and pass.
- Coverage all packages ≥70%.
- Final code-review agent's report has no Critical findings.
- README rewrite covers all 11 sections from §3.4.
- `docs/operator-guide.md` covers the 8 sections from §3.5.
- `docs/verdict-interpretation.md` has rows for every (test, plausible-status) pair.
- Captured review report committed.

## 6. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | Final code review surfaces a Critical issue that requires SP9-level work to fix | If found, SP10 stays open until resolved; not blocking SP1-SP9 since those are independently tagged |
| B | Coverage spot-checks reveal a package below 70% (most likely candidate: `internal/teranode/notifications` due to centrifuge live-only paths) | Add 1-2 unit tests for the package; if the live-only paths are uncoverable, document the constraint and accept |
| C | Doc-comment audit catches drift in test files modified during SP5-SP9 reviews | Easy fix — re-add the missing markers; one commit |
| D | README rewrite churn is large; existing skeleton becomes obsolete | Diff-friendly; existing skeleton stays in git history |
| E | The example reviewer-overrides YAML in docs/operator-guide-overrides-example.yaml could be used unmodified in production by a careless operator | Mark prominently as `# EXAMPLE — DO NOT USE FOR REAL OPERATIONS` at the top |

## 7. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | README full rewrite + new operator-guide.md + verdict-interpretation.md | per user (Q1=A) |
| 2 | Pipeline tests live in `internal/testrunner/pipeline_test.go` (not cmd/teranode-acceptance) | drafter — pipeline-level test, not CLI-binding |
| 3 | Doc-comment audit via `scripts/check-test-docs.go`; wired into `make verify` | drafter |
| 4 | Coverage threshold 70% across all helper packages | drafter — matches build-doc §10 |
| 5 | Final code-review agent gets all 10 prior reviews as input | drafter |
| 6 | SP10_LIVE=1 path is operator-only, optional in done-check | drafter |
| 7 | `docs/operator-guide-overrides-example.yaml` ships with explicit "DO NOT USE FOR REAL" warning | drafter |
| 8 | Existing test logic in `tests/` is NOT modified during SP10 | drafter — only docs and verification |

## 8. Out-of-scope reminders

SP10 doesn't ship: new acceptance tests, new compose services, new fixtures, new clients, new generators. It only sharpens what's already there and writes the operator-facing docs that turn the project from "engineer's repo" to "shippable suite".

After SP10, the project is **DONE**: 11 sub-projects all tagged, the suite is documented, the operator can `make compose-up` and produce a verdict.
