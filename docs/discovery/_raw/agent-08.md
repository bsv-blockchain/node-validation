## 11. Double-spend detection / notification

### Summary

Teranode detects double-spend attempts at two distinct layers:

**Mempool / propagation layer (pre-block):** When a transaction arrives via the Propagation Service or RPC `sendrawtransaction`, the Validator calls `utxoStore.Spend()` which atomically checks whether referenced UTXOs are already spent. If they are, the attempt is **immediately rejected** with `ErrSpent` (`ERR_UTXO_SPENT = 70`) or `ErrTxConflicting` (`ERR_TX_CONFLICTING = 36`). The rejected transaction is published to the Kafka topic `rejectedtx` (env var `KAFKA_REJECTEDTX`) as a `KafkaRejectedTxTopicMessage` and forwarded peer-to-peer over libp2p gossip.

**Block validation layer (post-PoW):** When a mined block arrives from a remote peer, the SubtreeValidation and BlockValidation services can receive double-spending transactions because the remote miner invested PoW. These transactions are stored with `conflicting = true`. A five-phase commit (`ProcessConflicting`) runs on chain reorgs to swap which side of a fork wins. The transaction flagged as "not conflicting" on the longest chain is the one considered likely to confirm.

Detection logic entry points:
- Mempool rejection: `services/validator/Validator.go` `validateInternal()` → `spendUtxos()` → `utxoStore.Spend()`
- Block-level tracking: `stores/utxo/process_conflicting.go` `ProcessConflicting()`
- Reorg coordination: `services/blockassembly/subtreeprocessor/SubtreeProcessor.go` `moveForwardBlock()` + `getConflictingNodes()`

### Findings table

| Aspect | Behaviour | Source ref |
|---|---|---|
| Detection trigger (mempool) | `utxoStore.Spend()` atomically marks UTXOs spent; if already spent, `ErrSpent` (70) | `services/validator/Validator.go:490-558` |
| Detection trigger (block) | `SubtreeProcessor.moveForwardBlock()` calls `getConflictingNodes()` then `utxoStore.ProcessConflicting()` | `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:769-793` |
| Double-spend window | `DoubleSpendWindow` (`double_spend_window_millis`, default `0`) — block assembler delays dequeue to allow competing tx to arrive | `settings/interface.go:236`, `settings/settings.go:31-32`, `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:568-569` |
| Error types | `ErrSpent` (70), `ErrTxConflicting` (36), `ErrTxInvalidDoubleSpend` (32) | `errors/Error_types.go:36,52,57`; `errors/error.pb.go:58,62,78` |
| Notification (Kafka) | Rejected tx → topic `KAFKA_REJECTEDTX` (default `rejectedtx`) as `KafkaRejectedTxTopicMessage{txHash, reason, peer_id}` (protobuf) | `services/validator/Validator.go:308-325`; `util/kafka/kafka_message/kafka_messages.proto:43-47` |
| Notification (P2P) | P2P server consumes `rejectedtx` Kafka topic; re-publishes JSON `RejectedTxMessage{PeerID, ClientName, TxID, Reason}` on libp2p gossip topic `<chain-prefix>-<p2p_rejected_tx_topic>` | `services/p2p/Server.go:741-804`; `services/p2p/message_types.go:72-77` |
| RPC response (rejection) | `sendrawtransaction` returns synchronous JSON-RPC error `{Code: ErrRPCVerify, Message: "TX rejected: <err>"}`. No out-of-band notification beyond error response. | `services/rpc/handlers.go:800-808` |
| UTXO `conflicting` flag | tx-level `Conflicting` bool + parent-tx-level `ConflictingChildren` list | `stores/utxo/process_conflicting.go:120-174`; `docs/topics/architecture/understandingDoubleSpends.md:233-249` |
| Subtree `ConflictingNodes` | Each subtree carries `ConflictingNodes` array of tx hashes; set during block validation | `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:839-885` |
| Message format | Protobuf `KafkaRejectedTxTopicMessage{txHash: string, reason: string, peer_id: string}` | `util/kafka/kafka_message/kafka_messages.proto:43-47` |
| Latency | No SLA in code. Validator Kafka producer buffer 10000 (`Validator.go:161-165`). Only configured delay is `DoubleSpendWindow` (default 0ms). | `services/validator/Validator.go:161`; `settings/settings.go:31-32` |
| Ignored during sync | Rejected-tx handler returns early when FSM state is `CATCHINGBLOCKS` or `LEGACYSYNCING` | `services/validator/Validator.go:295-302` |

### Source references

