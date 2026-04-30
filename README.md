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

## Functional Requirements

| ID | Title | Coverage | Covered by | Notes |
|---|---|---|---|---|
| FR-1 | Full Bitcoin SV Protocol Compliance | `AUTOMATED` | PC-1, PC-2 |  |
| FR-2 | Consistent Transaction and Block Formats | `AUTOMATED` | PC-3, CLIENT-2 |  |
| FR-3 | Script Interpreter Parity | `AUTOMATED` | PC-2, IBD-2 |  |
| FR-4 | Historical Chain Validation Evidence | `DOCUMENTATION_REVIEW` |  | IBD-1 in source plan; not an automated test. Reviewer override required. |
| FR-5 | Reliable Transaction Propagation | `AUTOMATED` | INTER-2, CLIENT-1 |  |
| FR-6 | Block and Transaction Notification Reliability | `AUTOMATED` | CLIENT-1, CLIENT-3 |  |
| FR-7 | Support for Unconfirmed Transaction Chains | `AUTOMATED` | NEW-FR7 |  |
| FR-8 | Transaction Fee Estimation | `AUTOMATED` | NEW-FR8 |  |
| FR-9 | Double-Spend Detection and Notification | `AUTOMATED` | NEW-FR9 |  |
| FR-10 | Historical Data Access | `AUTOMATED` | NEW-FR10 |  |
| FR-11 | Mempool Query and Filtering | `AUTOMATED` | NEW-FR11 |  |

## Non-Functional Requirements

| ID | Title | Coverage | Covered by | Notes |
|---|---|---|---|---|
| NFR-1 | Upstream Availability Guarantees | `LONG_TERM_OBSERVATION` |  | 30-day window required; runner records observed uptime as supporting metric only. |
| NFR-2 | Fault Tolerance and Recovery | `PRIVILEGED_ACCESS_REQUIRED` |  | OPS-1 excluded — requires admin access. |
| NFR-3 | Throughput and Latency Performance | `AUTOMATED` | PERF-1 |  |
| NFR-4 | Translation and Gateway Overhead | `PRIVILEGED_ACCESS_REQUIRED` |  | PERF-3 excluded — requires bypass of Teranode gateway. |
| NFR-5 | IPv4/IPv6 and Real-World Internet Compat. | `PARTIAL` | INTER-1, CLIENT-1 | AUTOMATED for IPv4 (runner self-hosts on IPv4). IPv6-only path not exercised. |
| NFR-6 | Interoperability Across Implementations | `AUTOMATED` | INTER-1, INTER-2 |  |
| NFR-7 | Deterministic Behavior Under Load | `AUTOMATED` | NEW-NFR7 |  |
| NFR-8 | API Stability and Versioning | `CONTRACTUAL` |  | BSVA documentation review required. |
| NFR-9 | API Pricing and Access Model | `CONTRACTUAL` |  | BSVA pricing documentation review required. |
| NFR-10 | Observability and Diagnostics | `AUTOMATED` | OPS-3 |  |
| NFR-11 | Security and Authentication | `AUTOMATED` | NEW-NFR11 |  |
| NFR-12 | Data Consistency Guarantees | `PARTIAL` | PC-1 | PC-1 covers cross-node consistency. OPS-2 excluded for reorg/partition scenarios. |
| NFR-13 | Rate Limiting and Throttling | `AUTOMATED` | NEW-NFR13 |  |

## Test Environment

| ID | Title | Notes |
|---|---|---|
| TE-1 | Testnet Deployment | EXCLUDED_SETUP — pre-existing infrastructure expected; runner only validates connectivity. |
| TE-2 | Client Integration Sandbox | EXCLUDED_SETUP — this project IS the integration sandbox. |
| TE-3 | Private Test Network | EXCLUDED_SETUP — optional setup, not used by any in-scope test. |

## Source-plan test cases

