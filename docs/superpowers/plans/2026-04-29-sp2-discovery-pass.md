# SP2 — Discovery Pass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce `docs/discovery.md` plus a machine-readable `docs/discovery.yaml` companion that map every Teranode external interface SP3+ will rely on, by reading the upstream Go source at `/Users/oskarsson/gitcheckout/teranode/`.

**Architecture:** 9 parallel `Explore` sub-agents fan out over the upstream tree (one per surface group), return structured markdown sections, the orchestrator synthesises them into `docs/discovery.md`, derives `docs/discovery.yaml` from the same findings, and a hand-written ~150-line Go validator at `scripts/check-refs.go` enforces (a) every `path:line` reference resolves and (b) the YAML is structurally valid.

**Tech Stack:** Go 1.22 (validator and YAML structural checks), `gopkg.in/yaml.v3`, no third-party schema library, no network calls.

---

### Task 1: Pre-flight check on upstream and capture commit SHA

**Files:**
- Create: `docs/discovery-method.md` (header only)

- [ ] **Step 1: Confirm upstream is present and recordable**

```bash
test -d /Users/oskarsson/gitcheckout/teranode/.git \
  && echo OK || (echo "FAIL: ../teranode is not a git repo" && exit 1)
sha=$(git -C /Users/oskarsson/gitcheckout/teranode rev-parse HEAD)
branch=$(git -C /Users/oskarsson/gitcheckout/teranode rev-parse --abbrev-ref HEAD)
echo "Upstream Teranode @ $branch / $sha"
```

Expected: prints a real branch name and 40-char SHA.

- [ ] **Step 2: Write the discovery-method.md header**

```markdown
# Discovery methodology

This document records how `docs/discovery.md` was produced so a future
engineer can reproduce or extend the discovery without re-deriving the
methodology from scratch.

## Upstream pinning

- Repository: `/Users/oskarsson/gitcheckout/teranode/`
- Commit:     `<filled in by Task 4>`
- Branch:     `<filled in by Task 4>`
- Captured:   `<filled in by Task 4>`

## Agent fan-out

Nine parallel `Explore` agents, each over `/Users/oskarsson/gitcheckout/teranode/`.

| # | Surface(s) | Search anchors |
|---|---|---|
| 1 | JSON-RPC + auth model | (filled in by Task 3) |
| 2 | REST / Asset HTTP API | (filled in by Task 3) |
| 3 | Notifications (WS / SSE / gRPC / Kafka) | (filled in by Task 3) |
| 4 | P2P listener | (filled in by Task 3) |
| 5 | Metrics + Health | (filled in by Task 3) |
| 6 | Extended transaction format | (filled in by Task 3) |
| 7 | Mempool + testmempoolaccept + fee estimation | (filled in by Task 3) |
| 8 | Double-spend detection / notification | (filled in by Task 3) |
| 9 | Settings / configuration | (filled in by Task 3) |

## Synthesis rules

1. Each agent's output is appended verbatim to `docs/discovery.md` in the order above.
2. Cross-agent conflicts (e.g. port disagreements between code and `settings.conf`) are inserted by the orchestrator as `**Discrepancy noted**` callouts citing both source references.
3. `docs/discovery.yaml` is derived programmatically from the same findings; the markdown is the human-readable face, the YAML is the structural source of truth.
```

- [ ] **Step 3: Commit**

```bash
mkdir -p docs
# (file already created above)
git add docs/discovery-method.md
git commit -m "docs(sp2): start discovery-method note"
```

---

### Task 2: Output skeleton

**Files:**
- Create: `docs/discovery.md` (header only)
- Create: `docs/discovery.yaml` (frontmatter only)
- Create: `docs/schema/discovery.schema.json`

- [ ] **Step 1: Write the discovery.md skeleton**

