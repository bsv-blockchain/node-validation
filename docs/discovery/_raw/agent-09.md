## Appendix A — Settings & default ports

### Summary: how config is loaded

Teranode uses `github.com/ordishs/gocore` (v1.0.81) for all configuration. There are **no command-line flags** for individual settings — all config flows through gocore's `Configuration` object.

**Loading sequence** (`gocore/config.go:197-280`):

1. `.env` file — loaded via `godotenv` before any conf file. Path overridable via `SETTINGS_ENV_FILE` (default `.env` in cwd).
2. `settings.conf` — mandatory base file. Searches upward from binary dir then cwd. Fatal if found but unreadable; warns and continues if absent.
3. `settings_test.conf` — optional override, loaded silently.
4. `settings_local.conf` — optional local override, takes precedence over `settings.conf`.
5. Environment variables — highest priority. `os.LookupEnv(key)` tried first (`gocore/config.go:561`).

**Context system** (`gocore/config.go:224-236`): `SETTINGS_CONTEXT` (default `"dev"`) selects active context. Optional `SETTINGS_APPLICATION` further qualifies. Conf files use dot-notation `key.context` and `key.context.application`; most-specific match wins.

**Canonical defaults** live in two places:
- `settings.conf` — bare `key = value`.
- `settings/settings.go` — Go fallback as second arg to `getString/getBool/getInt`.

**Precedence (high → low):**
```
env var > settings_local.conf > settings_test.conf > settings.conf (most-specific context match) > Go code default
```

### Default ports table

All port constants in `settings.conf:114-144` under `# @group: PORTS compact`. `PORT_PREFIX` (default `""`) is prepended in Docker multi-node setups.

| Service | Default port | `.conf` key / constant | Go-code default | Source ref |
|---|---|---|---|---|
| JSON-RPC (HTTP) | **9292** | `TERANODE_RPC_PORT` / `rpc_listener_url=http://:9292` | `""` (fatal if unset) | `settings.conf:141`, `settings.go:487`, `services/rpc/Server.go:1455` |
| Asset REST/HTTP | **8090** | `ASSET_HTTP_PORT` / `asset_httpListenAddress=:8090` | `:8090` | `settings.conf:118,217`, `settings.go:161` |
| Asset Centrifuge (notifications) | **8892** (declared) — actually mounted on Asset HTTP `:8090` | `CENTRIFUGE_PORT` / `asset_centrifugeListenAddress=:8892` | `:8892` | `settings.conf:122,202`, `settings.go:157` |
| Blockchain gRPC | **8087** | `BLOCKCHAIN_GRPC_PORT` / `blockchain_grpcListenAddress=:8087` | `:8087` | `settings.conf:117,313`, `settings.go:228` |
| Blockchain HTTP | **8082** | `BLOCKCHAIN_HTTP_PORT` / `blockchain_httpListenAddress=:8082` | `:8082` | `settings.conf:118,320`, `settings.go:229` |
| Block Assembly gRPC | **8085** | `BLOCK_ASSEMBLY_GRPC_PORT` / `blockassembly_grpcListenAddress=:8085` | `:8085` | `settings.conf:119,262`, `settings.go:198` |
| Block Persister HTTP | **8083** | `BLOCK_PERSISTER_HTTP_PORT` / `blockPersister_httpListenAddress=:8083` | `:8083` | `settings.conf:120,229`, `settings.go:170` |
| Block Validation gRPC | **8088** | `BLOCK_VALIDATION_GRPC_PORT` / `blockvalidation_grpcListenAddress=:8088` | `:8088` | `settings.conf:121,406`, `settings.go:241` |
| Subtree Validation gRPC | **8086** | `SUBTREE_VALIDATION_GRPC_PORT` / `subtreevalidation_grpcListenAddress=:8086` | `:8089`* | `settings.conf:140`, `settings.go:425` |
| Validator gRPC | **8081** | `VALIDATOR_GRPC_PORT` / `validator_grpcListenAddress=:8081` | `:8081` | `settings.conf:143,1482`, `settings.go:299` |
| Validator HTTP | **8834** | `VALIDATOR_HTTP_PORT` / `validator_httpListenAddress=:8834` | `""` | `settings.conf:144,1490`, `settings.go:308` |
| Propagation gRPC | **8084** | `PROPAGATION_GRPC_PORT` / `propagation_grpcListenAddress=:8084` | `""` | `settings.conf:138,989`, `settings.go:477` |
| Propagation HTTP | **8833** | `PROPAGATION_HTTP_PORT` / `propagation_httpListenAddress=:8833` | `""` | `settings.conf:139,1004`, `settings.go:471` |
| P2P libp2p TCP | **9905** | `P2P_PORT` / `p2p_port=9905` | `9906`* | `settings.conf:133,913`, `settings.go:372` |
| P2P gRPC | **9904** | `P2P_GRPC_PORT` / `p2p_grpcListenAddress=:9904` | `:9906`* | `settings.conf:131,867`, `settings.go:367` |
| P2P HTTP | **9906** | `P2P_HTTP_PORT` / `p2p_httpListenAddress=:9906` | `""` | `settings.conf:132,880`, `settings.go:369` |
| P2P Bootstrap | **9901** | `P2P_BOOTSTRAP_PORT` | — | `settings.conf:130` |
| Coinbase gRPC | **8093** | `COINBASE_GRPC_PORT` / `coinbase_grpcListenAddress=:8093` | `""` | `settings.conf:122,486`, `settings.go:403` |
| Coinbase P2P | **9907** | `P2P_PORT_COINBASE` / `p2p_port_coinbase` | `9906` | `settings.conf:134,916`, `settings.go:417` |
| Alert P2P | **9908** | `ALERT_P2P_PORT` | `9908` | `settings.conf:115`, `settings.go:153` |
| Legacy gRPC | **8099** | `LEGACY_GRPC_PORT` / `legacy_grpcListenAddress=:8099` | `""` | `settings.conf:128,687`, `settings.go:459` |
| Legacy HTTP | **8098** | `LEGACY_HTTP_PORT` / `legacy_httpListenAddress=:8098` | `""` | `settings.conf:129,699` |
| Health check | **8000** | `HEALTH_CHECK_PORT` / `health_check_httpListenAddress=:8000` | `:8000` | `settings.conf:125,603`, `settings.go:56` |
| Profiler / pprof | **9091** | `PROFILE_PORT` / `profilerAddr=:9091` | `""` (disabled if empty) | `settings.conf:137,961`, `settings.go:53` |
| Prometheus metrics | served on `profilerAddr` at path `/metrics` | path `""` (disabled) | — | `settings.conf:965`, `settings.go:55`, `daemon/daemon_services.go:188` |
| Faucet HTTP | **8097** | `FAUCET_HTTP_PORT` / `faucet_httpListenAddress=:8097` | `""` | `settings.conf:124,575`, `settings.go:492` |
| Kafka | **9092** | `KAFKA_PORT=9092` | `9092` | `settings.conf:63`, `settings.go:109` |
| Aerospike | **3000** | `aerospike_port` | `3000` | `settings.conf:172`, `settings.go:139` |
| PostgreSQL | **5432** | `POSTGRES_PORT=5432` | — | `settings.conf:135` |
| Jaeger UDP | **6831** | `JAEGER_PORT=6831` | — | `settings.conf:126` |
| Jaeger HTTP (OTLP) | **4318** | `JAEGER_PORT_HTTP=4318` | `http://localhost:4318` | `settings.conf:127`, `settings.go:43` |

