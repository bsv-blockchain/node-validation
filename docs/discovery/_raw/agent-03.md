## 3. Notifications

### Summary

Teranode does not use bitcoind-style ZMQ. The primary external notification mechanism is **Centrifuge** (`github.com/centrifugal/centrifuge` v0.33.2), a publish-subscribe framework layered over WebSocket (`gorilla/websocket` v1.5.4). Clients connect to the **Asset service** at `ws://<host>:8090/connection/websocket` (default; config key `asset_httpListenAddress`, default `:8090`; WebSocket path `/connection/websocket`). The Asset service acts as a fan-out relay: it maintains a `gorilla/websocket` connection to the P2P service's internal `/p2p-ws` endpoint (`p2p_httpAddress`, default `localhost:9906`), reads raw JSON notification messages, and re-publishes them onto named Centrifuge channels. The Centrifuge transport is configured as **unidirectional** (server-to-client), JSON-encoded, Centrifuge Protocol Version 2. Internally, inter-service events also flow over **Kafka** (`IBM/sarama` v1.45.1), but Kafka is not a client-facing notification endpoint; it is used for block, subtree, rejected-tx, invalid-block, and invalid-subtree routing between Teranode microservices.

### Findings table

| Topic (Centrifuge channel) | Message format (JSON) | Auth | Source reference |
|---|---|---|---|
| `block` | `{timestamp, type:"block", hash, height, base_url, peer_id, client_name}` — `notificationMsg` struct | Hardcoded `UserID:"42"` set by `authMiddleware`; CORS `*`; no token | `services/p2p/server_helpers.go:48-57`; `services/p2p/HandleWebsocket.go:24-56`; `services/asset/centrifuge_impl/centrifuge.go:370` |
| `subtree` | `{timestamp, type:"subtree", hash, base_url, peer_id, client_name}` | Same | `services/p2p/server_helpers.go:170-178` |
| `node_status` | `{type:"node_status", peer_id, version, commit_hash, best_block_hash, best_height, tx_count, subtree_count, fsm_state, start_time, uptime, client_name, miner_name, listen_mode, chain_work, sync_peer_id, sync_peer_height, sync_peer_block_hash, sync_connected_at, min_mining_tx_fee, connected_peers_count, storage}` — published on every block + 10s ticker | Same | `services/p2p/Server.go:929-956`; `services/p2p/message_types.go:3-28`; `services/p2p/Server.go:1052-1071` |
| `ping` | Empty `SubscribeOptions{}` — channel declared but no producer in non-test code (see Gaps) | Same | `services/asset/centrifuge_impl/centrifuge.go:152` |
| `mining_on` | Empty `SubscribeOptions{}` — channel declared; `NotificationType_NotUsed` (formerly MiningOn) is deprecated; no producer | Same | `services/asset/centrifuge_impl/centrifuge.go:155`; `model/model.pb.go:32` |
| (Kafka) blocks | Protobuf `KafkaBlockTopicMessage{hash, url, peer_id}` — internal only | Kafka TLS optional (`kafka_enableTLS`) | `services/p2p/server_helpers.go:120-138` |
| (Kafka) rejected-tx | Protobuf `KafkaRejectedTxTopicMessage{tx_hash, peer_id, reason}` — internal only | Same | `services/p2p/Server.go:760-804` |
| (no channel) double-spend | Not emitted as a WebSocket notification. Internal only via UTXO store. | N/A | `services/validator/Server.go:12` |
| (no channel) reorg | Not emitted as a WebSocket notification. Internal to blockvalidation. | N/A | `services/blockvalidation/fork_manager.go:778` |

### Source references

- `go.mod:22-23` — `centrifugal/centrifuge v0.33.2`, `centrifugal/centrifuge-go v0.10.1`
- `go.mod:8` — `IBM/sarama v1.45.1` (Kafka)
- `go.mod:29` — `gorilla/websocket v1.5.4-0.20250319132907`
- `services/asset/centrifuge_impl/centrifuge.go:1-674` — Centrifuge server: channels, auth middleware, P2P listener, message relay
- `services/asset/centrifuge_impl/websocket.go:17-19` — ping interval 25s, write timeout 1s, message size limit 64KB
- `services/asset/centrifuge_impl/client/client.go:57-102` — reference Go client; `ws://<addr>/connection/websocket`
- `services/p2p/HandleWebsocket.go:1-289` — P2P-side raw WebSocket server at `/p2p-ws`; `notificationMsg` struct; `broadcastMessage`
- `services/p2p/Server.go:504` — registration: `e.GET("/p2p-ws", s.HandleWebSocket(...))`
- `services/p2p/Server.go:587-590` — libp2p pubsub topics: block, subtree, node_status, rejected_tx
- `services/p2p/message_types.go:1-77` — `BlockMessage`, `SubtreeMessage`, `RejectedTxMessage`, `NodeStatusMessage`
- `services/p2p/server_helpers.go:26-139` — `handleBlockTopic` → notificationCh; Kafka publish
- `services/p2p/server_helpers.go:141-233` — `handleSubtreeTopic` → notificationCh; Kafka publish
- `services/asset/Server.go:208-220` — Centrifuge conditionally disabled by `asset_centrifuge_disable`
- `settings/settings.go:157-158` — `asset_centrifugeListenAddress` default `:8892`; **but** WebSocket handler is mounted on `httpServer` via `AddHTTPHandler` so actual port is the Asset HTTP port `:8090` (`services/asset/centrifuge_impl/centrifuge.go:221`)
- `model/model.pb.go:29-36` — `NotificationType` enum; `NotificationType_NotUsed = 3`