```markdown
---
upstream_commit: "<filled in by Task 4>"
upstream_branch: "<filled in by Task 4>"
discovered_at:   "<filled in by Task 4>"
---

# Teranode External Interfaces — Discovery

> **Upstream pinning.** The findings below were derived from the
> Teranode commit recorded in the frontmatter. SP3 starts by checking
> the SHA still matches; if not, re-run SP2.

## Summary table

| # | Surface | Present? | Endpoint / port | Auth | Source-of-truth file |
|---|---|---|---|---|---|
| 1 | JSON-RPC service | (filled in by Task 4) | | | |
| 2 | REST / Asset HTTP API | | | | |
| 3 | Notifications | | | | |
| 4 | P2P listener | | | | |
| 5 | Metrics endpoint | | | | |
| 6 | Health endpoint | | | | |
| 7 | Extended transaction format | | | | |
| 8 | testmempoolaccept analogue | | | | |
| 9 | Fee estimation | | | | |
| 10 | Mempool query / filtering | | | | |
| 11 | Double-spend detection / notification | | | | |

(Sections 1-11 follow, plus appendices.)
```

- [ ] **Step 2: Write the discovery.yaml frontmatter**

```yaml
# docs/discovery.yaml — machine-readable companion to discovery.md.
# Authoritative for SP3 client code; markdown is for humans.
upstream_commit: "<filled in by Task 4>"
discovered_at:   "<filled in by Task 4>"
surfaces: []
```

- [ ] **Step 3: Write the JSON Schema (human-readable contract)**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Teranode discovery YAML",
  "type": "object",
  "required": ["upstream_commit", "discovered_at", "surfaces"],
  "properties": {
    "upstream_commit": { "type": "string", "pattern": "^[0-9a-f]{7,40}$" },
    "discovered_at": { "type": "string", "format": "date-time" },
    "surfaces": {
      "type": "array",
      "minItems": 11,
      "maxItems": 11,
      "items": {
        "type": "object",
        "required": ["id", "name", "present", "source_refs", "notes"],
        "properties": {
          "id": {
            "type": "string",
            "enum": [
              "json_rpc", "rest_asset", "notifications", "p2p",
              "metrics", "health", "extended_tx",
              "testmempoolaccept", "fee_estimation",
              "mempool_query", "double_spend"
            ]
          },
          "name": { "type": "string" },
          "present": { "enum": [true, false, "partial"] },
          "endpoint": { "type": "object" },
          "auth": { "type": "object" },
          "methods": { "type": "array" },
          "source_refs": {
            "type": "array",
            "items": { "type": "string", "pattern": ".+:[0-9]+(-[0-9]+)?" }
          },
          "notes": { "type": "string" }
        }
      }
    }
  }
}
```

- [ ] **Step 4: Commit**

```bash
mkdir -p docs/schema
git add docs/discovery.md docs/discovery.yaml docs/schema/discovery.schema.json
git commit -m "docs(sp2): scaffold discovery.md, discovery.yaml, schema"
```

---

### Task 3: Dispatch the 9 Explore agents

This task is performed by an orchestrator (you). All 9 agent calls go in **one** assistant message with multiple `Agent` tool calls so they run concurrently.

- [ ] **Step 1: Send the 9-agent batch**

Each agent receives the same shape of prompt, customised for its surface(s). Template:

> Read `/Users/oskarsson/gitcheckout/teranode/` for surface(s) `<X>`. Produce a markdown section with:
>
> 1. One-paragraph summary.
> 2. A structured findings table with at minimum: endpoint / port / path / method / auth / parameters / response shape (column subset that applies).
> 3. Every claim accompanied by a `relative/path:line` source reference (relative to `/Users/oskarsson/gitcheckout/teranode/`).
> 4. Gaps / ambiguities / feature-flag gating.
> 5. Implementation notes for SP3 clients.
>
> Constraints: do not browse outside `/Users/oskarsson/gitcheckout/teranode/`. Do not invent. If a feature is absent, say "absent" and document the search method (grep patterns tried, files inspected). Limit output to one markdown section, no preamble.

Surface-specific tail per agent:

1. **JSON-RPC + auth.** Surface ID `json_rpc`. Look in `services/`, `daemon/`, `cmd/`, `pkg/`. Search patterns: `RegisterRPC`, `getbestblockhash`, `BasicAuth`, `cookie`, `JSON-RPC`, JSON-RPC handler tables.
2. **REST / Asset HTTP API.** Surface ID `rest_asset`. Search: `gin.Engine`, `mux.NewRouter`, `http.HandleFunc`, `/api/`, `/asset/`, `/tx/`, `/block/`, `/utxo/`.
3. **Notifications.** Surface ID `notifications`. Search: `websocket`, `gorilla/websocket`, `nhooyr.io/websocket`, `kafka`, `Subscribe`, `EventBus`, `gRPC stream`.
4. **P2P listener.** Surface ID `p2p`. Search: `libp2p`, `peer.AddrInfo`, `p2p.NewServer`, `:8333`, port 9905, BSV magic bytes.
5. **Metrics + Health.** Surface IDs `metrics` and `health`. Search: `prometheus`, `promhttp`, `/metrics`, `/health`, `/healthz`, `Liveness`, `Readiness`.
6. **Extended transaction format.** Surface ID `extended_tx`. Search: `extended`, `tx_extended`, `BIP-239`, `extended format`.
7. **Mempool + testmempoolaccept + fee estimation.** Surface IDs `mempool_query`, `testmempoolaccept`, `fee_estimation`. Search: `mempool`, `testmempoolaccept`, `EstimateFee`, `estimatesmartfee`, `FeeEstimator`, `GetMempool`.
8. **Double-spend detection / notification.** Surface ID `double_spend`. Search: `doublespend`, `double-spend`, `conflict`, `dsdetected`, `doublespent`.
9. **Settings / configuration.** Cross-cuts. Read `settings.conf`, `settings_local.conf`, `chaincfg/`, env-var lookup helpers, default-port constants. Output is a foundational appendix, not its own surface row, but the agent must list every default port and feature flag it finds with refs.

Each agent's output is captured into a per-agent file `docs/discovery/_raw/agent-NN.md` (don't commit these yet).

- [ ] **Step 2: Validate every returned section**

For each returned section, sanity-check:
- Section heading present.
- At least 3 `path:line` refs.
- Either positive findings with concrete data or an explicit "absent" with documented search method.

If any agent returned vague or speculative content ("seems to use", "likely is"), re-dispatch with a stricter prompt.

- [ ] **Step 3: Commit raw outputs**

```bash
mkdir -p docs/discovery/_raw
# Save each agent's section to docs/discovery/_raw/agent-01.md etc.
git add docs/discovery/_raw/
git commit -m "docs(sp2): capture raw output of 9 explore agents"
```

---

### Task 4: Synthesise into discovery.md

**Files:**
- Modify: `docs/discovery.md`
- Modify: `docs/discovery-method.md`

- [ ] **Step 1: Compute frontmatter values**

```bash
sha=$(git -C /Users/oskarsson/gitcheckout/teranode rev-parse HEAD)
branch=$(git -C /Users/oskarsson/gitcheckout/teranode rev-parse --abbrev-ref HEAD)
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
echo "$sha" "$branch" "$now"
```

- [ ] **Step 2: Replace frontmatter placeholders in discovery.md and discovery-method.md**

Edit both files to substitute `<filled in by Task 4>` with the captured values.

- [ ] **Step 3: Concatenate raw agent sections in surface order**

For surfaces 1-11, append the matching agent section under headings `## 1. JSON-RPC service` … `## 11. Double-spend detection / notification`. Append the settings agent's output as `## Appendix A — Settings & default ports`.

