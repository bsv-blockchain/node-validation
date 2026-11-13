# Threshold provenance

This page states, for each acceptance check and each run-size setting, where its threshold comes from. The source plan is confidential (see [Source plan](../README.md#source-plan)), so this page never quotes the plan's values. It only states how each harness value relates to the plan.

| Provenance | Meaning |
|---|---|
| Plan | The plan states this threshold or condition, and the harness applies it as written. |
| Harness | The plan states no threshold for this check. The harness authors chose it. |
| Deviates | The plan states a threshold or condition, and the harness applies a different one. |
| n/a | The check is a precondition or a probe step, with no threshold. |

A `Deviates` row means the result is weaker evidence for that criterion than the plan asks for. Many deviations come from the regtest Docker stack: short runs, low rates and a single host. `config.example.yaml` holds the full-length run settings.

Check text is the description string in the code. `%d`, `%v` and `%s` are filled in at run time. Where a check has separate pass and fail branches, each branch has its own row.

| Test ID | File | Check | Provenance | Harness rationale |
|---|---|---|---|---|
| CLIENT-1 | tests/client1.go | Notification session stays alive across observation window | Deviates | No rationale recorded. |
| CLIENT-1 | tests/client1.go | All REST-seen blocks observed via subscription (≤2 misses tolerated for race conditions) | Deviates | Misses are tolerated to absorb race conditions between REST polling and the subscription. |
| CLIENT-1 | tests/client1.go | ≥40 of 50 target broadcasts succeeded (10 tolerance for short observation) | Harness | Tolerance allows for a short observation window. |
| CLIENT-1 | tests/client1.go | Reconnection simulated mid-observation | n/a | Disconnect pause shortened from 60s to 10s; still simulates a disconnect. |
| CLIENT-1 | tests/client1.go | ≥%d%% of broadcast txs eventually mined (80) | Deviates | No rationale recorded. |
| CLIENT-1 | config.docker.yaml | client1_observation: 5m | Harness | No rationale recorded. |
| CLIENT-1 | config.example.yaml | client1_observation: 1h | Harness | No rationale recorded. |
| CLIENT-2 | tests/client2.go | Teranode accepts extended-format transaction | Plan | No rationale recorded. |
| CLIENT-2 | tests/client2.go | Standard-format hex computed from extended source | n/a | No rationale recorded. |
| CLIENT-2 | tests/client2.go | Teranode accepts standard-format transaction (backward compat) | Plan | No rationale recorded. |
| CLIENT-3 | tests/client3.go | Reconnection (fresh NotificationClient) succeeded (pass branch) | Deviates | A fresh client is constructed after the disconnect, per an internal spec decision. |
| CLIENT-3 | tests/client3.go | Reconnection (fresh NotificationClient) succeeded (fail branch) | Deviates | A fresh client is constructed after the disconnect, per an internal spec decision. |
| CLIENT-3 | tests/client3.go | ≥99% of generated txs confirmed via REST (proxy for notification delivery) | Deviates | Teranode emits no per-tx events on Centrifuge, so REST tx fetch is used as a delivery proxy. |
| CLIENT-3 | tests/client3.go | Block heights arrive in non-decreasing order via subscription | Deviates | No rationale recorded. |
| CLIENT-3 | tests/client3.go | Notification mechanism documented (architectural finding) | n/a | Records the finding that per-tx events are absent; always passes. |
| CLIENT-3 | config.docker.yaml | client3_tx_count: 50 | Harness | No rationale recorded. |
| CLIENT-3 | config.example.yaml | client3_tx_count: 500 | Harness | No rationale recorded. |
| IBD-2 | tests/ibd2.go | ≥10 IBD-2 fixtures present (harness minimum) | Harness | No rationale recorded. |
| IBD-2 | tests/ibd2.go | 100% match on accept/reject decisions across all fixtures | Deviates | Implementations legitimately reject for different reasons, so only the decision is compared. |
| INTER-1 | tests/inter1.go | Transient divergence ≤33% of polling rounds during observe phase | Deviates | Block propagation under ARM emulation causes ~25-30% transient divergence; a persistent fork would exceed 50%. |
| INTER-1 | tests/inter1.go | Reorg induction completed and both nodes converged | Harness | No rationale recorded. |
| INTER-1 | tests/inter1.go | Comparable orphan rate (within 2×) during reorg phase (pass branch) | Deviates | Inferred from the induced reorg (one orphan per side), not measured. |
| INTER-1 | tests/inter1.go | Blocks from both implementations accepted with comparable frequency (pass branch) | Deviates | Inferred from the induced reorg (each side's block propagated), not measured. |
| INTER-1 | tests/inter1.go | Comparable orphan rate (within 2×) during reorg phase (fail branch) | Deviates | Fails only when the reorg did not complete. |
| INTER-1 | tests/inter1.go | Blocks from both implementations accepted with comparable frequency (fail branch) | Deviates | Fails only when the reorg did not complete. |
| INTER-1 | config.docker.yaml | inter1_observation: 20m | Deviates | No rationale recorded. |
| INTER-1 | config.example.yaml | inter1_observation: 336h | Plan | No rationale recorded. |
| INTER-2 | tests/inter2.go | Splitter tx with %d outputs accepted | n/a | No rationale recorded. |
| INTER-2 | tests/inter2.go | Build tx %d (fee=%d outs=%d) | n/a | No rationale recorded. |
| INTER-2 | tests/inter2.go | ≥99%% of Teranode-only txs reach SV Node within %v | Plan | No rationale recorded. |
| INTER-2 | tests/inter2.go | ≥99%% of SV-Node-only txs reach Teranode within %v | Plan | No rationale recorded. |
| INTER-2 | tests/inter2.go | Both-group: ≥99% of txs accepted by at least one backend | Harness | No rationale recorded. |
| INTER-2 | config.docker.yaml | inter2_tx_count: 100 | Deviates | No rationale recorded. |
| INTER-2 | config.example.yaml | inter2_tx_count: 1000 | Plan | No rationale recorded. |
| INTER-2 | config.docker.yaml | default_propagation: 10s | Plan | No rationale recorded. |
| INTER-2 | config.example.yaml | default_propagation: 10s | Plan | No rationale recorded. |
| NEW-FR10 | tests/new_fr10.go | tx-by-id p95 ≤ %v | Deviates | No rationale recorded. |
| NEW-FR10 | tests/new_fr10.go | block-by-hash p95 ≤ %v | Deviates | No rationale recorded. |
| NEW-FR10 | tests/new_fr10.go | block-by-height p95 ≤ %v (via /search) | Deviates | No rationale recorded. |
| NEW-FR10 | tests/new_fr10.go | Address-history queries supported with pagination | Deviates | No address route in the build under test; non-required until the route ships. |
| NEW-FR10 | config.docker.yaml | fr10_latency_target_ms: 100 | Plan | No rationale recorded. |
| NEW-FR10 | config.example.yaml | fr10_latency_target_ms: 100 | Plan | No rationale recorded. |
| NEW-FR11 | tests/new_fr11.go | getrawmempool returns []string | n/a | No rationale recorded. |
| NEW-FR11 | tests/new_fr11.go | Chain tx %d (%s…) in getrawmempool | n/a | Waits briefly because Teranode registers txs in getrawmempool asynchronously. |
| NEW-FR11 | tests/new_fr11.go | %s: per SP2 discovery absent or unimplemented | Deviates | Discovery found these mempool RPCs absent, so the test asserts the negative. |
| NEW-FR7 | tests/new_fr7.go | Chain link %d (depth %d) accepted by Teranode | n/a | No rationale recorded. |
| NEW-FR7 | tests/new_fr7.go | Full chain of depth %d accepted by Teranode mempool | Plan | No rationale recorded. |
| NEW-FR7 | tests/new_fr7.go | All %d chain txs visible in SV Node mempool within %v | Harness | Waits briefly for P2P propagation before checking the SV Node mempool. |
| NEW-FR7 | tests/new_fr7.go | Teranode tip reached mined block within 30s | Harness | No rationale recorded. |
| NEW-FR7 | tests/new_fr7.go | All %d chain txs mined into a single block (no intermediate confirmations) | Plan | No rationale recorded. |
| NEW-FR7 | config.docker.yaml | fr7_chain_depth: 25 | Plan | No rationale recorded. |
| NEW-FR7 | config.example.yaml | fr7_chain_depth: 25 | Plan | No rationale recorded. |
| NEW-FR8 | tests/new_fr8.go | estimatefee returned a non-unimplemented error | n/a | Any error other than unimplemented is recorded as a failure. |
| NEW-FR8 | tests/new_fr8.go | estimatefee returned a value (unexpected per SP2 discovery) | n/a | Discovery expected the endpoint to be unimplemented. |
| NEW-FR8 | tests/new_fr8.go | Multiple priority levels supported (economy/standard/priority) | Deviates | Only one priority level exists per discovery, so multi-tier accuracy cannot be verified. |
| NEW-FR8 | config.docker.yaml | fr8_priority_levels: [economy, standard, priority] | Plan | No rationale recorded. |
| NEW-FR8 | config.example.yaml | fr8_priority_levels: [economy, standard, priority] | Plan | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | tx1 (first-seen) accepted by Teranode RPC | n/a | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | tx2 (conflicting) rejected synchronously by Teranode RPC | Plan | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | Notification on /p2p-ws within 5s carrying tx2's txid | Deviates | Notification is best-effort and depends on internal scheduling, so it is non-required. |
| NEW-FR9 | tests/new_fr9.go | Mine confirms tx1 | n/a | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | Fetch mined block to verify winner | n/a | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | tx1 (winner) is in the mined block | Harness | No rationale recorded. |
| NEW-FR9 | tests/new_fr9.go | Low-confirmation double-spend handled (FR-9 criterion 3 part 2) | Deviates | Regtest mining cadence makes this hard to test deterministically; non-required. |
| NEW-FR9 | tests/new_fr9.go | Clear indication of which transaction is likely to confirm | Deviates | Derived from the synchronous RPC error on tx2; always passes. |
| NEW-NFR11 | tests/new_nfr11.go | [%s] URL parses | n/a | No rationale recorded. |
| NEW-NFR11 | tests/new_nfr11.go | [%s] TLS handshake succeeded with version >= 1.2 | Plan | No rationale recorded. |
| NEW-NFR11 | tests/new_nfr11.go | [%s] transport scheme is %q | Deviates | Regtest uses plain transport; production must terminate TLS in front. |
| NEW-NFR11 | tests/new_nfr11.go | Construct unauthenticated client for auth probe | n/a | No rationale recorded. |
| NEW-NFR11 | tests/new_nfr11.go | Teranode RPC rejects unauthenticated requests with 401 | Deviates | Teranode RPC requires Basic Auth. |
| NEW-NFR11 | tests/new_nfr11.go | Teranode RPC accepts authenticated requests | n/a | No rationale recorded. |
| NEW-NFR13 | tests/new_nfr13.go | Rate-limit probe completed without hitting a limit | Deviates | No rationale recorded. |
| NEW-NFR13 | tests/new_nfr13.go | Rate limit observed during probe | Deviates | No rationale recorded. |
| NEW-NFR13 | tests/new_nfr13.go | Service resumes after brief wait | Harness | No rationale recorded. |
| NEW-NFR13 | config.docker.yaml | nfr13_max_probe_rate: 1000 | Deviates | Maximum read-probe rate in req/s; 0 disables the probe. |
| NEW-NFR13 | config.example.yaml | nfr13_max_probe_rate: 1000 | Deviates | Maximum read-probe rate in req/s; 0 disables the probe. |
| NEW-NFR13 | config.docker.yaml | nfr13_probe_duration: 5s | Harness | Probe duration; 0 disables the probe. |
| NEW-NFR13 | config.example.yaml | nfr13_probe_duration: 5s | Harness | Probe duration; 0 disables the probe. |
| NEW-NFR7 | tests/new_nfr7.go | getblockhash @%d returns identical result across %d iterations | Plan | No rationale recorded. |
| NEW-NFR7 | tests/new_nfr7.go | getblock(verbosity=0) returns byte-identical hex across %d iterations | Plan | No rationale recorded. |
| NEW-NFR7 | tests/new_nfr7.go | getbestblockhash returns well-formed hash across %d iterations | Harness | No rationale recorded. |
| NEW-NFR7 | tests/new_nfr7.go | Read ops return identical results under 100 TPS load | Deviates | Not yet integrated with the PERF-1 TPS ramp; non-required. |
| NEW-NFR7 | tests/new_nfr7.go | Read ops return identical results under 500 TPS load | Deviates | Not yet integrated with the PERF-1 TPS ramp; non-required. |
| NEW-NFR7 | config.docker.yaml | new_nfr7_iterations: 50 | Harness | No rationale recorded. |
| NEW-NFR7 | config.example.yaml | new_nfr7_iterations: 100 | Harness | No rationale recorded. |
| OPS-1 | internal/chaos/ops1.go | Crashed node is down (teranode-1 not running, RPC unreachable) | n/a | No rationale recorded. |
| OPS-1 | internal/chaos/ops1.go | Surviving mesh mined new blocks while teranode-1 was down | n/a | No rationale recorded. |
| OPS-1 | internal/chaos/ops1.go | Surviving mesh kept operating: 5 nodes advanced and converged while node down | Plan | No rationale recorded. |
| OPS-1 | internal/chaos/ops1.go | Recovered node RPC came back up | Deviates | A generous recovery budget is kept as headroom for a rare cold-start wedge. |
| OPS-1 | internal/chaos/ops1.go | Crashed node restarted and RPC recovered | Deviates | A generous recovery budget is kept as headroom for a rare cold-start wedge. |
| OPS-1 | internal/chaos/ops1.go | Recovered node re-converged to the mesh tip | Deviates | A killed all-in-one Teranode is slow to restart, re-handshake and re-sync. |
| OPS-1 | internal/chaos/ops1.go | Recovered node re-converged to the mesh tip (re-synced missed blocks) | Plan | No rationale recorded. |
| OPS-1 | internal/chaos/ops1.go | Full mesh (6 nodes) converged after recovery | Harness | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Partition established: svnode-3 detached from tng-net | Deviates | Teranodes are never disconnected; the partition isolates one SV node. |
| OPS-2 | internal/chaos/ops2.go | Side B (svnode-3) mined an independent block | Deviates | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Side A (svnode-1) mined a longer competing chain | Deviates | Side A mines more blocks so its chain is strictly longer. |
| OPS-2 | internal/chaos/ops2.go | Side A (5 nodes incl. all Teranodes) converged on the longer chain | Harness | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Side B (svnode-3) diverged: isolated on its own shorter chain | n/a | Verifies that the isolated node actually diverged. |
| OPS-2 | internal/chaos/ops2.go | Competing chains: side A strictly longer than side B | n/a | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Partition healed: svnode-3 reconnected | n/a | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Partition healed: svnode-3 restarted | n/a | A restart forces a fresh P2P handshake. |
| OPS-2 | internal/chaos/ops2.go | Partition healed: svnode-3 reconnected to tng-net | n/a | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | All 6 nodes reconverged on a single tip after heal | Harness | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Mesh converged on the longer chain (chain A) via reorg | Plan | No rationale recorded. |
| OPS-2 | internal/chaos/ops2.go | Minority chain reorged away (svnode-3 dropped its competing block) | Plan | No rationale recorded. |
| OPS-3 | tests/ops3.go | Metrics endpoint returns 200 with parseable Prometheus body | Plan | No rationale recorded. |
| OPS-3 | tests/ops3.go | Metric %q present (%s) | Plan | No rationale recorded. |
| OPS-3 | tests/ops3.go | Health endpoint returns 200 with JSON-parseable body | Plan | No rationale recorded. |
| PC-1 | tests/pc1.go | Transient divergence in accepted/rejected blocks ≤33% of polling rounds | Deviates | Block propagation under ARM emulation causes ~25-30% transient divergence; a persistent fork would exceed 50%. |
| PC-1 | tests/pc1.go | ≥95% match in tx validity decisions across all batches | Deviates | Covers races where one node validates a tx before its parent reaches the other node. |
| PC-1 | tests/pc1.go | Both nodes converge to same tip within %v of reorg | Deviates | No rationale recorded. |
| PC-1 | config.docker.yaml | pc1_observation: 30m | Deviates | No rationale recorded. |
| PC-1 | config.example.yaml | pc1_observation: 168h | Plan | No rationale recorded. |
| PC-2 | tests/pc2.go | ≥30 PC-2 fixtures present (harness minimum) | Harness | No rationale recorded. |
| PC-2 | tests/pc2.go | 100% match on accept/reject decisions across all fixtures | Deviates | Implementations legitimately reject for different reasons, so only the decision is compared. |
| PC-3 | tests/pc3.go | All 3 test txs propagated to svnode-1 mempool (fail branch) | Harness | Waits for relay so that svnode-1 does not mine an empty block first; non-required. |
| PC-3 | tests/pc3.go | All 3 test txs propagated to svnode-1 mempool (pass branch) | Harness | Waits for relay so that svnode-1 does not mine an empty block first; non-required. |
| PC-3 | tests/pc3.go | Block mined via svnode-1 | n/a | No rationale recorded. |
| PC-3 | tests/pc3.go | Teranode tip reached mined block within 60s | Harness | No rationale recorded. |
| PC-3 | tests/pc3.go | Block bytes fetched via Teranode REST | n/a | No rationale recorded. |
| PC-3 | tests/pc3.go | Block parses with standard libsv parser | Plan | A failure of the standard parser is treated as a finding. |
| PC-3 | tests/pc3.go | Block contains %s test tx (%s…) | Plan | No rationale recorded. |
| PC-3 | tests/pc3.go | Teranode accepts %s tx via sendrawtransaction | Plan | No rationale recorded. |
| PC-3 | tests/pc3.go | Returned %s txid matches locally-computed | Plan | No rationale recorded. |
| PC-3 | tests/pc3.go | Teranode REST returns %s tx body | n/a | No rationale recorded. |
| PC-3 | tests/pc3.go | %s tx re-parses with matching txid | Plan | No rationale recorded. |
| PERF-1 | tests/perf1.go | Latency measured per rate (SV Node baseline comparison deferred) | Deviates | No baseline run exists, so the measurement is recorded as a soft pass. |
| PERF-1 | tests/perf1.go | p95 at highest tested rate ≤ 5× p95 at 100 TPS | Harness | No rationale recorded. |
| PERF-1 | tests/perf1.go | Resource usage sampled from metrics endpoint (pass branch) | Deviates | No rationale recorded. |
| PERF-1 | tests/perf1.go | Resource usage sampled from metrics endpoint (fail branch) | Deviates | Fails only when no metrics client is configured. |
| PERF-1 | config.docker.yaml | perf1_max_tps: 100 | Deviates | No rationale recorded. |
| PERF-1 | config.example.yaml | perf1_max_tps: 1000 | Plan | `perf1_max_tps` only filters the ramp steps. With the listed steps, the run never exceeds 250 TPS. |
| PERF-1 | config.docker.yaml | perf1_ramp_steps: [10, 50, 100, 250] | Deviates | No rationale recorded. |
| PERF-1 | config.example.yaml | perf1_ramp_steps: [10, 50, 100, 250] | Deviates | Steps top out at 250 TPS. `perf1_max_tps` only filters this list, so the run never exceeds 250 TPS. Add steps to extend. |
| PERF-1 | config.docker.yaml | perf1_per_rate: 30s | Harness | No rationale recorded. |
| PERF-1 | config.example.yaml | perf1_per_rate: 5m | Harness | No rationale recorded. |
