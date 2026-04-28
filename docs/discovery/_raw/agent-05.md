## 5. Metrics endpoint

### Summary

Teranode exposes a single Prometheus-format metrics endpoint. Path is configured via `prometheusEndpoint` (default `/metrics`, `settings.conf:965`). Port depends on whether `profilerAddr` is also configured:

| Scenario | Port / address | Source ref |
|---|---|---|
| `profilerAddr` set | Same HTTP server as pprof; default `:9091` | `daemon/daemon_services.go:161-198` |
| `profilerAddr` empty | `http.DefaultServeMux` — port depends on first `ListenAndServe` call (undocumented) | `daemon/daemon_services.go:201-213` |

In standard `settings.conf`: `profilerAddr = :9091`, `prometheusEndpoint = /metrics`. Canonical address: **`:9091/metrics`**. Docker-host adds `PORT_PREFIX` digit prefix.

**Exposition format:** Prometheus text (OpenMetrics-compatible) via `prometheus/client_golang/prometheus/promhttp` v1.23.2 (`go.mod:47`). OpenTelemetry tracing wired separately (`util/tracing/tracing.go:11-16`); OTel metrics not bridged.

### Findings table

All metric names use pattern `teranode_<subsystem>_<name>`.

| Category | Metric names | Type | Source ref |
|---|---|---|---|
| Chain tip / block height | `teranode_blockassembly_best_block_height`, `teranode_blockassembly_current_block_height` | Gauge | `services/blockassembly/metrics.go:255-271` |
| Sync status (FSM) | `teranode_blockchain_fsm_current_state` (numeric) | Gauge | `services/blockchain/metrics.go:393-400` |
| Sync status (catchup) | `teranode_blockvalidation_catchup_active`, `teranode_blockvalidation_catchup_duration_seconds`, `teranode_blockvalidation_catchup_blocks_fetched_total{peer_id}`, `teranode_blockvalidation_catchup_headers_fetched_total{peer_id}` | Gauge / Histogram / Counter | `services/blockvalidation/metrics.go:214-262` |
| Mempool / in-flight transactions | `teranode_blockassembly_transactions`, `teranode_blockassembly_queued_transactions` | Gauge | `services/blockassembly/metrics.go:179-195` |
| Tx throughput (validator) | `teranode_validator_transactions`, `teranode_validator_transactions_validate_total`, `teranode_validator_invalid_transactions` | Histogram / Counter | `services/validator/metrics.go:139-244` |
| Tx throughput (propagation) | `teranode_propagation_transactions`, `teranode_propagation_transactions_batch`, `teranode_propagation_invalid_transactions` | Histogram / Counter | `services/propagation/metrics.go:80-132` |
| Block validation latency | `teranode_blockvalidation_validate_block`, `teranode_blockvalidation_process_block_found`, `teranode_blockvalidation_block_found`, `teranode_block_valid`, `teranode_blockpersister_persist_block` | Histogram | `services/blockvalidation/metrics.go:117-164`; `model/metrics.go:45-53`; `services/blockpersister/metrics.go:111-119` |
| Fork tracking | `teranode_blockvalidation_fork_count`, `_fork_processing_workers`, `_fork_average_depth`, `_fork_longest_depth`, `_fork_average_lifetime_seconds`, `_fork_created_total{reason}`, `_fork_resolved_total{result}`, `_fork_orphaned_total`, `_fork_lifetime_seconds`, `_fork_depth_blocks`, `_fork_resolution_depth_blocks` | Gauge / Counter / Histogram | `services/blockvalidation/metrics.go:296-413` |
| Priority queue | `teranode_blockvalidation_priority_queue_size{priority}`, `_added_total{priority}`, `_processed_total{priority,result}` | Gauge / Counter | `services/blockvalidation/metrics.go:265-293` |
| gRPC | `grpc_server_*` family | Histogram / Counter | `util/grpc_helper.go:267-273` |
| Peer handler (P2P/Legacy) | `teranode_<service>_peer_*_count{peer}`, `_peer_block_processing_ms{peer}` | Counter | `util/tracing/peer_handler_collector.go:53-93` |
| Asset API | `teranode_asset_http_get_transaction{function,operation}`, `_get_block_header`, `_get_best_block_header`, `_get_block`, `_get_last_n_blocks`, `_get_utxo`, `_get_merkle_proof` | Counter | `services/asset/httpimpl/metrics.go:87-229` |
| UTXO store (Aerospike) | `teranode_<subsystem>_utxo_*` (batch create/spend) | Counter / Histogram | `stores/utxo/aerospike/metrics.go:65-80` |
| Subtree validation | `teranode_subtreevalidation_validate_subtree`, `_check_subtree`, `_validate_subtree_retry` | Histogram / Counter | `services/subtreevalidation/metrics.go:96-204` |
| Block assembly ops | `teranode_blockassembly_add_tx`, `_remove_tx`, `_submit_mining_solution`, `_reorg`, `_reorg_duration` | Histogram / Counter | `services/blockassembly/metrics.go:74-291` |
| Blockchain service ops (35+) | `teranode_blockchain_add_block`, `_get_get_best_block_header`, `_get_chain_tips`, `_get_block_is_mined`, etc. | Histogram | `services/blockchain/metrics.go:78-431` |
| RPC service | `teranode_rpc_get_block`, `_get_best_block_hash`, `_send_raw_transaction`, `_get_rawmempool`, `_get_blockchain_info`, `_get_chaintips`, etc. | Histogram | `services/rpc/metrics.go:87-349` |