- [ ] **Step 4: Cross-check for conflicts and insert "Discrepancy noted" callouts**

For each port, auth model, or default URL appearing in two agents, verify they agree. When they don't, insert:

```markdown
> **Discrepancy noted.** Code at `services/rpc/server.go:42` declares port 9292; `settings_local.conf:18` defaults to 9293. SP3 should resolve via runtime config; recording both for traceability.
```

- [ ] **Step 5: Fill in the summary table at top**

Each row's `Present?` matches the agent's finding. `Endpoint / port` is the canonical location. `Source-of-truth file` is the single most-authoritative `path:line` reference.

- [ ] **Step 6: Commit**

```bash
git add docs/discovery.md docs/discovery-method.md
git commit -m "docs(sp2): synthesise discovery.md from agent outputs"
```

---

### Task 5: Generate discovery.yaml from findings

**Files:**
- Modify: `docs/discovery.yaml`

- [ ] **Step 1: Hand-write the YAML from the agent outputs**

For each of the 11 surfaces, populate one entry. Example skeleton:

```yaml
upstream_commit: "abcdef1234..."     # from Task 4
discovered_at:   "2026-04-29T15:00:00Z"
surfaces:
  - id: json_rpc
    name: "JSON-RPC service"
    present: true                    # or false / partial
    endpoint:
      scheme: "http"
      port: 9292
      path: "/"
    auth:
      mechanism: "basic"             # cookie | basic | api_key | jwt | none
      header: "Authorization"
      notes: ""
    methods:
      - name: "getbestblockhash"
        signature: "() -> string"
        source_ref: "services/rpc/handlers.go:124"
    source_refs:
      - "services/rpc/server.go:42"
      - "services/rpc/handlers.go:118-340"
    notes: ""
  - id: rest_asset
    # ... fill from Agent 2 output
  # ... and so on for all 11 surfaces in this exact order:
  #   json_rpc, rest_asset, notifications, p2p,
  #   metrics, health, extended_tx,
  #   testmempoolaccept, fee_estimation,
  #   mempool_query, double_spend
```

