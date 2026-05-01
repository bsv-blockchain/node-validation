# SP10 — Hardening Pass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Final hardening pass — no new acceptance tests; deliver operator-ready documentation, end-to-end verdict-pipeline tests, doc-comment audit, coverage spot-fixes, and a final code-review report. After SP10, project is shippable.

**Tech Stack:** Existing.

---

### Task 1: Verdict-pipeline tests

**Files:**
- Create: `internal/testrunner/pipeline_test.go`

- [ ] **Step 1: Implement**

```go
// internal/testrunner/pipeline_test.go
//
// End-to-end verdict-pipeline tests: Suite.Run → BuildReportModel →
// ComputeVerdict → exit code. Complement to verdict_test.go (which
// covers ComputeVerdict directly).

package testrunner

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func newPipelineEnv(t *testing.T) *Env {
	t.Helper()
	cfg := config.Config{
		Network:     config.NetworkTestnet,
		TestTimeout: 5 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return NewEnv(cfg, logger, matrix.Load(), nil)
}

// allOverridesPassing returns an overrides.File marking the 5 doc-review
// rows as PASS so a fully-green automated run can land at GO.
func allOverridesPassing() overrides.File {
	return overrides.File{
		Reviewer:   "test-reviewer",
		ReviewedAt: time.Now(),
		Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
}

// registerAllPassing registers every in-scope test ID with a no-op
// function returning a synthetic PASS Result. Optional override `id →
// status` lets a caller plant a specific outcome on chosen tests.
func registerAllPassing(suite *Suite, status map[string]Status) {
	m := suite.env.Manifest
	for _, id := range m.InScopeTestIDs() {
		id := id
		s := StatusPass
		if status != nil {
			if explicit, ok := status[id]; ok {
				s = explicit
			}
		}
		suite.Register(id, func(_ context.Context, _ *Env) Result {
			return Result{ID: id, Status: s}
		})
	}
}

func runPipeline(t *testing.T, status map[string]Status, ovr overrides.File) ReportModel {
	t.Helper()
	env := newPipelineEnv(t)
	suite := NewSuite(env)
	registerAllPassing(suite, status)
	results := suite.Run(context.Background())
	model, err := BuildReportModel(env, results, ovr, time.Now(), time.Now(), "test")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	return model
}