**Coverage of OPS-3 acceptance categories:**

| Category | Direct metric | Notes |
|---|---|---|
| Chain tip height | `teranode_blockassembly_best_block_height` | Block-assembly only; no dedicated blockchain-service gauge |
| Sync status | `_fsm_current_state` + `_catchup_active` | FSM state numeric; mapping not in metric labels |
| Mempool size | `_blockassembly_transactions` + `_queued_transactions` | No metric named `mempool_size` |
| Tx throughput | `_validator_transactions`, `_propagation_transactions` | Rate derived from `_count` |
| Block validation latency | `_blockvalidation_validate_block` (seconds) | Full coverage |

### Source references

| File | Relevance |
|---|---|
| `settings/settings.go:55,53` | `PrometheusEndpoint`, `ProfilerAddr` fields |
| `settings.conf:961,965` | `profilerAddr=:9091`, `prometheusEndpoint=/metrics` |
| `daemon/daemon_services.go:152-213` | `startProfilerAndMetrics()` mounts handler |
| `daemon/daemon.go:304` | `util.RegisterPrometheusMetrics()` for gRPC |
| `util/grpc_helper.go:267-273` | gRPC Prometheus integration |
| `services/blockchain/metrics.go` | 35+ blockchain operation histograms; `fsm_current_state` gauge |
| `services/blockvalidation/metrics.go` | Catchup, fork, priority-queue metrics |
| `services/blockassembly/metrics.go` | Block-height gauges, tx-count gauges |
| `services/validator/metrics.go` | Validation throughput / latency |
| `services/propagation/metrics.go` | Propagation metrics |
| `services/blockpersister/metrics.go` | Persist duration |
| `services/subtreevalidation/metrics.go` | Subtree validation latency |
| `services/legacy/metrics.go` | Peer message handler histograms |
| `services/rpc/metrics.go` | RPC command latency histograms |
| `services/asset/httpimpl/metrics.go` | Asset HTTP API counters |
| `stores/utxo/aerospike/metrics.go` | UTXO store metrics |
| `util/tracing/peer_handler_collector.go` | Per-peer counter collector |
| `go.mod:47` | `prometheus/client_golang v1.23.2` |

### Gaps / ambiguities

1. Chain-tip height is only on the block-assembly subsystem — no `teranode_blockchain_block_height` gauge.
2. FSM state is a raw integer; mapping not in metric labels.
3. No canonical `mempool_size`. Use `_blockassembly_transactions` + `_queued_transactions`.
4. When `profilerAddr` is empty, `/metrics` lands on `http.DefaultServeMux` — port undocumented; in practice `profilerAddr` is always set.
5. OTel tracing co-exists with Prometheus but is independent.
6. Kafka consumer metrics (`util/kafka/metrics.go`) registered globally; appear on same `/metrics` endpoint.

### Implementation notes for SP3 (OPS-3)

- **URL pattern:** `http://<host>:9091/metrics`. Docker-host: prefix port (e.g. `19091`).
- **Verification:** GET returns `200 OK`, `Content-Type: text/plain; version=0.0.4` or `application/openmetrics-text`. Look for any `teranode_` prefix.
- **Chain tip height assertion:** read `teranode_blockassembly_best_block_height`; non-zero and advancing.
- **Sync status assertion:** `teranode_blockchain_fsm_current_state` stable after sync; `teranode_blockvalidation_catchup_active == 0`.
- **Mempool assertion:** `teranode_blockassembly_transactions >= 0`.
- **Block validation latency assertion:** `_validate_block_sum / _validate_block_count` = avg latency.
- **`prometheusEndpoint` must not be empty** in settings — validate before testing.
- gRPC metrics also present after `util.RegisterPrometheusMetrics()` (`daemon/daemon.go:304`).

## 6. Health endpoint

### Summary

Teranode exposes a dedicated HTTP health server on a separate port from metrics.

| Property | Value | Source ref |
|---|---|---|
| Port | Default `:8000` (`HEALTH_CHECK_PORT`) | `settings.conf:126,603` |
| Paths | `/health`, `/health/readiness`, `/health/liveness` | `daemon/daemon.go:325-327` |
| Response format | JSON body, UTF-8 | `util/servicemanager/service_manager.go:288-318` |
| Status code | `200` if healthy; `503` if any dependency fails | `util/servicemanager/service_manager.go:292-308` |
| Content-Type | `text/plain; charset=utf-8` (Go default; not explicitly set) | `daemon/daemon.go:320-323` |

**Liveness vs readiness:**

- `/health/liveness` — `Health(ctx, checkLiveness=true)` — services self-check only. `blockvalidation.Health` returns `200 OK` unconditionally (`services/blockvalidation/Server.go:327-331`).
- `/health/readiness` and `/health` — `Health(ctx, checkLiveness=false)` — services check gRPC self-conn, Kafka, downstream stores.