For absent features:

```yaml
  - id: fee_estimation
    name: "Fee estimation"
    present: false
    source_refs: []
    notes: "Searched: grep -r 'EstimateFee|fee_estimate|estimatesmartfee' on commit <sha>. Closest match: <none>."
```

- [ ] **Step 2: Sanity-check entry count**

```bash
grep -c '^  - id:' docs/discovery.yaml
```

Expected: `11`.

- [ ] **Step 3: Commit**

```bash
git add docs/discovery.yaml
git commit -m "docs(sp2): populate discovery.yaml from synthesised findings"
```

---

### Task 6: `scripts/check-refs.go` — ref-link checker and YAML validator

**Files:**
- Create: `scripts/check-refs.go`
- Create: `scripts/check-refs_test.go`

- [ ] **Step 1: Write failing tests**

```go
// scripts/check-refs_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRefs_validReference(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(src, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("see `fake.go:2`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMarkdownRefs(mdPath, dir); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestCheckRefs_outOfBoundsLine(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(src, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("see `fake.go:99`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMarkdownRefs(mdPath, dir); err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestCheckYAML_minimumSurfaceCount(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "x.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
upstream_commit: "abcdef1"
discovered_at: "2026-04-29T00:00:00Z"
surfaces: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkYAML(yamlPath); err == nil {
		t.Error("expected 'surfaces must have 11' error")
	}
}
```

- [ ] **Step 2: Implement the checker**

```go
// Command check-refs validates docs/discovery.md and docs/discovery.yaml
// for SP2: every path:line reference in the markdown must resolve, and
// the YAML must match the structural rules in docs/schema/discovery.schema.json.
//
// Run via: go run ./scripts/check-refs.go --discovery docs/discovery.md \
//                                          --yaml docs/discovery.yaml \
//                                          --upstream /path/to/teranode
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	mdPath := flag.String("discovery", "docs/discovery.md", "discovery markdown")
	yamlPath := flag.String("yaml", "docs/discovery.yaml", "discovery YAML")
	upstream := flag.String("upstream", "/Users/oskarsson/gitcheckout/teranode", "upstream root")
	flag.Parse()

	var errs []string
	if err := checkMarkdownRefs(*mdPath, *upstream); err != nil {
		errs = append(errs, err.Error())
	}
	if err := checkYAML(*yamlPath); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(errs, "\n"))
		os.Exit(1)
	}
	fmt.Println("check-refs: OK")
}

var refPattern = regexp.MustCompile("`([^`\\s]+\\.[A-Za-z0-9]+):(\\d+)(?:-(\\d+))?`")

func checkMarkdownRefs(mdPath, upstreamRoot string) error {
	f, err := os.Open(mdPath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", mdPath, err)
	}
	defer f.Close()

	var problems []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		matches := refPattern.FindAllStringSubmatch(scanner.Text(), -1)
		for _, m := range matches {
			rel, startS, endS := m[1], m[2], m[3]
			start, _ := strconv.Atoi(startS)
			end := start
			if endS != "" {
				end, _ = strconv.Atoi(endS)
			}
			full := filepath.Join(upstreamRoot, rel)
			info, err := os.Stat(full)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q not found (%v)", mdPath, lineNum, rel, err))
				continue
			}
			if info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q is a directory", mdPath, lineNum, rel))
				continue
			}
			lines, err := countLines(full)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: counting lines of %q: %v", mdPath, lineNum, rel, err))
				continue
			}
			if start < 1 || end < start || end > lines {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q line %d-%d out of bounds (file has %d)",
					mdPath, lineNum, rel, start, end, lines))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning %s: %w", mdPath, err)
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func countLines(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	n := strings.Count(string(b), "\n")
	if !strings.HasSuffix(string(b), "\n") {
		n++
	}
	return n, nil
}

