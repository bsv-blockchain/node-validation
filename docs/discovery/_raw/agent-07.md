## 8. testmempoolaccept analogue

### Summary

**present: false.**

Teranode has no RPC or REST endpoint named `testmempoolaccept` or any functional equivalent that validates a transaction without committing state changes. The closest architectural surface is the gRPC `ValidateTransaction` endpoint on the validator service, which accepts a boolean option `add_tx_to_block_assembly` (default `true`). Setting that flag to `false` together with `skip_utxo_creation=true` would theoretically run script/fee/policy checks without touching the UTXO store or block assembly, but this is an internal gRPC interface not exposed through the public JSON-RPC layer.

### Findings table

| Method | Signature | Behaviour | Source ref |
|---|---|---|---|
| `sendrawtransaction` (JSON-RPC) | `handleSendRawTransaction(ctx, s, cmd, _)` | Validates via `s.validatorClient.Validate(ctx, tx, 0)` then stores result; **always commits** | `services/rpc/handlers.go:757-812` |
| `ValidateTransaction` (gRPC internal) | `ValidateTransaction(ctx, *ValidateTransactionRequest)` | Full validation; options `skip_utxo_creation`, `add_tx_to_block_assembly`, `skip_policy_checks`, `create_conflicting` | `services/validator/Server.go:447-511` |
| Proto option: `add_tx_to_block_assembly` | `optional bool add_tx_to_block_assembly = 4` | When false, tx is validated but not forwarded to block assembly | `services/validator/validator_api/validator_api.proto:61` |
| Proto option: `skip_utxo_creation` | `optional bool skip_utxo_creation = 3` | When true, UTXO state is not mutated; used internally for in-block validation | `services/validator/validator_api/validator_api.proto:60` |
| `estimatefee` (JSON-RPC) | `handleUnimplemented` | Returns `ErrRPCUnimplemented (-1)` | `services/rpc/Server.go:162` |
| `testmempoolaccept` | not registered | Absent from `rpcHandlersBeforeInit` | `services/rpc/Server.go:156-222` |

### Source references

- `services/rpc/Server.go:156-222` — complete `rpcHandlersBeforeInit` map; `testmempoolaccept` absent.
- `services/rpc/Server.go:378-383` — `handleUnimplemented` returns `ErrRPCUnimplemented`.
- `services/validator/validator_api/validator_api.proto:18-38` — `ValidatorAPI` gRPC; 5 methods, none is a public dry-run.
- `services/validator/validator_api/validator_api.proto:56-64` — `ValidateTransactionRequest` fields.
- `services/validator/Server.go:447-511` — `validateTransaction` impl.
- `services/validator/options.go:11-53` — `Options` defaults: `AddTXToBlockAssembly=true`, `SkipUtxoCreation=false`.
- `services/legacy/netsync/handle_block.go:600-609` — only internal caller using `SkipUtxoCreation=true, AddTXToBlockAssembly=false, SkipPolicyChecks=true` (block-replay).

### Gaps

1. No public JSON-RPC or REST endpoint provides a side-effect-free "would this tx be accepted?" query.
2. The internal gRPC `ValidateTransaction` with `skip_utxo_creation=true, add_tx_to_block_assembly=false` would approximate dry-run but still executes UTXO read checks and script execution; no atomicity guarantees for concurrent callers.
3. No structured response analogous to Bitcoin Core's `testmempoolaccept` array (`allowed`, `reject-reason`, `vsize`, `fees`).

### Implementation notes for SP3 / IBD-2 / PC-2

- **IBD-2 / PC-2 probe strategy:** Tests must use actual `sendrawtransaction` for acceptance probing. There is no safe dry-run path over JSON-RPC. Document broadcast risk: a test that probes with a real send commits the UTXO and propagates the tx.
- **Alternative:** Direct gRPC access to validator with `skip_utxo_creation=true, add_tx_to_block_assembly=false` works as approximation but bypasses some UTXO checks (`Validator.go:540-558`).
- **Recommended SP3 ticket:** Expose a `testmempoolaccept` JSON-RPC endpoint that calls `validateTransaction` with combined options and wraps response in BC-compatible schema.

## 9. Fee estimation

### Summary

**present: false.**