A handler also registers `GET /health` on the default mux returning a deprecation message (`daemon/daemon.go:330-334`).

### Findings table

**Top-level envelope:**

| Field | JSON key | Type | Meaning | Source ref |
|---|---|---|---|---|
| Aggregate status | `status` | string (numeric) | Mirrors HTTP status, `"200"` or `"503"` | `util/servicemanager/service_manager.go:308` |
| Service list | `services` | array | One entry per registered service | `util/servicemanager/service_manager.go:303-308` |

**Per-service entry:**

| Field | JSON key | Type | Meaning |
|---|---|---|---|
| Service name | `service` | string | E.g. `"Blockchain"`, `"BlockValidation"`, `"Validator"` |
| Status | `status` | string (numeric) | `"200"`/`"503"` |
| Dependencies | `dependencies` | array or string | Inner check results |

**Per-dependency entry** (from `health.CheckAll`):

| JSON key | Type | Meaning |
|---|---|---|
| `resource` | string | E.g. `"gRPC Server"`, `"Kafka"`, `"BlockchainClient"`, `"FSM"`, `"SubtreeStore"`, `"TxStore"`, `"UTXOStore"`, `"CatchupStatus"` |
| `status` | string | `"200"`/`"503"` |
| `error` | string or `"<nil>"` | Error message |
| `message` | string | Human-readable description |
| `dependencies` | array | Nested if check returns JSON object |

**Known dependency names:**

| Service | Readiness dependencies | Source ref |
|---|---|---|
| Blockchain | `gRPC Server`, `HTTP Server`, `Kafka`, `BlockchainStore` | `services/blockchain/Server.go:243-280` |
| BlockValidation | `gRPC Server`, `Kafka`, `BlockchainClient`, `FSM`, `SubtreeStore`, `TxStore`, `UTXOStore`, `CatchupStatus` | `services/blockvalidation/Server.go:343-416` |

**`CatchupStatus` message format** (string, not JSON):
```
active=<bool>, last_time=<RFC3339|"never">, last_success=<bool>, attempts=<int>, successes=<int>, rate=<float>
```
Source: `services/blockvalidation/Server.go:381-413`.

### Source references

| File | Relevance |
|---|---|
| `daemon/daemon.go:306-363` | Health server: mux, routes, `http.Server`, `Serve()` |
| `daemon/daemon.go:325-327` | Routes: `/health`, `/health/readiness`, `/health/liveness` |
| `util/servicemanager/service_manager.go:288-318` | `HealthHandler()` aggregates all services |
| `util/servicemanager/service.go:10` | `Service.Health(ctx, checkLiveness bool)` |
| `util/health/health.go:17-41` | `CheckAll()` JSON serialisation |
| `util/health/http_health.go:18-51` | `CheckHTTPServer()` |
| `util/health/grpc_health.go:23-52` | `CheckGRPCServerWithSettings()` |
| `services/blockchain/Server.go:222-281` | `Blockchain.Health()` |
| `services/blockvalidation/Server.go:326-418` | `Server.Health()` + `CatchupStatus` |
| `settings/interface.go:37`; `settings/settings.go:56` | `HealthCheckHTTPListenAddress` defaults `:8000` |
| `settings.conf:126,603-609` | `HEALTH_CHECK_PORT=8000` |
| `test/utils/helper.go:1222-1234` | `WaitForHealthLiveness()` polls `/health/readiness` |

### Gaps / ambiguities

1. **Content-Type not explicitly set** — Go default `text/plain; charset=utf-8` even though body is JSON. Strict JSON clients must not assert content type.
2. **Liveness behaviour service-dependent** — most services unconditionally `200 OK` for liveness; semantics not individually verified across all 13+ services.
3. **No authenticated health endpoint** — any host with network access can query.
4. **`CatchupStatus` always `200 OK`** regardless of `successRate` — must parse `message` to detect stuck catchup.
5. **`/services` endpoint** registered on default mux (`util/servicemanager/service_manager.go:74-79`); port unspecified.
6. **Docker-host port mapping** — `PORT_PREFIX` (1/2/3) prepended; node 1 health = `:18000`.

### Implementation notes for SP3

- **Probe URL:** `GET http://<host>:8000/health/readiness` for readiness; `/health/liveness` for liveness. Docker-host: `:18000`, `:28000`, `:38000`.
- **Pass:** HTTP `200` with `body.status == "200"`; all `body.services[*].status == "200"`.
- **Fail:** HTTP `503` or any `body.services[*].status == "503"`. Status is JSON string.
- **Content-Type:** do not assert; accept `text/plain` or `application/json`.
- **Catchup detection:** parse `body.services[?(@.service=="BlockValidation")].dependencies[?(@.resource=="CatchupStatus")].message` for `active=false`.
- **OPS-3 should use `/health/readiness`** (not liveness) to verify dependencies.
- **Reference:** `WaitForHealthLiveness` in `test/utils/helper.go:1222-1234`.
- **`/health` (no suffix) on default mux** returns deprecation text — do not use.