// --- YAML validator ---

type discoveryDoc struct {
	UpstreamCommit string             `yaml:"upstream_commit"`
	DiscoveredAt   string             `yaml:"discovered_at"`
	Surfaces       []discoverySurface `yaml:"surfaces"`
}

type discoverySurface struct {
	ID         string   `yaml:"id"`
	Name       string   `yaml:"name"`
	Present    any      `yaml:"present"`
	SourceRefs []string `yaml:"source_refs"`
	Notes      string   `yaml:"notes"`
}

var allowedIDs = map[string]bool{
	"json_rpc": true, "rest_asset": true, "notifications": true,
	"p2p": true, "metrics": true, "health": true,
	"extended_tx": true, "testmempoolaccept": true, "fee_estimation": true,
	"mempool_query": true, "double_spend": true,
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
var refLinePattern = regexp.MustCompile(`.+:[0-9]+(-[0-9]+)?$`)

func checkYAML(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var doc discoveryDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	var errs []string
	if !commitPattern.MatchString(doc.UpstreamCommit) {
		errs = append(errs, fmt.Sprintf("upstream_commit %q is not a 7-40 char hex sha", doc.UpstreamCommit))
	}
	if doc.DiscoveredAt == "" {
		errs = append(errs, "discovered_at is required")
	}
	if len(doc.Surfaces) != 11 {
		errs = append(errs, fmt.Sprintf("surfaces: want 11, got %d", len(doc.Surfaces)))
	}

	seen := map[string]bool{}
	for i, s := range doc.Surfaces {
		ctx := fmt.Sprintf("surfaces[%d] (id=%q)", i, s.ID)
		if !allowedIDs[s.ID] {
			errs = append(errs, fmt.Sprintf("%s: id not in allowed enum", ctx))
		}
		if seen[s.ID] {
			errs = append(errs, fmt.Sprintf("%s: duplicate id", ctx))
		}
		seen[s.ID] = true
		switch v := s.Present.(type) {
		case bool:
		case string:
			if v != "partial" {
				errs = append(errs, fmt.Sprintf("%s: present must be true|false|\"partial\", got %q", ctx, v))
			}
		default:
			errs = append(errs, fmt.Sprintf("%s: present must be bool or \"partial\"", ctx))
		}
		if s.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name required", ctx))
		}
		for j, ref := range s.SourceRefs {
			if !refLinePattern.MatchString(ref) {
				errs = append(errs, fmt.Sprintf("%s: source_refs[%d] %q must be path:line[-line]", ctx, j, ref))
			}
		}
		// "absent" surfaces must explain the search method in notes.
		if pres, ok := s.Present.(bool); ok && !pres {
			if !strings.Contains(strings.ToLower(s.Notes), "search") {
				errs = append(errs, fmt.Sprintf("%s: present=false requires notes documenting search method", ctx))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
```

- [ ] **Step 3: Run, expect pass**

```bash
go test -race ./scripts/...
```

Expected: PASS.

- [ ] **Step 4: Run the checker against the actual artefacts**

```bash
go run ./scripts/check-refs.go \
  --discovery docs/discovery.md \
  --yaml docs/discovery.yaml \
  --upstream /Users/oskarsson/gitcheckout/teranode
```

Expected: `check-refs: OK`. If any ref is broken, fix the markdown and rerun.

- [ ] **Step 5: Wire into make verify**

Edit the `Makefile` to extend the existing `verify` target:

```makefile
verify: gen
	@./bin/gen-traceability
	@git diff --exit-code README.md docs/traceability.md \
	  || (echo "README / traceability.md out of sync — run 'make gen' and commit" && exit 1)
	@if [ -f docs/discovery.md ]; then \
	  go run ./scripts/check-refs.go --discovery docs/discovery.md --yaml docs/discovery.yaml --upstream /Users/oskarsson/gitcheckout/teranode ; \
	fi
```

- [ ] **Step 6: Commit**

```bash
git add scripts/ Makefile
git commit -m "feat(sp2): add check-refs validator and wire into make verify"
```

---

### Task 7: `scripts/sp2-done-check.sh`

**Files:**
- Create: `scripts/sp2-done-check.sh`

- [ ] **Step 1: Write the script**

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "==> all 11 surface section headings present"
for s in \
  "JSON-RPC service" \
  "REST / Asset HTTP API" \
  "Notifications" \
  "P2P listener" \
  "Metrics endpoint" \
  "Health endpoint" \
  "Extended transaction format" \
  "testmempoolaccept" \
  "Fee estimation" \
  "Mempool query" \
  "Double-spend detection"; do
    grep -q "^## .*$s" docs/discovery.md \
      || { echo "FAIL: missing surface section: $s"; exit 1; }
done

echo "==> frontmatter records upstream commit"
grep -q '^upstream_commit:' docs/discovery.md

echo "==> running check-refs"
go run ./scripts/check-refs.go \
  --discovery docs/discovery.md \
  --yaml docs/discovery.yaml \
  --upstream /Users/oskarsson/gitcheckout/teranode

echo "==> SP1 still green"
make build lint test

echo "==> SP2 done-check passed."
```

- [ ] **Step 2: Make executable and run**

```bash
chmod +x scripts/sp2-done-check.sh
./scripts/sp2-done-check.sh
```

Expected: every line passes, "SP2 done-check passed."

- [ ] **Step 3: Commit**

```bash
git add scripts/sp2-done-check.sh
git commit -m "chore(sp2): add definition-of-done check"
```

---

### Task 8: Code review and final closeout

- [ ] **Step 1: Run `superpowers:code-reviewer` agent**

Dispatch with this prompt:

> Review SP2 against `docs/superpowers/specs/2026-04-28-sp2-discovery-pass-design.md`. Specifically:
>
> - All 11 surfaces have a section in `docs/discovery.md` with summary, findings table, source refs, gaps, and SP3 implementation notes.
> - Frontmatter records the upstream commit SHA, branch, and discovery time.
> - `docs/discovery.yaml` has exactly 11 surface entries; every entry has `id` from the allowed enum; `present` is bool or "partial".
> - For every surface marked `present: false`, `notes` documents the search method.
> - `docs/discovery-method.md` exists and lists which agent searched what.
> - `scripts/check-refs.go` exists, passes its tests, and validates the artefacts.
> - `make verify` passes (no broken refs, schema valid).
> - `scripts/sp2-done-check.sh` exits 0.
> - No speculative claims (every assertion has a `path:line` reference).
> - All cross-agent conflicts surfaced as "Discrepancy noted" callouts.
>
> Capture findings as Critical / Important / Minor.

- [ ] **Step 2: Address Critical/Important findings; capture report**

```bash
mkdir -p docs/superpowers/reviews
$EDITOR docs/superpowers/reviews/2026-04-29-sp2-code-review.md
git add docs/superpowers/reviews/
git commit -m "docs: capture SP2 code-review report"
```

- [ ] **Step 3: Tag SP2 completion**

```bash
git tag -a sp2-complete -m "SP2 — Discovery Pass complete"
```

---

## Self-review checklist (run by the planner, not the engineer)

- [x] Spec coverage — every section of `2026-04-28-sp2-discovery-pass-design.md` is implemented by at least one task.
- [x] Sub-agent fan-out — Task 3 dispatches 9 agents in parallel per the spec.
- [x] Output triple — discovery.md + discovery.yaml + discovery-method.md all written.
- [x] Validator — `scripts/check-refs.go` enforces ref-link integrity and YAML structure without violating the build-doc dependency allow-list.
- [x] Pinning — upstream commit SHA captured in both files' frontmatter.
- [x] Done-check — `scripts/sp2-done-check.sh` mechanically verifies the spec's definition-of-done.
- [x] Reviewer agent — Task 8 invokes the code-reviewer per the locked decision.
- [x] No placeholders — every step has runnable content; agent-output bodies are content the orchestrator will fill in at execution time, but the schema and acceptance criteria for each are explicit.