### Reconnect / catch-up semantics

There is **no message-level backfill**. The Centrifuge channel `SubscribeOptions{}` at `services/asset/centrifuge_impl/centrifuge.go:151-158` are zero-valued — no `HistorySize`, no `HistoryTTL`, no `RecoveryMode`. Messages missed during disconnect are permanently lost.

The only warm-start behaviour is the **cached `node_status` snapshot**: on every new WebSocket connection to the Asset service, `OnConnect` publishes the most recent cached `node_status` to the `node_status` channel before live events arrive (`services/asset/centrifuge_impl/centrifuge.go:172-191`).

On the raw P2P WebSocket (`/p2p-ws`), `sendInitialNodeStatuses` (`services/p2p/HandleWebsocket.go:230-248`) sends a fresh `node_status` to every newly registered channel.

The P2P-to-Asset connection auto-reconnects on a 1-second polling loop (`services/asset/centrifuge_impl/centrifuge.go:276-300`). When that connection drops, the cached node status is cleared; new WebSocket upgrades are rejected with HTTP 503 until the P2P connection is re-established (`services/asset/centrifuge_impl/centrifuge.go:543-552`).

The `centrifuge-go` client library (`services/asset/centrifuge_impl/client/client.go:61`) handles automatic client-side reconnection.

### Gaps / ambiguities

1. **`ping` channel has no producer.** Registered (`centrifuge.go:152`), wired (`centrifuge.go:587`), but no code emits a `type:"ping"` message in non-test code.
2. **`mining_on` channel has no producer.** `NotificationType_NotUsed` (formerly MiningOn, `model.pb.go:32`) is deprecated.
3. **`asset_centrifugeListenAddress` vs actual port.** Setting defaults to `:8892` (`settings.go:157`) but Centrifuge handler is added to the shared Asset HTTP server (`:8090`). The `addr` argument to `centrifugeServer.Start` is dead code in the live path. Clients use `:8090`.
4. **No token / JWT auth.** `authMiddleware` injects `UserID:"42"` unconditionally (`centrifuge.go:556`).
5. **REST `subscribe` / `unsubscribe`** at `/subscribe?client=<id>` and `/unsubscribe?client=<id>` exist (`centrifuge.go:222-223`) as alternative subscription, not used by reference client.
6. **No double-spend or reorg notification channel** — these events are internal only.

### Implementation notes for SP3

**Subscription protocol:**

1. Dial `ws://<asset-host>:8090/connection/websocket`.
2. Wait for HTTP upgrade (HTTP 503 = node not ready; retry).
3. Send Centrifuge Protocol v2 connect frame. Server's `OnConnecting` auto-subscribes every client to all five channels regardless of `subs` field. Example (matches `services/p2p/asset_websocket_integration_test.go:49-57`):
   ```json
   {"id":1,"connect":{"subs":{"node_status":{},"block":{},"subtree":{}}}}
   ```
4. Cached `node_status` snapshot will be pushed immediately on connect.

**Message dispatch:** `type` discriminator routes to channel. Server dispatches by lowercased type (`centrifuge.go:370`).

**Heartbeat:** Server sends WebSocket `PingMessage` every **25 seconds** (`DefaultWebsocketPingInterval`, `websocket.go:17`). Clients respond with `PongMessage` within `pongWait ≈ 27.8s` (`websocket.go:174-178`). The `centrifuge-go` client handles this automatically.

**Preferred client:** `github.com/centrifugal/centrifuge-go` (already in `go.mod:23`). Instantiate:
```go
client := centrifuge.NewJsonClient("ws://<host>:8090/connection/websocket", centrifuge.Config{})
```
Matches `services/asset/centrifuge_impl/client/client.go:61`.
