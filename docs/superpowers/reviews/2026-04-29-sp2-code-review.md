---
sp: SP2
reviewer: superpowers:code-reviewer (Opus 4.7, 1M context)
reviewed_at: 2026-04-29
spec: docs/superpowers/specs/2026-04-28-sp2-discovery-pass-design.md
plan: docs/superpowers/plans/2026-04-29-sp2-discovery-pass.md
critical_findings: 0
important_findings: 2
minor_findings: 4
recommendation: APPROVE_WITH_MINOR
---

# SP2 Discovery Pass — Code Review

## Verdict

**APPROVE_WITH_MINOR.** Every Critical invariant (1–10) holds. `make verify`,
`go test -race ./scripts/...`, and `./scripts/sp2-done-check.sh` all exit 0.
The four required cross-agent discrepancies are surfaced as explicit callouts.
SP2 is in shape to close out and unblock SP3.

The two Important findings are off-by-one errors in `settings.conf` line
numbers inside two of the discrepancy callouts — they do not break the
ref-link checker (which validates structural bounds, not semantic content)
but they do undermine SP2's "every claim resolves to the cited source"
contract. They should be patched before SP3 starts consuming the YAML.

---

## Critical issues (must fix before SP2 closeout)

**None.** All ten core invariants verified:

1. All 11 `## N. <surface>` headings present in `docs/discovery.md` at the
   expected line numbers (31, 186, 309, 393, 478, 570, 669, 752, 793, 837,
   895). Order matches the spec.
2. Frontmatter records the exact upstream commit SHA
   `11f5fa6a81c36490e2796561f76a39294fc422b5` and branch
   `test/longest-chain-double-spend`. Verified live against
   `git -C /Users/oskarsson/gitcheckout/teranode rev-parse HEAD` —
   matches.
3. `docs/discovery.yaml` has exactly 11 `- id:` entries; every `id` is in
   the allowed enum; every entry has `id`, `name`, `present`, `source_refs`,
   `notes`. Tri-state `present` is correctly `true|false|"partial"`.
4. Both `present: false` surfaces (`testmempoolaccept`, `fee_estimation`)
   document the search method in `notes` (the substring "search" appears
   verbatim, satisfying the validator's structural check, and the prose
   names the actual grep targets — `services/rpc/Server.go`,
   `rpcHandlersBeforeInit`, `rpcUnimplemented`, `EstimateFee`,
   `estimatesmartfee`, etc.).
5. `docs/discovery-method.md` exists, lists all 9 agents with concrete
   search anchors filled in (no `(filled in by Task 3)` placeholders
   remain).
6. `scripts/check-refs.go` exists. `go test -race ./scripts/...` PASS.
   Direct invocation of `go run ./scripts/check-refs.go --discovery
   docs/discovery.md --yaml docs/discovery.yaml --upstream
   /Users/oskarsson/gitcheckout/teranode` prints `check-refs: OK`.
7. `make verify` exits 0. SP1 codegen drift gate is unaffected.
8. `./scripts/sp2-done-check.sh` exits 0; every step prints success and
   `make build lint test` chains through cleanly.
9. Spot-checks of `path:line` refs against the upstream tree confirm the
   cited lines contain what the doc claims:
   - `services/rpc/Server.go:156-222` — `rpcHandlersBeforeInit` map. ✓
   - `services/rpc/Server.go:905-936` — `checkAuth`. ✓
   - `services/asset/centrifuge_impl/centrifuge.go:221` —
     `AddHTTPHandler("/connection/websocket", …)`. ✓
   - `settings/settings.go:157` — `CentrifugeListenAddress: … ":8892"`. ✓
   - `settings/settings.go:163` — `SignHTTPResponses: getBool(
     "asset_sign_http_responses", false, …)`. ✓
   - `settings.conf:612` — `http_sign_response = true`. ✓
   - `services/validator/Validator.go:490-542` — conflict-handling
     branch with `ErrSpent` / `ErrTxConflicting`. ✓
