# SP2 — Discovery Pass (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP2 / 10 — Upstream Discovery
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-28
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Upstream under analysis:** `/Users/oskarsson/gitcheckout/teranode/` (locally cloned)
**Status:** awaiting user review

---

## 1. Purpose

Map Teranode's actual external interfaces — the surfaces SP3's clients and SP5+ tests will
depend on — by reading the upstream Go source. Build doc §2 lists *what* must be discovered;
this spec defines *how*, *what artefacts* are produced, and *how completeness is verified*.

The output of SP2 unblocks SP3 (clients) and informs SP6 (discovery-gated feature tests). SP1
runs in parallel and is unblocked by neither.

## 2. Scope

### In scope for SP2

- Read-only analysis of `/Users/oskarsson/gitcheckout/teranode/`.
- Production of `docs/discovery.md` covering all 11 surfaces listed in §4.
- Production of `docs/discovery.yaml` machine-readable companion.
- Production of `docs/discovery-method.md` reproducibility note.
- A small Go ref-link checker at `scripts/check-refs.go` that validates every `path:line`
  reference in `discovery.md` resolves.
- A `scripts/sp2-done-check.sh` script asserting SP2's definition of done.
- A `superpowers:code-reviewer` agent review of the final artefacts.

### Out of scope for SP2

- Any code in `internal/teranode/`, `internal/svnode/`, or any test in `tests/` — SP3 / SP5+.
- Live verification that the documented endpoints actually respond — SP3.
- Any modifications to `../teranode/` — read-only.
- Any decision about *which* surface to prefer when multiple are available — SP3 picks based
  on the discovery doc's recommendations.

## 3. Methodology

### 3.1 Sub-agent fan-out

Nine parallel `Explore` agents (read-only, no isolation), each responsible for one or two
related surfaces. Agents run in a single batch (one assistant message with nine `Agent` tool
calls) so they execute concurrently.

| Agent | Surface(s) |
|---|---|
| 1 | JSON-RPC service + auth model |
| 2 | REST / Asset HTTP API |
| 3 | Notifications (WebSocket / SSE / gRPC stream / Kafka detection) |
| 4 | P2P listener |
| 5 | Metrics + Health endpoints |
| 6 | Extended transaction format detection |
| 7 | Mempool query, `testmempoolaccept` analogue, fee estimation |
| 8 | Double-spend detection / notification |
| 9 | Settings / configuration files (`settings.conf`, `settings_local.conf`, env vars) — provides the cross-cutting "default ports / default flags" index that the other agents anchor against |

Agent 9's output is consulted by the orchestrator (me) when consolidating findings — if Agent 1
reports an RPC port and Agent 9 reports a different default in `settings.conf`, that conflict
is flagged in the discovery doc rather than silently resolved.

### 3.2 Per-agent brief (template)

Each agent receives a self-contained prompt of the form:

> Read `/Users/oskarsson/gitcheckout/teranode/` for surface `<X>`. Produce a markdown section
> with: (1) one-paragraph summary; (2) structured findings table with at minimum
> `endpoint / port / path / method / auth / parameters / response shape`;
> (3) every claim accompanied by a source reference in `relative/path:line` form
> (relative to `/Users/oskarsson/gitcheckout/teranode/`); (4) gaps / ambiguities;
> (5) implementation notes for SP3 clients.
>
> Do not browse outside `/Users/oskarsson/gitcheckout/teranode/`. Do not invent. If a feature is
> absent, say "absent" and document the search method (grep patterns, files inspected). Limit
> output to one markdown section, no preamble.

### 3.3 Synthesis

After the nine agents return:

1. The orchestrator concatenates sections into `docs/discovery.md` in a fixed order (the same
   order as the §4 surface list below, with Agent 9's settings findings as a foundation
   appendix).
2. Adds a top-of-document summary table (one row per surface) and a frontmatter block
   recording the upstream commit SHA, current branch, and discovery date.
