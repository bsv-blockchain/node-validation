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