10. All 4 required cross-agent discrepancies surfaced as
    `**Discrepancy noted**` callouts:
    - L345 — Centrifuge `:8892` vs Asset HTTP `:8090`.
    - L1044 — `SUBTREE_VALIDATION_GRPC_PORT=8086` vs Go fallback `:8089`.
    - L1052 — `P2P_GRPC_PORT=9904` vs Go fallback `:9906`.
    - L1181 — `http_sign_response` (conf, ignored) vs
      `asset_sign_http_responses` (Go).

---

## Important issues (should fix soon)

### I1. Off-by-one in two Appendix-A discrepancy callouts

Two of the four required discrepancy callouts cite the wrong line
numbers in `settings.conf`:

- L1044: cites `settings.conf:140` for `SUBTREE_VALIDATION_GRPC_PORT=8086`.
  The actual line in upstream is **141** (line 140 is `TERANODE_RPC_PORT`).
- L1052: cites `settings.conf:131` for `P2P_GRPC_PORT=9904`. The actual
  line is **132** (line 131 is `PROPAGATION_HTTP_PORT`).

The discrepancy *content* is correct — those env names and values exist
in `settings.conf` — but the line numbers are off by one. This slipped
past `check-refs.go` because the validator only enforces that line
numbers are within file bounds; it does not (and cannot, without a
deeper symbol index) check that the cited line actually contains what
the markdown claims it contains.

The Default-ports table entries on L1037 and L1041 also use the same
`settings.conf:138,989` / `:131,867` style refs — every conf-side line
ref in the Appendix-A table should be re-checked.

**Fix:** sweep `## Appendix A` for `settings.conf:NNN` refs and bump
each one by the correct delta (most likely +1 throughout — looks like
the table was authored against an earlier copy of `settings.conf`).
Re-run `./scripts/sp2-done-check.sh` after the patch.

**Impact if unfixed:** SP3 client authors who try to read the cited
line to confirm a port will land on the wrong row, briefly. Not a
correctness blocker; the env names and values are correct.

### I2. `discovered_by` field dropped from frontmatter

Spec §3.4 specifies frontmatter fields:
`upstream_commit`, `upstream_branch`, `discovered_at`, **`discovered_by`**.
`discovery.md` records the first three but omits `discovered_by`.
`discovery-method.md` does carry "Captured by: discovery sub-agent
fan-out" on line 13, which arguably substitutes — but the spec puts it
in the markdown frontmatter, and the JSON schema for the YAML doesn't
require it either. Minor consistency miss with the spec.

**Fix:** add `discovered_by: "<runner identity>"` to discovery.md
frontmatter. Optionally mirror to discovery.yaml.

---

## Minor / nits

### N1. Validator does not implement Risk-F drift self-test

Spec §8 Risk F mandates a self-test in `check-refs.go` that loads the
JSON schema, parses its `properties`, and asserts the validator's
checks correspond 1-to-1 with the schema's `required` and `enum`
fields. The current validator hard-codes `allowedIDs` and the required
field list as Go literals; if someone edits `discovery.schema.json`
they will not get a build-break.

**Fix (not blocking SP2):** add a test that reads
`docs/schema/discovery.schema.json`, extracts the `id` enum and the
required-fields list, and diffs against the Go `allowedIDs` map and
the struct tags. Cheap, ~25 LoC.

### N2. Sub-section heading inconsistency across surfaces

Surfaces use slightly different sub-headings:
- "Findings" (1 surface) vs "Findings table" (10 surfaces).
- "Gaps" (4 surfaces) vs "Gaps / ambiguities" (7 surfaces).
- "Implementation notes for SP3" (5 surfaces) vs
  "Implementation notes for SP3 / IBD-2 / PC-2" / "/ NEW-FR8" /
  "/ NEW-FR9" / "/ NEW-FR11" / "/ CLIENT-2" / "(OPS-3)".

The "/ XYZ-N" suffix is helpful (it ties each surface to the
build-doc requirement IDs) but is applied inconsistently — only 6 of
11 surfaces carry it. Either drop the suffix from all or add it to all
five missing ones (sections 1, 2, 3, 4, 6).