| ID | Title | Severity | Status | Notes |
|---|---|---|---|---|
| PC-1 | Parallel Node Comparison | critical | `IN_SCOPE` |  |
| PC-2 | Historical Script and Consensus Regression | critical | `IN_SCOPE` |  |
| PC-3 | Message Format and Wire Protocol Verification | critical | `IN_SCOPE` | Format scope only; raw P2P capture out of scope. |
| IBD-1 | Historical Validation Evidence Review | critical | `EXCLUDED_DOCUMENTATION` | Source plan itself notes this is documentation review, not testing. |
| IBD-2 | Historical UTXO Spend Verification | critical | `IN_SCOPE` |  |
| PERF-1 | Throughput and Latency Baseline | important | `IN_SCOPE` |  |
| PERF-2 | Microservices Horizontal Scaling | important | `EXCLUDED_PRIVILEGED` | Requires admin access to scale Teranode replicas. |
| PERF-3 | Gateway and Translation Overhead | important | `EXCLUDED_PRIVILEGED` | Requires bypass of Teranode P2P gateway. |
| INTER-1 | Mixed-Network Consensus | critical | `IN_SCOPE` |  |
| INTER-2 | Cross-Implementation Transaction Propagation | critical | `IN_SCOPE` |  |
| CLIENT-1 | TNG P2P Client Functional Tests | critical | `IN_SCOPE` |  |
| CLIENT-2 | Extended Transaction Format Support | important | `IN_SCOPE` | Skips at runtime if no extended format advertised. |
| CLIENT-3 | Notification Stream Reliability | critical | `IN_SCOPE` |  |
| OPS-1 | Service Failure and Recovery | important | `EXCLUDED_PRIVILEGED` | Requires killing internal microservices. |
| OPS-2 | Network Partition and Reorg Convergence | important | `EXCLUDED_PRIVILEGED` | Requires controlling the entire test network. |
| OPS-3 | Observability and Monitoring | important | `IN_SCOPE` | Probe scope only; integration with TNG monitoring out of scope. |

## New test cases

| ID | Title | Severity | Satisfies |
|---|---|---|---|
| NEW-FR7 | Unconfirmed Transaction Chain Acceptance | advisory | FR-7 |
| NEW-FR8 | Fee Estimation Endpoint Validation | advisory | FR-8 |
| NEW-FR9 | Double-Spend Detection Behaviour | advisory | FR-9 |
| NEW-FR10 | Historical Data Access Latency | advisory | FR-10 |
| NEW-FR11 | Mempool Query Capabilities | advisory | FR-11 |
| NEW-NFR7 | Deterministic Behaviour Under Repeated Operations | advisory | NFR-7 |
| NEW-NFR11 | Transport Security and Authentication Probe | advisory | NFR-11 |
| NEW-NFR13 | Rate Limit Discovery and Error Semantics | advisory | NFR-13 |

## Risks

| ID | Description | Mitigating tests | Notes |
|---|---|---|---|
| R1 | Transaction loss / inconsistent mempools (changed propagation, overlays) | INTER-2, CLIENT-1, CLIENT-3, NEW-FR7 |  |
| R2 | Protocol fragmentation / forks (message formats, translation, interop gaps) | PC-1, PC-3, INTER-1, INTER-2, CLIENT-2 |  |
| R3 | Undetected consensus bugs (script interpreter parity, historical flags) | PC-1, PC-2, IBD-2 |  |
| R4 | Undetected consensus bugs (incomplete historical validation) | IBD-1, IBD-2 | IBD-1 documentation review required for full mitigation. |
| R5 | Excessive operational cost / API fees | PERF-1 | Cost side is CONTRACTUAL via NFR-9. |
| R6 | Underdocumented architecture (txn trees, microservices, overlays) | OPS-3, CLIENT-1, CLIENT-3, NEW-FR11 | PERF-2 / OPS-1 needed for full mitigation. |
| R7 | Real-world IPv4/IPv6 connectivity and partition behaviour | INTER-1, CLIENT-1 | OPS-2 needed for partition behaviour. |

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