3. Resolves cross-agent conflicts (e.g. port disagreements) inline by adding a
   "**Discrepancy noted**" callout citing both source references.
4. Generates `docs/discovery.yaml` from the same findings (single source of truth — but the
   markdown is hand-curated for prose and the YAML is mechanically structured).

### 3.4 Pinning the discovery

`docs/discovery.md` frontmatter records:

```yaml
upstream_commit: <output of `git -C ../teranode rev-parse HEAD`>
upstream_branch: <output of `git -C ../teranode rev-parse --abbrev-ref HEAD`>
discovered_at:   <RFC3339 timestamp>
discovered_by:   <runner identity>
```

SP3 starts with a check that the upstream SHA still matches; mismatch triggers a re-run of
SP2. The check is a one-line bash assertion in SP3's setup script.

## 4. Surfaces (11)

| # | Surface | Required findings |
|---|---|---|
| 1 | JSON-RPC service | port, route, method list (full enumeration), auth model, error shape |
| 2 | REST / Asset HTTP API | port, route prefix, endpoints for tx / block / header / UTXO retrieval, response format |
| 3 | Notifications | mechanism, URL / port / topic, message format, reconnect / catch-up semantics |
| 4 | P2P listener | port, handshake (BSV-standard or custom), peer-manifest mechanism |
| 5 | Metrics endpoint | port, path, exposition format (Prometheus or other), enumerated metric names |
| 6 | Health endpoint | port, path, response shape |
| 7 | Extended transaction format | advertised? where? format spec? backward-compat path? |
| 8 | `testmempoolaccept` analogue | present? signature? semantics? |
| 9 | Fee estimation | endpoint? signature? priority levels? |
| 10 | Mempool query / filtering | endpoint? filter dimensions (fee rate, ancestor / descendant)? stats endpoint? |
| 11 | Double-spend detection / notification | detection logic? notification channel? message format? |

Auth-model findings are recorded **per-surface** (each surface may have its own auth posture)
and additionally summarised in a top-of-doc table.

## 5. Output artefacts

### 5.1 `docs/discovery.md`

Single human-readable file, ~600-1200 lines, structure:

```markdown
---
upstream_commit: <sha>
upstream_branch: <name>
discovered_at:   <iso8601>
---

# Teranode External Interfaces — Discovery

## Summary table
<row per surface: name, present?, port, auth, source-of-truth file>

## 1. JSON-RPC service
### Summary
### Findings
<table>
### Source references
- `services/rpc/server.go:42` — RPC server entrypoint
- `services/rpc/handlers.go:118-340` — registered methods
### Gaps / ambiguities
### Implementation notes for SP3
### Auth model

## 2. REST / Asset HTTP API
... (same structure)

## 3. Notifications
...

## 11. Double-spend detection / notification
...

## Appendix A — Settings & default ports
<from Agent 9>

## Appendix B — Discovery methodology
<pointer to docs/discovery-method.md>
```

### 5.2 `docs/discovery.yaml`

Machine-readable companion. Schema (informal — full schema in §5.3):

```yaml
upstream_commit: "abcdef1234..."
discovered_at: "2026-04-28T15:00:00Z"
surfaces:
  - id: "json_rpc"
    name: "JSON-RPC service"
    present: true
    endpoint:
      scheme: "http"
      port: 9292
      path: "/"
    auth:
      mechanism: "basic"     # or "cookie", "api_key", "jwt", "none"
      header: "Authorization"
      notes: ""
    methods:
      - name: "getbestblockhash"
        signature: "() -> string"
        source_ref: "services/rpc/handlers.go:124"
      - name: "getblock"
        signature: "(hash: string, verbosity: int) -> object"
        source_ref: "services/rpc/handlers.go:155"
    source_refs:
      - "services/rpc/server.go:42"
    notes: ""
  - id: "rest_asset"
    ...
  - id: "notifications"
    present: true
    mechanism: "websocket"   # or "sse", "grpc_stream", "kafka", "absent"
    ...
  - id: "fee_estimation"
    present: false
    search_method: "grep -r 'EstimateFee\\|fee_estimate\\|estimatesmartfee' ../teranode/"
    source_refs: []
    notes: "No symbol matching these patterns. Closest match: ..."
  ...
```