| File | Lines | Description |
|---|---|---|
| `services/validator/Validator.go` | 282-331 | `ValidateWithOptions` publishes to Kafka on failure |
| `services/validator/Validator.go` | 490-542 | `validateInternal` detects spent UTXOs |
| `services/validator/options.go` | 24-52 | `Options.CreateConflicting` flag (default false) |
| `stores/utxo/process_conflicting.go` | 50-175 | `ProcessConflicting` five-phase commit for reorg |
| `stores/utxo/process_conflicting.go` | 188-209 | `markConflictingRecursively` |
| `services/blockassembly/subtreeprocessor/SubtreeProcessor.go` | 769-793 | `moveForwardBlock` calls `ProcessConflicting` |
| `services/blockassembly/subtreeprocessor/SubtreeProcessor.go` | 839-885 | `getConflictingNodes` |
| `services/blockassembly/subtreeprocessor/SubtreeProcessor.go` | 568-569 | `DoubleSpendWindow` dequeue hold-off |
| `services/p2p/Server.go` | 741-804 | `rejectedTxHandler` consumes Kafka, publishes to P2P gossip |
| `services/p2p/server_helpers.go` | 285-330 | `handleRejectedTxTopic` (informational only) |
| `services/p2p/message_types.go` | 64-77 | `RejectedTxMessage` |
| `util/kafka/kafka_message/kafka_messages.proto` | 43-47 | `KafkaRejectedTxTopicMessage` |
| `errors/Error_types.go` | 36, 52, 57 | `ErrSpent`, `ErrTxConflicting`, `ErrTxInvalidDoubleSpend` |
| `errors/error.pb.go` | 58, 62, 78 | Error codes 32, 36, 70 |
| `settings/interface.go` | 236 | `DoubleSpendWindow time.Duration` |
| `settings/settings.go` | 31-32, 110 | `double_spend_window_millis`, `KAFKA_REJECTEDTX` |
| `stores/utxo/status.pb.go` | 40-41 | `Status_CONFLICTING = 6` |
| `docs/topics/architecture/understandingDoubleSpends.md` | full | Architecture narrative |
| `test/sequentialtest/double_spend/double_spend_test.go` | 29-1191 | Sequential test suite |

### Notification semantics: which transaction is "likely to confirm"?

The transaction that is **not** marked `conflicting` in the UTXO store on the current longest chain.

- **At mempool time** (no blocks yet): the **first-seen** valid transaction spending a UTXO is kept with `conflicting=false`; later attempts are rejected outright and never stored (`Validator.go:490-542`).
- **After a reorg:** `ProcessConflicting()` runs five-phase commit. The **winning-chain transaction** gets `conflicting=false` (`process_conflicting.go:164`); the losing-chain transaction gets `conflicting=true`. Winning = chain with most cumulative PoW.

There is no explicit "likely to confirm" label in the API surface. The test suite uses `VerifyConflictingInUtxoStore(t, false, tx)` to assert non-conflicting (`double_spend_test.go:224-258`).

### Gaps / ambiguities

| Gap | Detail |
|---|---|
| **No client subscription for double-spend** | Only out-of-band channel is Kafka `rejectedtx` topic and its P2P gossip broadcast. No dedicated WebSocket / webhook for clients. RPC WebSocket types in `bsvjson/chainsvrwsntfns.go` are declared (`txaccepted`, `redeemingtx`) but no `dsdetected` path fires. |
| **`ErrTxInvalidDoubleSpend` never raised at runtime** | Defined (`Error_types.go:57`) and tested in unit tests only. `grep` found no production call-site. Production paths raise `ErrSpent` or `ErrTxConflicting`. |
| **`DoubleSpendWindow` defaults to 0** | Default `0ms` makes dequeue hold-off a no-op. Test harness logs but does not enforce minimum (`test/tnb/tnb2_daemon_test.go:55-58`). |
| **Rejected-tx P2P message is informational** | When a peer's P2P server receives `RejectedTxMessage` via gossip, the handler logs and does nothing (`server_helpers.go:327-329`). No mechanism to notify subscribed wallets / SPV clients. |
| **No latency commitment** | No SLA between detection and P2P publication. Path: validator → Kafka producer (buffered 10000) → P2P handler → libp2p Publish. Only documented delay is `DoubleSpendWindow`. |
| **Commented-out test** | `testSingleDoubleSpendNotMinedForLong` disabled waiting for issue #2853 (`double_spend_test.go:73-74,124-126,175-177`). |
| **`testMarkAsConflictingMultipleSameBlock` not implemented** | `t.Errorf("...not implemented")` (`double_spend_test.go:285-287`). Multiple conflicting txs in same block untested. |

### Implementation notes for SP3 / NEW-FR9

**Objective (FR-9):** Detect a double-spend attempt and notify subscribed clients within seconds.

**What exists:**

1. Kafka topic `rejectedtx` (configured via `KAFKA_REJECTEDTX` / `kafka_rejectedTxConfig`) carries protobuf `KafkaRejectedTxTopicMessage{txHash, reason, peer_id}` for every internally rejected tx including double-spends. A test harness running its own Kafka consumer can observe events.
2. The reason string is the Go error message, e.g. `"validator: UTXO Store spend failed for <txid>: utxo already spent"`. No structured error-code field — test must parse `reason` or rely on `peer_id == ""` (internal) + `reason` containing `"spent"` or `"conflicting"`.

**How to construct NEW-FR9 test:**

```
1. Subscribe to Kafka `rejectedtx` topic before sending any tx.
2. Submit valid tx1 spending a known UTXO via Propagation client. Wait for
   block-assembly acceptance.
3. Submit tx2 spending same UTXO. Expect Propagation to return error.
4. Consume Kafka topic. Within 5s assert:
   - msg.txHash == tx2.TxID()
   - msg.peer_id == "" (internal rejection)
   - msg.reason contains "spent" or "conflicting"
5. Optionally verify P2P gossip carries the message via second test node.
```

**Gaps NEW-FR9 must document:**

- No dedicated DS notification subscription API. FR-9 satisfied via generic `rejectedtx` Kafka topic; transport-layer event, not first-class DS notification.
- If `DoubleSpendWindow > 0`, sleep that duration before asserting (`test/tnb/tnb2_daemon_test.go:55-58`).
- If feature absent (no Kafka, no P2P gossip), report `FEATURE_NOT_AVAILABLE`.

**present: true** — detection and Kafka/P2P rejection notification implemented. Dedicated subscribed-client double-spend notification (WebSocket/webhook) **absent**; only observable via Kafka `rejectedtx`.