*\*Discrepancy: `settings.conf:140` defines `SUBTREE_VALIDATION_GRPC_PORT=8086` but `settings.go:425` Go fallback is `:8089`. Conf wins. Similarly `P2P_GRPC_PORT` conf is 9904, Go fallback `:9906`. Conf wins.*

### Auth flags table

| Service | Auth mode | Default | Key | Source ref |
|---|---|---|---|---|
| JSON-RPC | HTTP Basic Auth (SHA256), two-tier admin + limited | Required when set; warns if missing | `rpc_user`, `rpc_pass`, `rpc_limit_user`, `rpc_limit_pass` | `settings.go:480-483`, `services/rpc/Server.go:877-933`, `settings.conf:1023-1026` |
| JSON-RPC default credentials | `bitcoin`/`bitcoin` | **Plaintext defaults in conf** | same | `settings.conf:1024-1026` |
| JSON-RPC max clients | 1 (Go) / 3 (conf) | 3 | `rpc_max_clients` | `settings.conf:1022`, `settings.go:484` |
| All gRPC services | TLS disabled by default | `0` (plain) | `security_level_grpc` | `settings.go:64`, `settings.conf:1028` |
| All HTTP services | TLS disabled by default | `0` | `securityLevelHTTP` | `settings.go:46`, `settings.conf:1032` |
| TLS cert/key files | `certs/teranode.crt` / `certs/teranode.key` | when `securityLevelHTTP > 0` | `server_certFile`, `server_keyFile` | `settings.conf:1034-1036`, `settings.go:47-48` |
| gRPC admin API key | Bearer token | `"testkey"` (hard-coded dev value!) | `grpc_admin_api_key` | `settings.conf:597`, `settings.go:66` |
| Asset HTTP response signing | Ed25519 over response | `false` | `asset_sign_http_responses` | `settings.go:163`, `services/asset/httpimpl/http.go:147` |
| Kafka TLS | TLS for Kafka | `false` | `KAFKA_ENABLE_TLS`, `KAFKA_TLS_SKIP_VERIFY` | `settings.conf:87-89`, `settings.go:125-126` |