Eleven entries, one per surface. Tri-state `present` (true / false / partial) where ambiguity
exists. The YAML is the source of truth for the markdown summary table — both are produced
together.

### 5.3 `docs/schema/discovery.schema.json`

JSON Schema for `docs/discovery.yaml`. Validated by `scripts/check-refs.go` (re-used for both
ref-link checking and schema validation). Concrete schema:

```
type: object
required: [upstream_commit, discovered_at, surfaces]
properties:
  upstream_commit:  { type: string, pattern: "^[0-9a-f]{7,40}$" }
  discovered_at:    { type: string, format: date-time }
  surfaces:
    type: array
    minItems: 11
    maxItems: 11
    items:
      type: object
      required: [id, name, present, source_refs, notes]
      properties:
        id:           { type: string, enum: [json_rpc, rest_asset, notifications, p2p, metrics, health, extended_tx, testmempoolaccept, fee_estimation, mempool_query, double_spend] }
        name:         { type: string }
        present:      { enum: [true, false, partial] }
        endpoint:     { type: object }
        auth:         { type: object }
        methods:      { type: array }
        source_refs:  { type: array, items: { type: string, pattern: ".+:[0-9]+(-[0-9]+)?" } }
        notes:        { type: string }
        # surface-specific extensions allowed
```

### 5.4 `docs/discovery-method.md`

One-page reproducibility note:

- Which agent looked at which directories.
- Which `grep` / pattern searches each ran.
- Which files were inspected by hand.
- Notable false-positives or near-misses (so a future re-run knows what to ignore).

This is what a future engineer needs to extend or refresh discovery without re-deriving the
methodology from scratch.

### 5.5 `scripts/check-refs.go`

Standalone Go tool, ~80-120 lines. Two responsibilities:

1. **Ref-link check.** Parse `docs/discovery.md`, extract every `path:line` and `path:line-line`
   reference, verify the file exists in `../teranode/`, verify the line range is in bounds.
   Emit a list of broken refs with line numbers in the markdown source.
2. **Schema validation.** Validate `docs/discovery.yaml` structurally against the rules in
   §5.3. Implementation is hand-written Go (~150 lines) using `gopkg.in/yaml.v3` (already in
   the build-doc §5 allow-list) — no JSON-Schema library dependency. The schema in
   `docs/schema/discovery.schema.json` is the human-readable contract; the Go validator is
   the executable enforcement. The schema is small and stable enough that this is cheaper
   than pulling a third-party validator and doesn't violate build doc §5's dependency
   allow-list.

