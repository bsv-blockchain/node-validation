# Discovery methodology

This document records how `docs/discovery.md` was produced so a future
engineer can reproduce or extend the discovery without re-deriving the
methodology from scratch.

## Upstream pinning

- Repository: `/Users/oskarsson/gitcheckout/teranode/`
- Commit:     `11f5fa6a81c36490e2796561f76a39294fc422b5`
- Branch:     `test/longest-chain-double-spend`
- Captured:   `2026-04-29T16:00:00Z`
- Captured by: discovery sub-agent fan-out (9 parallel Explore agents)

## Agent fan-out

Nine parallel `Explore` agents, each over `/Users/oskarsson/gitcheckout/teranode/`.

| # | Surface(s) | Search anchors |
|---|---|---|
| 1 | JSON-RPC + auth model | `services/rpc/Server.go`, grep `RegisterRPC`, `BasicAuth`, `rpc_user`/`rpc_pass` |
| 2 | REST / Asset HTTP API | `services/asset/httpimpl/`, grep `apiGroup.GET`, `/api/v1/` |
| 3 | Notifications (WS / SSE / gRPC / Kafka) | `services/asset/centrifuge_impl/`, `services/p2p/HandleWebsocket.go`, grep `centrifuge`, `gorilla/websocket`, `kafka` |
| 4 | P2P listener | `services/legacy/`, `services/p2p/`, `chaincfg/`, grep `DefaultPort`, `libp2p` |
| 5 | Metrics + Health | `services/*/metrics.go`, `daemon/daemon_services.go`, grep `prometheus`, `/health`, `health.CheckAll` |
| 6 | Extended transaction format | `docs/misc/BIP-239.md`, `go-bt/v2`, grep `ExtendedBytes`, `IsExtended`, `0xEF` |
| 7 | Mempool + testmempoolaccept + fee estimation | `services/rpc/Server.go` (rpcHandlersBeforeInit), `services/validator/`, grep `testmempoolaccept`, `EstimateFee`, `getrawmempool` |
| 8 | Double-spend detection / notification | `services/validator/`, `stores/utxo/process_conflicting.go`, grep `doublespend`, `ErrSpent`, `KAFKA_REJECTEDTX` |
| 9 | Settings / configuration | `settings.conf`, `settings_local.conf`, `settings/settings.go`, grep port constants, `gocore.Config` |

## Synthesis rules

1. Each agent's output is appended verbatim to `docs/discovery.md` in the order above.
2. Cross-agent conflicts (e.g. port disagreements between code and `settings.conf`) are inserted by the orchestrator as `**Discrepancy noted**` callouts citing both source references.
3. `docs/discovery.yaml` is derived programmatically from the same findings; the markdown is the human-readable face, the YAML is the structural source of truth.