### Feature flags table

| Flag | Default | Effect | Source ref |
|---|---|---|---|
| `network` | `mainnet` (bare); `regtest` for `dev`/`docker`; `teratestnet` for `teratestnet` | Selects Bitcoin network params | `settings.conf:778-794`, `settings.go:17` |
| `use_cgo_verifier` | `true` | Inject BDK secp256k1 native verifier (faster) | `daemon/daemon_native.go:12`, `settings.go:60` |
| `useLocalValidator` | `false` | In-process validator vs gRPC | `settings.go:312`, `daemon/daemon_stores.go:142` |
| `blockassembly_disabled` | `false` | Disables Block Assembly entirely | `settings.conf:252-253`, `settings.go:196` |
| `blockassembly_useDynamicSubtreeSize` | Go: `false`; conf: `true` | Dynamic subtree Merkle item count | `settings.conf:293`, `settings.go:220` |
| `double_spend_window_millis` | `0` | DAH filter window in subtree processor (`0` = disabled) | `settings.conf:567`, `settings.go:31-32,216`, `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:569` |
| `blockvalidation_fail_fast_validation` | Go: `true`; conf: `false` | Subtree validation uses txmeta cache only | `settings.conf:394`, `settings.go:177` |
| `blockvalidation_optimistic_mining` | `true` (Go) | Allow block-assembly optimistic execution | `settings.go:259` |
| `blockvalidation_useCatchupWhenBehind` | `false` | Enter catch-up when behind block assembly | `settings.go:266` |
| `startBlockchain` | conf: `true` | Start Blockchain service | `settings.conf:1116`, `daemon/daemon_services.go:58,430-464` |
| `startBlockAssembly` | conf: `true` | Start Block Assembly | `settings.conf:1072`, `daemon/daemon_services.go:59` |
| `startBlockValidation` | conf: `true` | Start Block Validation | `settings.conf:1103`, `daemon/daemon_services.go:61` |
| `startValidator` | conf: `false`; docker: `true` | Start Tx Validator | `settings.conf:1238-1239`, `daemon/daemon_services.go:62` |
| `startSubtreeValidation` | conf: `true` | Start Subtree Validation | `settings.conf:1221`, `daemon/daemon_services.go:60` |
| `startPropagation` | conf: `true` | Start Propagation | `settings.conf:1197`, `daemon/daemon_services.go:63` |
| `startP2P` | conf: `true` | Start P2P (libp2p) | `settings.conf:1177`, `daemon/daemon_services.go:64` |
| `startAsset` | conf: `true` | Start Asset HTTP | `settings.conf:1059`, `daemon/daemon_services.go:65` |
| `startRPC` | conf: `true` | Start JSON-RPC | `settings.conf:1210`, `daemon/daemon_services.go:69` |
| `startLegacy` | conf: `true` (off in `docker`/`operator`) | Start Legacy Bitcoin P2P | `settings.conf:1160`, `daemon/daemon_services.go:68` |
| `dashboard_enabled` | `true` | Enables dashboard UI via Asset centrifuge | `settings.conf:558`, `settings.go:495` |
| `asset_centrifuge_disable` | `false` | Disable real-time notification server | `settings.conf:207`, `settings.go:158` |
| `acceptnonstdoutputs` | `true` | Accept non-standard tx outputs | `settings.go:89` |
| `excessiveblocksize` | conf: `10737418240` (10 GB); Go: `4294967296` (4 GB) | Max accepted block size | `settings.conf:571`, `settings.go:71` |
| `blockmaxsize` | conf bare: `0` (unlimited) | Max mined block size | `settings.conf:364-366`, `settings.go:22` |
| `minminingtxfee` | conf bare: `0.00000001` BSV/kB; `0` for `dev`/`docker`/`teratestnet` | Minimum fee rate | `settings.conf:771-775`, `settings.go:76` |
| `p2p_dht_mode` | conf: `server`; local: `client` | DHT mode | `settings.conf:855`, `settings.go:388` |
| `p2p_enable_mdns` | `false` | mDNS discovery | `settings.go:392` |
| `p2p_allow_private_ips` | `false` | Connect to RFC1918 | `settings.go:393` |
| `fsm_state_restore` | `false` | Restore FSM from snapshot | `settings.conf:584`, `settings.go:233` |

**Cross-cuts for the other agents:**