func TestPipeline_AllGreenWithOverrides_GO(t *testing.T) {
	model := runPipeline(t, nil, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s, want GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 0 {
		t.Errorf("got exit %d, want 0", model.Verdict.ExitCode)
	}
}

func TestPipeline_AllGreenNoOverrides_INCOMPLETE(t *testing.T) {
	model := runPipeline(t, nil, overrides.File{})
	if model.Verdict.Decision != "INCOMPLETE" {
		t.Errorf("got %s, want INCOMPLETE (Critical doc-review rows unsatisfied)", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 3 {
		t.Errorf("got exit %d, want 3", model.Verdict.ExitCode)
	}
}

func TestPipeline_OneCriticalFail_NO_GO(t *testing.T) {
	model := runPipeline(t, map[string]Status{"PC-1": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "NO_GO" {
		t.Errorf("got %s, want NO_GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 1 {
		t.Errorf("got exit %d, want 1", model.Verdict.ExitCode)
	}
}

func TestPipeline_OneImportantFail_CONDITIONAL(t *testing.T) {
	model := runPipeline(t, map[string]Status{"OPS-3": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "CONDITIONAL_GO" {
		t.Errorf("got %s, want CONDITIONAL_GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 2 {
		t.Errorf("got exit %d, want 2", model.Verdict.ExitCode)
	}
}

func TestPipeline_ImportantSkipped_StillGO(t *testing.T) {
	// Important SKIPPED is acceptable per source plan ("Must Pass or Have Mitigation Plan").
	model := runPipeline(t, map[string]Status{"CLIENT-2": StatusSkipped}, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s (rationale=%q), want GO — SKIPPED Important is acceptable",
			model.Verdict.Decision, model.Verdict.Rationale)
	}
}

func TestPipeline_AdvisoryFail_NoVerdictDemotion(t *testing.T) {
	model := runPipeline(t, map[string]Status{"NEW-FR7": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s, want GO — Advisory FAIL must not demote", model.Verdict.Decision)
	}
}

func TestPipeline_OverrideFail_NO_GO(t *testing.T) {
	ovr := allOverridesPassing()
	rejected := ovr.Overrides["IBD-1"]
	rejected.Decision = overrides.DecisionFail
	ovr.Overrides["IBD-1"] = rejected
	model := runPipeline(t, nil, ovr)
	if model.Verdict.Decision != "NO_GO" {
		t.Errorf("got %s, want NO_GO — explicit override FAIL should fail Critical doc-review row", model.Verdict.Decision)
	}
}

func TestPipeline_FullModel_HasAllRows(t *testing.T) {
	model := runPipeline(t, nil, allOverridesPassing())
	// Sanity: report shape.
	if got := len(model.Requirements); got != 24 {
		t.Errorf("requirements: %d, want 24", got)
	}
	if got := len(model.TestEnvironment); got != 3 {
		t.Errorf("test_environment: %d, want 3", got)
	}
	if got := len(model.TestCases); got != 24 {
		t.Errorf("test_cases: %d, want 24", got)
	}
	if got := len(model.Risks); got != 7 {
		t.Errorf("risks: %d, want 7", got)
	}
}
```

- [ ] **Step 2: Run, expect pass**

```bash
go test -race ./internal/testrunner/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/testrunner/pipeline_test.go
git commit -m "test(testrunner): add end-to-end verdict-pipeline tests"
```

---

### Task 2: Doc-comment audit + Makefile wiring

**Files:**
- Create: `scripts/check-test-docs.go`
- Modify: `Makefile` (extend `verify`)

- [ ] **Step 1: Implement audit tool**

```go
// Command check-test-docs asserts every tests/<id>.go file's top-of-file
// comment block contains "Objective:", "Method:", and "Acceptance criteria:"
// markers. Run via:
//
//   go run ./scripts/check-test-docs.go --tests-dir tests/
//
// Skips fixtures.go, helper.go, doc.go, and any *_test.go files. Reports
// violations to stderr; exits 1 if any are found.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var skipFiles = map[string]bool{
	"fixtures.go": true, "helper.go": true, "doc.go": true,
	"tests_test.go": true, "fixtures_test.go": true,
}

var requiredMarkers = []string{"Objective:", "Method:", "Acceptance criteria:"}

func main() {
	dir := flag.String("tests-dir", "tests", "directory containing tests/*.go files")
	flag.Parse()

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir: %v\n", err)
		os.Exit(2)
	}
	var violations []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if skipFiles[name] {
			continue
		}
		path := filepath.Join(*dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: read: %v", path, err))
			continue
		}
		// Heuristic: extract the leading comment block (lines starting with "//").
		lines := strings.Split(string(body), "\n")
		var commentBlock strings.Builder
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "//") {
				commentBlock.WriteString(strings.TrimPrefix(trimmed, "//"))
				commentBlock.WriteString("\n")
				continue
			}
			if trimmed == "" {
				continue
			}
			break
		}
		blob := commentBlock.String()
		for _, m := range requiredMarkers {
			if !strings.Contains(blob, m) {
				violations = append(violations, fmt.Sprintf("%s: missing %q in top comment block", path, m))
			}
		}
	}
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Doc-comment audit failed:")
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, "  - "+v)
		}
		os.Exit(1)
	}
	fmt.Printf("check-test-docs: OK (audited %s)\n", *dir)
}
```

- [ ] **Step 2: Add unit tests**

```go
// scripts/check-test-docs_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAudit_validFile(t *testing.T) {
	dir := t.TempDir()
	src := `// Package x
//
// Objective: do a thing
// Method: try it
// Acceptance criteria: it works
package x
`
	if err := os.WriteFile(filepath.Join(dir, "valid.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := runAudit(t, dir)
	if len(violations) != 0 {
		t.Errorf("unexpected: %v", violations)
	}
}

func TestAudit_missingMarker(t *testing.T) {
	dir := t.TempDir()
	src := `// Package x
//
// Objective: a thing
// Method: try it
package x
`
	if err := os.WriteFile(filepath.Join(dir, "incomplete.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := runAudit(t, dir)
	if len(violations) != 1 {
		t.Errorf("got %d violations, want 1: %v", len(violations), violations)
	}
}

// runAudit factors out the audit logic for tests. Refactor the main()
// body to extract this helper if it's not already factored.
func runAudit(t *testing.T, dir string) []string {
	t.Helper()
	// Replicate the main()'s walk + marker check; in the real impl, factor
	// the logic into auditDir(dir string) ([]string, error) so this test
	// can call it directly.
	return auditDir(dir)
}
```

The implementer should refactor `main()` to extract `auditDir(dir string) []string` so the test calls it directly.

- [ ] **Step 3: Wire into Makefile `verify`**

Append to `verify`:

```makefile
	@go run ./scripts/check-test-docs.go --tests-dir tests/
```

- [ ] **Step 4: Run; should detect any test files missing markers**

```bash
go run ./scripts/check-test-docs.go --tests-dir tests/
```

If violations are reported, fix the `tests/<id>.go` files by adding the missing markers (likely just adjusting the comment block format). Re-run until clean.

- [ ] **Step 5: Run full verify**

```bash
make build lint test verify
```

- [ ] **Step 6: Commit**

```bash
git add scripts/check-test-docs.go scripts/check-test-docs_test.go Makefile
git commit -m "feat(scripts): add test doc-comment audit + wire into make verify"
```

If any test files needed marker fixes, add another commit with those.

---

### Task 3: Coverage spot-checks

**Files:**
- Possibly modify: any package below 70% threshold.

- [ ] **Step 1: Measure current coverage across all helper packages**

```bash
for pkg in config internal/matrix internal/overrides internal/jsonrpc \
           internal/teranode internal/svnode internal/compare \
           internal/txgen internal/testrunner internal/observer; do
    pct=$(go test -race -cover ./$pkg/... 2>&1 | grep -oE 'coverage: [0-9.]+%' | head -1)
    printf "%-30s %s\n" "$pkg" "$pct"
done
```

- [ ] **Step 2: For any package below 70%, add targeted tests**

Likely candidates:
- `internal/teranode/notifications.go` — Centrifuge code paths are largely live-only.
- `internal/teranode/p2p_ws.go` — same.

If a package is below 70%, identify uncovered functions via:

```bash
go test -race -coverprofile=cov.out ./<pkg>/...
go tool cover -func=cov.out | awk '$3+0 < 70.0'
```

Add table-driven tests for the lowest-coverage exported functions until the package crosses 70%. Don't chase 100% — the goal is to avoid degradation.

- [ ] **Step 3: Re-run full coverage; commit any new tests**

```bash
git add internal/.../...
git commit -m "test: bump coverage on <pkg> to ≥70%"
```

If no commits are needed (everything was already above), record the measurement in the SP10 review report (Task 8).

---

### Task 4: README full rewrite

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Rewrite README** per spec §3.4 — 11 sections.

```markdown
# Teranode Acceptance Tests

External, black-box acceptance test suite for [Teranode](https://github.com/bsv-blockchain/teranode), driven by the *TNG Teranode Requirements and Test Plan* v1.3 (28/04/2026). Produces a complete report addressing every functional requirement, non-functional requirement, test environment item, source-plan test case, derived test case, and risk in the source document, with a final go / no-go verdict per the source plan's Decision Framework.

## Quickstart

```bash
git clone https://github.com/bsv-blockchain/node-validation.git
cd node-validation
make build              # ~30 sec — builds 4 binaries
make compose-up         # ~2 min — pulls images, starts 12-service stack, mines 110 blocks
make compose-test       # ~30 min in --short mode — runs the full 19-test suite
make compose-down       # tears down (volumes wiped)
```

Then read `report.html` in your browser and `report.json` for machine-readable output.

## Sub-projects

This project was developed in 11 sub-projects, each independently tagged. All complete:

| # | Sub-project | Tag | Delivered |
|---|---|---|---|
| 1 | Reportable Skeleton | `sp1-complete` | go.mod, matrix package, config, testrunner, reporters, CLI |
| 2 | Discovery Pass | `sp2-complete` | docs/discovery.md mapping all 11 Teranode external interfaces |
| 3 | Backend Clients | `sp3-complete` | typed clients for Teranode (RPC, REST, Centrifuge, P2P probe, metrics, health) and SV Node (RPC, ZMQ) |
| 4 | Transaction Generator | `sp4-complete` | internal/txgen/ — funded WIF wallet + builder for P2PKH, P2MS, P2SH, OP_RETURN, chain |
| 4-DOCKER | Test Environment | `sp4-docker-complete` | compose stack: 3 Teranodes + 3 SV nodes + Aerospike + Postgres + Kafka |
| 5 | Cheap Probe Tests | `sp5-complete` | OPS-3, PC-3, NEW-NFR11, NEW-NFR13 |
| 6 | Discovery-Gated Feature Tests | `sp6-complete` | CLIENT-2, NEW-FR8, NEW-FR9, NEW-FR10, NEW-FR11; raw `/p2p-ws` client |
| 7 | Tx-Generation Tests | `sp7-complete` | NEW-FR7, NEW-NFR7, INTER-2 (1000-tx splitter pattern) |
| 8 | Notification + Fixture Tests | `sp8-complete` | CLIENT-1, CLIENT-3, PC-2, IBD-2; gen-fixtures (30+10 fixtures) |
| 9 | Long-Observation + Perf | `sp9-complete` | PC-1, INTER-1, PERF-1; observer package; reorg-induction |
| 10 | Hardening Pass | `sp10-complete` | pipeline tests, doc audit, README + operator guide + verdict interpretation |

## Exit codes

| Code | Verdict | Meaning |
|---|---|---|
| 0 | GO | All Critical pass; all Important pass or have documented mitigation. |
| 1 | NO_GO | A Critical requirement failed, or a harness ERROR occurred. |
| 2 | CONDITIONAL_GO | All Critical pass; one or more Important fail or were not run. |
| 3 | INCOMPLETE | Required automated coverage missing, or required documentation review not yet supplied via `--reviewer-overrides`. |
| 4 | Config error | Bad / missing configuration. |

## Test catalogue

19 acceptance tests + 5 documentation/contractual rows requiring reviewer overrides.

| ID | Severity | Source | What it measures |
|---|---|---|---|
| PC-1 | Critical | Plan §PC-1 | Parallel-node consistency (Teranode vs SV) over observation window + reorg convergence |
| PC-2 | Critical | Plan §PC-2 | Historical script-rule parity across 30 fixture txs |
| PC-3 | Critical | Plan §PC-3 | Tx round-trip (P2PKH/P2MS/OP_RETURN); standard-parser block parse |
| IBD-2 | Critical | Plan §IBD-2 | Historical UTXO-spend parity across 10 fixture txs |
| INTER-1 | Critical | Plan §INTER-1 | Mixed-network observation + reorg-induction convergence |
| INTER-2 | Critical | Plan §INTER-2 | 1000-tx propagation; ≥99% in 10s each direction |
| CLIENT-1 | Critical | Plan §CLIENT-1 | Notification session, broadcast, mid-window reconnect |
| CLIENT-3 | Critical | Plan §CLIENT-3 | 500-tx ordered broadcast; block height ordering |
| PERF-1 | Important | Plan §PERF-1 | TPS ramp [10,50,100,250]; per-rate p50/p95 latency |
| OPS-3 | Important | Plan §OPS-3 | Metrics + health endpoints, 5 required metric categories |
| CLIENT-2 | Important | Plan §CLIENT-2 | Extended-tx-format submission; standard-format backward compat |
| NEW-FR7 | Advisory | Derived (FR-7) | 25-deep unconfirmed chain |
| NEW-FR8 | Advisory | Derived (FR-8) | Fee estimation endpoint (FEATURE_NOT_AVAILABLE per discovery) |
| NEW-FR9 | Advisory | Derived (FR-9) | Double-spend detection + /p2p-ws notification |
| NEW-FR10 | Advisory | Derived (FR-10) | Historical data access latency p95 ≤ 100ms |
| NEW-FR11 | Advisory | Derived (FR-11) | Mempool query (most queries absent per discovery) |
| NEW-NFR7 | Advisory | Derived (NFR-7) | Idle-determinism (3 read ops × 100 iterations) |
| NEW-NFR11 | Advisory | Derived (NFR-11) | TLS + auth probe (plain HTTP recorded as finding) |
| NEW-NFR13 | Advisory | Derived (NFR-13) | Rate-limit discovery probe (configurable rate × duration) |

The 5 documentation/contractual rows requiring reviewer overrides:
- IBD-1 — Historical Validation Evidence Review (DOCUMENTATION_REVIEW)
- FR-4 — Historical Chain Validation Evidence (DOCUMENTATION_REVIEW; verified by IBD-1 evidence)
- NFR-1 — Upstream Availability Guarantees (LONG_TERM_OBSERVATION; 30-day uptime evidence)
- NFR-8 — API Stability and Versioning (CONTRACTUAL; BSVA documentation)
- NFR-9 — API Pricing and Access Model (CONTRACTUAL; BSVA pricing)

See `docs/operator-guide.md` for how to supply these via the `--reviewer-overrides` YAML.

## Reviewer overrides

The runner alone cannot turn `INCOMPLETE` into `GO`. Five rows require human-supplied evidence (audit reports, uptime CSVs, contracts). The operator passes a YAML at `--reviewer-overrides`:

```yaml
reviewer: "Lars Jorgensen <l.jorgensen@teranode.group>"
reviewed_at: "2026-04-29T14:00:00Z"
overrides:
  IBD-1:
    decision: PASS
    artefacts: ["bsva-audit-2026-q1.pdf"]
    note: "Reviewed BSVA's IBD report dated 2026-03-15."
  # ... FR-4, NFR-1, NFR-8, NFR-9 similarly
```

The override file is recorded into the JSON report under `run.reviewer_overrides` for audit. Without it, the verdict tops out at `INCOMPLETE`.

## Troubleshooting

**`OPS-3` fails with "Metric `teranode_blockassembly_best_block_height` absent"**
→ Likely Teranode-version drift since SP2 discovery (commit `11f5fa6a8…`). Update the metric-name set in `tests/ops3.go` to match v0.15.0-beta-2; commit; rerun.

**`PC-1` reorg phase fails ("svnode-1 did not reorg to T2")**
→ Check that teranode-1 has wallet support / can mine via `generatetoaddress`. The test depends on it being able to mine the longer chain.

**`INTER-2` < 99% propagation**
→ Check legacy-mesh peering: `make compose-logs SERVICE=teranode-1 | grep "addnode\|peer"`. The `bitcoin.conf` overlays per node must list all 5 OTHER nodes.

**`NEW-FR9` no notification on /p2p-ws**
→ Verify port 9906 is exposed on each Teranode in `compose/docker-compose.yml` (host 19906/29906/39906).

**`PERF-1` higher rates fail with errors**
→ Local docker can't sustain 1000 TPS. Reduce `Limits.PERF1MaxTPS` in `config.docker.yaml` to 250.

**`CLIENT-1` / `CLIENT-3` notification client errors**
→ The Centrifuge WebSocket is on `:8090/connection/websocket` (not `:8892`). Per SP2 discovery, the `asset_centrifugeListenAddress` setting is misleading.

For more, see `docs/operator-guide.md` and `docs/verdict-interpretation.md`.

## Version note

The project's compose stack uses `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2`. SP2 discovery (`docs/discovery.md`) was performed against commit `11f5fa6a81c36490e2796561f76a39294fc422b5` from a feature branch. The compose-pinned image may have been built from a different commit; if specific endpoint behaviour drifts, see the troubleshooting section.

To upgrade Teranode:
1. Update `compose/docker-compose.yml` to pin the new image tag.
2. Re-run SP2 discovery if the version is significantly newer; update `docs/discovery.md`.
3. Re-run `make verify` (catches doc/manifest drift).
4. Re-run the suite; address any test failures triggered by API changes.

## Traceability matrix

<!-- TRACEABILITY:START -->
<!-- generated by cmd/gen-traceability — do not edit by hand -->
<!-- TRACEABILITY:END -->

## What this project does NOT do

- It does not run inside or modify Teranode. No source dependency on `github.com/bsv-blockchain/teranode`.
- It does not perform raw P2P packet capture (PC-3 is format-scope only).
- It does not exercise privileged-access scenarios (PERF-2, PERF-3, OPS-1, OPS-2 are excluded).
- It does not produce 30-day uptime evidence for NFR-1 (long-term observation).
- It does not assess pricing / SLAs / API stability — those are `CONTRACTUAL` rows requiring documentation review.
- It does not source PC-2 / IBD-2 fixtures from real testnet history (synthetic regtest fixtures per SP8 design).
- It does not exercise IPv6-only environments.

## License

TBD.
```

- [ ] **Step 2: Run codegen to populate the traceability matrix block**

```bash
make gen
```

- [ ] **Step 3: Verify make verify passes**

```bash
make build lint test verify
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README for SP10 — operator-facing"
```

---

### Task 5: `docs/operator-guide.md`

**Files:**
- Create: `docs/operator-guide.md`

- [ ] **Step 1: Implement** per spec §3.5 — 8 sections.

(The plan provides a section outline; the implementer writes prose for each. Roughly 300-500 lines total.)

```markdown
# Operator Guide — Teranode Acceptance Tests

This guide walks a first-time operator through running the suite, interpreting the report, and submitting findings to BSVA.

## 1. Prerequisites

- Docker Engine ≥ 24
- Docker Compose v2 (the `docker compose` plugin)
- Go 1.22 (only if rebuilding fixtures)
- bash, curl, jq, awk on the host
- ~8 GB RAM free
- ~30 GB disk for images + ephemeral state

## 2. Initial setup

```bash
git clone https://github.com/bsv-blockchain/node-validation.git
cd node-validation
make build              # builds 4 binaries
make build lint test    # static checks must pass
```

If `make lint` reports `staticcheck not installed`, run `go install honnef.co/go/tools/cmd/staticcheck@latest`.

## 3. First live run

```bash
make compose-up
```

Watch the bootstrap output. You should see:
```
==> waiting for svnode-1 (port 18332, method getblockchaininfo)
==> waiting for teranode-1 RPC (port 19292, method getblockchaininfo)
==> generating mining address (svnode-1 wallet)
==> mining 110 blocks
==> waiting up to 60s for the mesh to converge on the same tip
==> Bootstrap complete.
```

If bootstrap stalls at "waiting for mesh to converge", check `make compose-logs SERVICE=teranode-1` for connection errors.

Then run the suite:
```bash
make compose-test
```

This takes ~30 minutes in `--short` mode (or longer if PC-1/INTER-1 are included with longer durations).

When done:
```bash
make compose-down
```

## 4. Reading the report

After `make compose-test`, you have:
- `report.html` — open in a browser. Verdict banner at top; tables of all 58 rows; collapsible per-test details with acceptance checks.
- `report.json` — machine-readable; same data structured.

Key fields:
- `verdict.decision` — one of GO / CONDITIONAL_GO / NO_GO / INCOMPLETE.
- `verdict.exit_code` — matches the CLI exit code (0/1/2/3/4).
- `verdict.rationale` — short string explaining the verdict.
- `requirements[]` — every FR + NFR with `result_status` + evidence.
- `test_cases[]` — every TC + NEW-* with `result.status` + acceptance checks.
- `risks[]` — every R with `mitigation_status` + mitigating tests.

For per-test interpretation see `docs/verdict-interpretation.md`.

## 5. Reviewer overrides workflow

Five rows require human-supplied evidence:
- IBD-1 — BSVA IBD validation evidence
- FR-4 — same evidence as IBD-1
- NFR-1 — 30-day uptime CSV from BSVA
- NFR-8 — BSVA versioning policy doc
- NFR-9 — BSVA pricing doc

Without these, the runner caps verdict at INCOMPLETE.

Create an overrides YAML (e.g. `~/overrides.yaml`):

```yaml
reviewer: "Your Name <your.email@bsvassociation.org>"
reviewed_at: "2026-04-29T14:00:00Z"
overrides:
  IBD-1:
    decision: PASS
    artefacts: ["bsva-audit-2026-q1.pdf"]
    note: "Reviewed BSVA's IBD report; methodology covers consensus rule changes through 2026-Q1."
  FR-4:
    decision: PASS
    artefacts: ["bsva-audit-2026-q1.pdf"]
    note: "Same audit covers FR-4."
  NFR-1:
    decision: PASS
    artefacts: ["bsva-uptime-jan-mar-2026.csv"]
    note: "30-day window 2026-03-01..2026-03-30 shows 99.94% availability."
  NFR-8:
    decision: PASS
    artefacts: ["bsva-versioning-policy.pdf"]
    note: "Reviewed; minimum 6-month deprecation window honoured."
  NFR-9:
    decision: PASS
    artefacts: ["bsva-pricing-2026.pdf"]
    note: "Pricing is competitive with current SV Node operator costs at TNG's anticipated volumes."
```

Then run:
```bash
./bin/teranode-acceptance --short --config config.docker.yaml --reviewer-overrides ~/overrides.yaml
```

The override file is embedded in the JSON report under `run.reviewer_overrides` for audit.

## 6. Reporting findings to BSVA

If a test fails or surfaces FEATURE_NOT_AVAILABLE, capture:
1. The full `report.json` and `report.html`.
2. The compose stack logs: `docker compose -f compose/docker-compose.yml logs > stack.log`.
3. The Teranode version: `docker compose -f compose/docker-compose.yml exec teranode-1 cat /app/VERSION` (if the file exists) or note the image tag (`v0.15.0-beta-2`).
4. The SP2 discovery commit: see `docs/discovery.md` frontmatter.

Send to BSVA with the failing test ID, the acceptance check that failed, and the `Detail` field's content.

## 7. Refreshing the suite when Teranode upgrades

1. Update the image pin in `compose/docker-compose.yml`.
2. Re-run discovery: clone the new Teranode commit to `../teranode/`, then re-run SP2's 9 Explore agents (see `docs/superpowers/plans/2026-04-29-sp2-discovery-pass.md`).
3. Update `docs/discovery.md` and `docs/discovery.yaml` with the new findings.
4. `make verify` to catch traceability drift.
5. Run the suite. Address any newly-failing tests by updating the test logic if Teranode renamed an endpoint or behaviour.
6. Tag the new state.

## 8. Common operations

**Run a single test:**
```bash
./bin/teranode-acceptance --config config.docker.yaml --only PC-1
```

**Skip a flaky test:**
```bash
./bin/teranode-acceptance --config config.docker.yaml --skip INTER-2
```

**Compare two runs:**
```bash
diff <(jq '.test_cases[] | {id, status: .result.status}' run1.json) \
     <(jq '.test_cases[] | {id, status: .result.status}' run2.json)
```

**Increase per-test timeout** (PC-1 / INTER-1 long runs):
```bash
./bin/teranode-acceptance --short --test-timeout 90m --config config.docker.yaml
```

**Tear down stuck stack:**
```bash
docker compose -f compose/docker-compose.yml down -v --remove-orphans
docker volume prune -f
```

## Example overrides file

A copy of an example overrides YAML lives at `docs/operator-guide-overrides-example.yaml`. **DO NOT use it unmodified** — replace the reviewer name, artefact references, and notes with your real evidence.
```

- [ ] **Step 2: Create the example overrides file**

```yaml
# docs/operator-guide-overrides-example.yaml
#
# EXAMPLE — DO NOT USE FOR REAL OPERATIONS
#
# This file demonstrates the reviewer-overrides YAML schema. Replace
# every value with your actual evidence before running the suite with
# --reviewer-overrides=this-file.

reviewer: "EXAMPLE — Replace with your name + email"
reviewed_at: "2026-01-01T00:00:00Z"
overrides:
  IBD-1:
    decision: PASS
    artefacts: ["EXAMPLE-bsva-audit.pdf"]
    note: "EXAMPLE — Replace with your real review of BSVA's IBD evidence."
  FR-4:
    decision: PASS
    artefacts: ["EXAMPLE-bsva-audit.pdf"]
    note: "EXAMPLE — Replace."
  NFR-1:
    decision: PASS
    artefacts: ["EXAMPLE-uptime.csv"]
    note: "EXAMPLE — Replace with real uptime evidence."
  NFR-8:
    decision: PASS
    artefacts: ["EXAMPLE-versioning-policy.pdf"]
    note: "EXAMPLE — Replace."
  NFR-9:
    decision: PASS
    artefacts: ["EXAMPLE-pricing.pdf"]
    note: "EXAMPLE — Replace."
```

- [ ] **Step 3: Commit**

```bash
git add docs/operator-guide.md docs/operator-guide-overrides-example.yaml
git commit -m "docs: add operator guide + example overrides YAML"
```

---

### Task 6: `docs/verdict-interpretation.md`

**Files:**
- Create: `docs/verdict-interpretation.md`

- [ ] **Step 1: Implement** per spec §3.6 — table of (test, plausible-status, meaning, action).

The implementer writes ~80-100 rows. Format:

```markdown
# Verdict Interpretation Guide

Per-test, per-status reference. Use this when a test's status surprises you.

## Critical tests

### PC-1 — Parallel Node Comparison

| Status | Means | Action |
|---|---|---|
| PASS | Both nodes agreed throughout observation; reorg induction succeeded | None |
| FAIL ("Zero divergence...") | At least one observation showed Teranode vs SV Node disagreement on best-block | Investigate which side diverged; check chain integrity on both nodes |
| FAIL ("reorged=false") | Reorg induction failed — `invalidateblock` or `generatetoaddress` errored, or convergence didn't happen | Check teranode-1 wallet support; check connectivity between teranode-1 and svnode-1 |
| ERROR ("bootstrap...") | Funder couldn't bootstrap | SV Node wallet probably not enabled; check `disablewallet=0` on svnode-1 |
| SKIPPED ("client(s) not configured") | Required clients (Teranode RPC, SV Node RPC, TxGen) not all set | Set them in config.docker.yaml |

### PC-2 — Historical Script Regression
... (similar table for each test)

(Continue for all 19 tests + the 5 doc-review rows.)
```

- [ ] **Step 2: Commit**

```bash
git add docs/verdict-interpretation.md
git commit -m "docs: add verdict-interpretation reference"
```

---

### Task 7: SP10 done-check + final code-review

**Files:**
- Create: `scripts/sp10-done-check.sh`
- Captures review report at `docs/superpowers/reviews/2026-05-01-sp10-final-review.md`

- [ ] **Step 1: Create `scripts/sp10-done-check.sh`** per spec §3.8

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

echo "==> Coverage thresholds (≥70%)"
for pkg in config internal/matrix internal/overrides internal/jsonrpc \
           internal/teranode internal/svnode internal/compare \
           internal/txgen internal/testrunner internal/observer; do
    pct=$(go test -race -cover ./$pkg/... 2>&1 | grep -oE 'coverage: [0-9.]+%' | grep -oE '[0-9.]+' | head -1)
    if [ -z "$pct" ]; then continue; fi
    awk -v p="$pct" -v t=70 'BEGIN { if (p+0 < t+0) exit 1 }' \
        || { echo "FAIL: $pkg coverage $pct% < 70%"; exit 1; }
    printf "    %-30s %s%%\n" "$pkg" "$pct"
done

echo "==> Build-doc §13 mechanical checks"
make verify
if grep -q "github.com/bsv-blockchain/teranode " go.sum 2>/dev/null; then
    echo "FAIL: bsv-blockchain/teranode is a dependency"; exit 1
fi

echo "==> Documentation present"
test -s README.md
test -s docs/operator-guide.md
test -s docs/verdict-interpretation.md
test -s docs/operator-guide-overrides-example.yaml

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

- [ ] **Step 2: chmod +x, run static path**

```bash
chmod +x scripts/sp10-done-check.sh
./scripts/sp10-done-check.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/sp10-done-check.sh
git commit -m "chore(sp10): add definition-of-done check"
```

- [ ] **Step 4: Run final code-reviewer agent**

Dispatch with the prompt:

> Final cross-cutting review of the entire Teranode acceptance test suite at `/Users/oskarsson/gitcheckout/node-validation/`. Read all 11 spec docs in `docs/superpowers/specs/`, all 11 plan docs in `docs/superpowers/plans/`, all 10 prior code-review reports in `docs/superpowers/reviews/`, the source plan at `~/Documents/TNG_Teranode_Requirements_and_Test_Plan.md`, and the build doc that started this whole effort.
>
> Look for:
> - Cross-cutting issues that don't fit one sub-project (naming inconsistencies, divergent error-handling styles, package boundary violations, etc.)
> - Open spec gaps deferred to "SP10" or "later" that didn't get picked up
> - Final spot-checks on every spec's definition-of-done — anything not actually delivered
> - The build-doc §13 items: each one verified
> - Whether the project is actually shippable to an operator
>
> Save findings to `docs/superpowers/reviews/2026-05-01-sp10-final-review.md`. Structure as Critical / Important / Minor / Strengths / Spec coverage gaps / Recommendation.

- [ ] **Step 5: Address any Critical findings inline**

- [ ] **Step 6: Capture review report**

```bash
git add docs/superpowers/reviews/2026-05-01-sp10-final-review.md
git commit -m "docs: capture SP10 final review report"
```

- [ ] **Step 7: Tag SP10 complete**

```bash
git tag -a sp10-complete -m "SP10 — Hardening Pass complete; project DONE"
```

---

## Self-review checklist (planner)

- [x] Spec coverage — every section of the SP10 spec is implemented.
- [x] Pipeline tests cover all 4 verdict outcomes.
- [x] Doc-comment audit covers each `tests/<id>.go`.
- [x] README rewrite hits all 11 sections from spec §3.4.
- [x] Operator guide hits all 8 sections from spec §3.5.
- [x] Verdict-interpretation reference covers ~80 rows.
- [x] Done-check is meta — verifies all 10 prior done-checks + audit + coverage + docs.
- [x] Final code-review agent dispatched with full context.