### N3. `docs/discovery-review.md` not produced (spec §5.6)

Spec §5.6 has `sp2-done-check.sh` `grep -q '^critical_findings: 0$'
docs/discovery-review.md`. The actual `sp2-done-check.sh` drops that
check entirely (replaced by `make build lint test`). Decision §1 in
the spec lists `discovery-review.md` as one of four output artefacts.

The substitute artefact is **this file** (`docs/superpowers/reviews/
2026-04-29-sp2-code-review.md`). That is consistent with how SP1's
review was filed, so the deviation appears intentional and beneficial
(reviews live under `docs/superpowers/reviews/` for the whole
project, not scattered into `docs/`). But the spec/plan should record
the change so the trace from spec → done-check is intact.

**Fix:** either rename this file to `docs/discovery-review.md` (matches
spec literally) or update spec §5.6 / Decision §1 to point at the
reviews dir.

### N4. Stray `report.html` and `report.json` in repo root

`report.html` (12K) and `report.json` (21K) exist in the repo root,
unstaged but tracked-by-existence. They appear to be SP1 artefacts.
Probably should be in `.gitignore` or relocated. Not an SP2 deliverable
but visible noise during this review.

---

## Strengths

- **Findings tables are dense and concrete.** Section 1 (JSON-RPC)
  enumerates every method with auth tier and `impl: bool`. Section 2
  (Asset) lists 45 routes. Section 5 (Metrics) enumerates the
  `teranode_<subsystem>_<name>` pattern with concrete metric names.
  Nothing reads as filler.
- **Negative findings are well-defended.** `testmempoolaccept` and
  `fee_estimation` notes explicitly name the search files, the
  `rpcHandlersBeforeInit` and `rpcUnimplemented` maps, the absence in
  protobuf (`validator_api.proto`), and the closest-but-not-equivalent
  internal API. A future reader can see exactly what was looked for.
- **Tri-state `partial` used precisely.** Mempool, P2P, double-spend
  carry `partial` with prose that distinguishes "what works" from
  "what doesn't" — not a fudge. The double-spend section's
  five-phase `ProcessConflicting` discussion (§11) is particularly
  good: it points to `stores/utxo/process_conflicting.go:50-175` and
  explains the winning-chain semantics with test refs.
- **No speculative language.** A grep for "seems to|likely is|likely
  uses|appears to|probably|may use|might be|could be|presumably"
  across `discovery.md` returns zero hits. The one occurrence of the
  word "likely" is in a section heading "which transaction is 'likely
  to confirm'?" — quoted, in scare-quotes, deliberately probing what
  the upstream code means.
- **Discrepancies surfaced rather than silently resolved.** All four
  required cross-agent conflicts are explicit, with both refs and a
  prescription for SP3 ("Conf value wins. … Clients must connect on
  `:8090`.").
- **Validator design is appropriately small.** ~200 lines of Go,
  three tests, no external schema-validation dependency. The
  decision to enforce `present: false ⇒ notes contains "search"` is
  a clever lightweight way to mechanise the "document your search
  method" rule.
- **Five commits land in plan order.** `7ae7954` (scaffold) →
  `0e3209d` (method note) → `a75c39d` (raw agent outputs) →
  `e07cf15` (synthesise md) → `ed24292` (populate yaml) →
  `4dcf294` (validator + verify wiring) → `e393ffb` (done-check).
  Clean, reviewable history.

---

## Spec coverage gaps

None substantive. All §4 surfaces and §5 artefacts are implemented
modulo the two minor deviations noted above (N3 `discovery-review.md`
location and §3.4 `discovered_by` field).

§5.5's Risk-F self-test (N1) is the only missing testable mechanism;
§5.3's JSON schema exists but is treated as documentation only — the
Go validator is the actual enforcement, which is what the spec
explicitly authorises.

---

## Recommendation

**APPROVE_WITH_MINOR.**

Tag `sp2-complete` and proceed to SP3. Capture the Important issues
(I1, I2) as a small follow-up commit on `main` before SP3's setup
script starts pulling refs from the YAML. The Minor issues can be
batched into routine maintenance.