The `estimatefee` JSON-RPC command is declared and has a help string, but its handler is `handleUnimplemented`, returning `ErrRPCUnimplemented (-1)`. `estimatesmartfee` is not registered at all. There is no fee estimation logic anywhere. The node knows only its configured `minminingtxfee` (a static policy floor, not a predictive estimate). No priority levels (economy / standard / priority) exist.

### Findings table

| Method | Signature | Priority levels | Source ref |
|---|---|---|---|
| `estimatefee` (JSON-RPC) | `handleUnimplemented` → `ErrRPCUnimplemented (-1)` | None | `services/rpc/Server.go:162`; `services/rpc/rpcserverhelp.go:140-147,750` |
| `estimatepriority` (JSON-RPC) | In `rpcUnimplemented` map; handler absent | None | `services/rpc/Server.go:274` |
| `estimatesmartfee` | Not registered | None | absent from `services/rpc/Server.go` |
| `MinMiningTxFee` (policy) | `PolicySettings.GetMinMiningTxFee() float64` — static BSV/kB floor | Single value | `settings/policy.go:206-207`; `settings/settings.go:76` |
| `checkFees` (internal) | `(tv *TxValidator) checkFees(tx, blockHeight, utxoHeights)` rejects below floor | Single floor | `services/validator/TxValidator.go:431-474` |
| `EstimateFeeCmd` (bsvjson) | `type EstimateFeeCmd struct { NumBlocks int64 }` — wire only | None wired | `services/rpc/bsvjson/walletsvrcmds.go:71-82` |

### Source references

- `services/rpc/Server.go:162` — `"estimatefee": handleUnimplemented`.
- `services/rpc/Server.go:274` — `"estimatepriority": {}` in `rpcUnimplemented`.
- `services/rpc/rpcserverhelp.go:140-147,750` — help strings exist but handler is non-functional.
- `services/rpc/bsvjson/walletsvrcmds.go:71-82,654-655` — `EstimateFeeCmd`, `EstimatePriorityCmd` structs.
- `settings/policy.go:20,102-103,206-207` — `MinMiningTxFee float64` (BSV/kB).
- `settings/settings.go:76` — Default `0.00000500` BSV/kB.
- `services/validator/TxValidator.go:431-474` — `checkFees` static floor logic.
- `services/legacy/netsync/manager.go:298` — `// feeEstimator *mempool.FeeEstimator` commented out (historical intent).

### Gaps

1. `estimatefee` returns `-1` unconditionally.
2. `estimatepriority` is not even registered as a handler — `unknown command` error.
3. `estimatesmartfee` completely absent.
4. No tiered fee levels (economy / standard / priority) exist anywhere.
5. Only fee datum is the static `MinMiningTxFee` policy.

### Implementation notes for SP3 / NEW-FR8

- **NEW-FR8 outcome:** Report `FEATURE_NOT_AVAILABLE` for all priority levels. Node has no fee market data source.
- **Test probe:** Calling `estimatefee` with any `numBlocks` returns JSON-RPC error `-1`. Test asserts this deterministically.
- **FR-8 gap:** Customer requirement demands economy/standard/priority with <1s response. Teranode would need: (a) per-tx fee-rate persisted in block assembly, (b) percentile-bucket computation on query, (c) new handler for `estimatesmartfee`.

## 10. Mempool query and filtering

### Summary

Teranode's "mempool" is a block-assembly subtree processor, not a traditional mempool. Available queries:

- **List all tx IDs:** `getrawmempool` (JSON-RPC) is **implemented** — returns hashes via `blockAssemblyClient.GetTransactionHashes`.
- **Verbose per-tx detail:** `getrawmempool?verbose=true` is **partially implemented** — returns a single aggregate `GetRawMempoolVerboseResult` (not per-tx) repurposed from mining-candidate data.
- **Query by tx ID:** `getmempoolentry` is **declared** in `rpcUnimplemented` (handler absent).
- **Mempool stats:** `getmempoolinfo` registered as `handleUnimplemented`.
- **Ancestor / descendant:** `getmempoolancestors` / `getmempooldescendants` are **not registered** anywhere.
- **Filter by fee rate:** No filtering dimension exposed publicly. Block assembly stores fee and size internally but no query retrieves them.

### Findings table