Both checks run under `make verify` (added to SP1's existing target):

```makefile
verify: gen
	./bin/gen-traceability
	@git diff --exit-code README.md docs/traceability.md ...
	go run ./scripts/check-refs.go --discovery docs/discovery.md --schema docs/schema/discovery.schema.json --yaml docs/discovery.yaml
```

### 5.6 `scripts/sp2-done-check.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

# 1. All 11 surface section headings present
for s in "JSON-RPC service" "REST / Asset HTTP API" "Notifications" \
         "P2P listener" "Metrics endpoint" "Health endpoint" \
         "Extended transaction format" "testmempoolaccept" \
         "Fee estimation" "Mempool query" "Double-spend detection"; do
    grep -q "^## .*$s" docs/discovery.md || { echo "missing surface: $s"; exit 1; }
done

# 2. Frontmatter records upstream commit
grep -q '^upstream_commit:' docs/discovery.md

# 3. YAML schema-valid + ref-links resolve
go run ./scripts/check-refs.go --discovery docs/discovery.md \
    --schema docs/schema/discovery.schema.json \
    --yaml docs/discovery.yaml

# 4. Reviewer agent has no critical findings
grep -q '^critical_findings: 0$' docs/discovery-review.md
```

The reviewer agent's output is captured to `docs/discovery-review.md` so it's auditable; the
done-check looks for its summary line.

## 6. Verification & quality gates

| Gate | Mechanism |
|---|---|
| All 11 surfaces documented | `grep` checks in `sp2-done-check.sh` |
| YAML schema-valid | `check-refs.go` schema validation |
| All `path:line` refs resolve | `check-refs.go` ref-link check |
| No speculative claims | `superpowers:code-reviewer` agent prompted to flag any unsupported claim |
| Cross-agent conflicts surfaced | Orchestrator inserts "Discrepancy noted" callouts at synthesis time; reviewer agent verifies all conflicts are explicit |
| Reproducibility | `discovery-method.md` records search methods |
| Pinned upstream | Frontmatter SHA |

## 7. Definition of done

All true:

- `docs/discovery.md` exists with all 11 surface sections, structured per §5.1.
- `docs/discovery.yaml` exists, validates against `docs/schema/discovery.schema.json`, has
  exactly 11 surface entries.
- `docs/discovery-method.md` exists.
- `scripts/check-refs.go` exists; `make verify` passes (no broken refs, schema valid).
- `scripts/sp2-done-check.sh` exits 0.
- `superpowers:code-reviewer` agent's report at `docs/discovery-review.md` reports zero
  Critical findings.
- Frontmatter records the upstream commit SHA.
- Project still compiles (`make build`) and SP1 tests still pass (`make test`).

## 8. Risks tracked through SP2

| # | Risk | Mitigation |
|---|---|---|
| A | Teranode codebase too sprawling, agents miss code paths | Agent 9 (settings/config) provides anchor for other agents; reviewer agent does cross-check pass; `path:line` linter catches phantom refs. |
| B | Surface gated by build flag or feature flag we miss | Per-section "Gaps / ambiguities" subsection forces the agent to explicitly note flag-gating; reviewer prompt asks for these specifically. |
| C | Speculative claims sneak past reviewer | `path:line` ref required for every claim; ref-link checker validates structure; reviewer prompt explicitly tasks it with hunting unsupported claims. |
| D | Discovery rots between SP2 and downstream sub-projects | Pinned commit SHA; SP3 setup checks SHA match; documented refresh procedure in `discovery-method.md`. |
| E | Conflict between settings file defaults and code-evident behaviour | Synthesis step inserts "Discrepancy noted" callouts; reviewer verifies all such callouts include both source references. |
| F | Hand-written validator drifts from `discovery.schema.json` (the human-readable contract) | Validator includes a self-test that loads the JSON schema, parses its `properties`, and asserts the validator's checks correspond 1-to-1 with the schema's `required` and `enum` fields — drift fails the build. |

## 9. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Output is `discovery.md` + `discovery.yaml` companion + `discovery-method.md` + `discovery-review.md` | per user (Q1 yes) |
| 2 | `scripts/check-refs.go` kept in repo as tooling | per user (Q2 keep) |
| 3 | Sub-agents run without worktree isolation | per user (Q3 no isolation) |
| 4 | 9 parallel Explore agents, single message | drafter |
| 5 | Pinned upstream commit SHA in frontmatter | drafter |
| 6 | Reviewer agent output captured to `docs/discovery-review.md` | drafter |
| 7 | Tri-state `present` field (true / false / partial) | drafter |
| 8 | YAML schema is the structural source of truth, markdown is the prose | drafter |

## 10. Out-of-scope reminders

SP2 produces documentation only. It does not:
- Open any network connection.
- Modify `../teranode/`.
- Modify the working tree outside `docs/` and `scripts/`.
- Validate that documented endpoints respond — that's SP3.
- Pick *which* surface SP3 should use when multiple options exist — SP3 decides.