| Feature | Status | Method / flag | Source ref |
|---|---|---|---|
| `testmempoolaccept` | **Not implemented** | — | `services/rpc/Server.go:156-222` |
| Fee estimation (`estimatefee`) | **Stubbed** (`handleUnimplemented`) | `estimatefee` | `services/rpc/Server.go:162` |
| `getmempoolinfo` | **Stubbed** | — | `services/rpc/Server.go:185` |
| `getrawmempool` | **Implemented** | — | `services/rpc/Server.go:190`, `services/rpc/handlers.go:1249` |
| Double-spend notifications | **Partial** — Kafka topic only, no WS | `double_spend_window_millis=0` | `settings.conf:567`, `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:569` |
| Extended transaction format | **Implemented** (no flag — always active) | — | `go-bt/v2@v2.5.0` (see agent 6) |
| Mempool query | `getrawmempool` only | — | `services/rpc/Server.go:190,317` |

### Network-specific overrides

| Context | Network | Source ref |
|---|---|---|
| bare | `mainnet` | `settings.conf:778` |
| `dev` | `regtest` | `settings.conf:779` |
| `test` | `regtest` | `settings.conf:781` |
| `docker` | `regtest` | `settings.conf:782` |
| `teratestnet` | `teratestnet` | `settings.conf:780` |
| `operator` | `mainnet` | `settings.conf:790` |
| `operator.testnet` | `testnet` | `settings.conf:791` |

| Network | `DefaultPort` (Bitcoin P2P) | `CoinbaseMaturity` | Source ref |
|---|---|---|---|
| mainnet | 8333 | 100 | `go-chaincfg/params.go:255,279` |
| testnet | 18333 | 100 | `go-chaincfg/params.go:532,556` |
| regtest | 18444 | 100 | `go-chaincfg/params.go:451,460` |
| teratestnet | 18333 | 100 | `go-chaincfg/params.go:631,650` |
| stn | 9333 | 100 | `go-chaincfg/params.go:370,379` |

`DefaultPort` is for **Legacy Bitcoin P2P** only. Teranode internal services (gRPC, REST, etc.) use the same ports across all networks.

**Block size overrides:**
- bare: `excessiveblocksize=10737418240` (10 GB)
- `teratestnet`: `excessiveblocksize=1073741824` (1 GB)
- `docker.m`/`operator`: `blockmaxsize=4294967296` (4 GB)

**Fee overrides:** `minminingtxfee=0.00000001` (mainnet/bare); `0` (dev/docker/teratestnet).

### Source references

| File | Role |
|---|---|
| `settings.conf` | Canonical defaults + context variants |
| `settings_local.conf` | Local override (teratestnet-specific in this checkout) |
| `settings/settings.go` | Go struct construction; Go fallback defaults |
| `settings/interface.go` | `Settings` struct + sub-struct types |
| `settings/helpers.go` | Wrappers around `gocore.Config().Get*()` |
| `daemon/daemon.go` | `shouldStart()` — CLI flag + conf key |
| `daemon/daemon_services.go` | Per-service start logic; profiler/metrics wiring |
| `daemon/daemon_native.go` | `use_cgo_verifier` |
| `cmd/teranode/daemon.go` | Entry point; calls `settings.NewSettings()` |
| `services/rpc/Server.go` | RPC method registry, auth, listener |
| `services/asset/httpimpl/http.go` | Asset HTTP server; `securityLevelHTTP` branching |

### Gaps & ambiguities

1. **`http_sign_response` vs `asset_sign_http_responses` key mismatch.** `settings.conf:612` sets `http_sign_response=true` but Go reads `asset_sign_http_responses` (`settings.go:163`, default `false`). Conf entry **silently ignored**. Effective `SignHTTPResponses=false` unless explicit override.
2. **`blockvalidation_fail_fast_validation` conf/code disagreement.** `settings.conf:394` is `false`; `settings.go:177` Go fallback is `true`. Conf wins normally.
3. **`blockvalidation_optimistic_mining` absent from conf.** Go default `true` (`settings.go:259`). Controlled only by env var or local.conf.
4. **`blockvalidation_useCatchupWhenBehind` absent from conf.** Go default `false` (`settings.go:266`).
5. **`testmempoolaccept` completely absent** from RPC handler registry.
6. **`estimatefee` and `getmempoolinfo` stubbed** with `handleUnimplemented`.
7. **`grpc_admin_api_key` default is `"testkey"`** in `settings.conf:597` — plaintext dev credential. Production deployments must override.
8. **Profiler/Prometheus disabled by default** in operator context (`profilerAddr.operator` not set).
9. **`SUBTREE_VALIDATION_GRPC_PORT=8086` (conf) vs `:8089` (Go fallback)** — conf wins; node configured without conf would unexpectedly listen on 8089.