| Query | Endpoint | Filter dimensions | Source ref |
|---|---|---|---|
| List all tx IDs | `getrawmempool` (verbose=false) → `[]string` | None | `services/rpc/handlers.go:1251-1289`; `services/rpc/Server.go:190` |
| Verbose mempool | `getrawmempool` (verbose=true) → single `GetRawMempoolVerboseResult{Size, Fee, Time, Height, Depends}` | None — aggregate not per-tx | `services/rpc/handlers.go:1269-1286` |
| Single tx entry | `getmempoolentry` | By TxID — **not implemented** | `services/rpc/Server.go:276`; `services/rpc/bsvjson/chainsvrresults.go:190-207` |
| Mempool stats | `getmempoolinfo` → `GetMempoolInfoResult{Size, Bytes}` | None — **unimplemented** | `services/rpc/Server.go:185` |
| Ancestor chain | `getmempoolancestors` | Not registered | absent |
| Descendant chain | `getmempooldescendants` | Not registered | absent |
| List txs (internal gRPC) | `GetBlockAssemblyTxs` → `{txCount, txs[]}` | None | `services/blockassembly/Server.go:1390-1412` |
| Filter by fee rate | none | Not exposed | Fee stored in `blockassembly.ClientI.Store(hash, fee, size, ...)` but not queryable |
| Tx by ID (asset REST) | `GET /api/v1/tx/:hash` | By tx ID | `services/asset/httpimpl/http.go:185-187` |
| Tx metadata (asset REST) | `GET /api/v1/txmeta/:hash/json` | By tx ID | `services/asset/httpimpl/http.go:193` |

### Source references

- `services/rpc/Server.go:156-222` — handler map; `getrawmempool` present, `getmempoolinfo` unimplemented, `getmempoolentry` in `rpcUnimplemented` (line 276).
- `services/rpc/handlers.go:1249-1289` — `handleGetRawMempool` calls `GetTransactionHashes` for non-verbose; `GetMiningCandidate` for verbose.
- `services/rpc/bsvjson/chainsvrresults.go:190-207` — `GetMempoolEntryResult` data model with ancestor/descendant fields, no implementation.
- `services/rpc/bsvjson/chainsvrcmds.go:375-386` — `GetMempoolEntryCmd{TxID string}`.
- `services/blockassembly/Interface.go:140-148` — `GetTransactionHashes(ctx)` is the only query.
- `services/blockassembly/Interface.go:50` — `Store(ctx, hash, fee, size uint64, ...)` — fee/size stored but not retrievable.
- `services/blockassembly/blockassembly_api/blockassembly_api.proto:70-74,171-175` — `GetBlockAssemblyTxs` returns hashes only.
- `services/asset/httpimpl/http.go:185-197` — Asset REST tx lookup routes.

### Gaps

1. `getmempoolentry` data model defined but handler absent.
2. `getmempoolancestors` / `getmempooldescendants` completely absent.
3. `getmempoolinfo` registered as unimplemented.
4. **No fee-rate filtering anywhere.** Block assembly stores fee/size internally but only `GetTransactionHashes` exposes data, returning hashes only.
5. **Verbose `getrawmempool` is misleading** — returns aggregate, not per-tx map. `Fee` is `miningCandidate.CoinbaseValue` (total coinbase), not per-tx fee.
6. Asset REST `GET /api/v1/tx/:hash` is not mempool-specific (fetches from blob store regardless of confirmation).

### Implementation notes for SP3 / NEW-FR11

- **Query by tx ID:** Use `GET /api/v1/tx/:hash/json` (asset REST) for accepted txs. JSON-RPC `getrawtransaction` proxies the same. Neither confirms mempool-specifically.
- **List all mempool txs:** `getrawmempool` (verbose=false) returns `[]string`. The only working mempool enumeration.
- **Filter by fee rate:** Report `FILTER_NOT_SUPPORTED`.
- **Ancestor/descendant chains:** Report `FEATURE_NOT_AVAILABLE`. Commands not registered.
- **Mempool stats:** `getmempoolinfo` returns `-1`. Report `FEATURE_NOT_AVAILABLE`.
- **SP3 recommendation:** Before SP5 NEW-FR11 tests can pass, must implement: (a) `getmempoolentry` handler returning per-tx fee/size/dependencies; (b) `getmempoolinfo` handler with aggregate count and bytes; (c) optional verbose `getrawmempool` per-tx map. `GetMempoolEntryResult` struct is spec-complete.
