---
upstream_commit: "11f5fa6a81c36490e2796561f76a39294fc422b5"
upstream_branch: "test/longest-chain-double-spend"
discovered_at:   "2026-04-29T16:00:00Z"
discovered_by:   "discovery sub-agent fan-out (9 parallel Explore agents)"
---

# Teranode External Interfaces — Discovery

> **Upstream pinning.** The findings below were derived from the
> Teranode commit recorded in the frontmatter. SP3 starts by checking
> the SHA still matches; if not, re-run SP2.

## Summary table

| # | Surface | Present? | Endpoint / port | Auth | Source-of-truth file |
|---|---|---|---|---|---|
| 1 | JSON-RPC service | true | http://host:9292/ (POST /) | Basic Auth (two-tier) | `services/rpc/Server.go` |
| 2 | REST / Asset HTTP API | true | http://host:8090/api/v1 | None (GET); dashboard session (some POSTs) | `services/asset/httpimpl/http.go` |
| 3 | Notifications | true | ws://host:8090/connection/websocket | None (hardcoded UserID:"42") | `services/asset/centrifuge_impl/centrifuge.go` |
| 4 | P2P listener | partial | TCP 8333 (legacy BSV wire) + TCP 9905 (libp2p) | None / libp2p noise | `services/legacy/peer_server.go`, `services/p2p/Server.go` |
| 5 | Metrics endpoint | true | http://host:9091/metrics | None | `daemon/daemon_services.go` |
| 6 | Health endpoint | true | http://host:8000/health/readiness | None | `daemon/daemon.go` |
| 7 | Extended transaction format | true | BIP-239 EF marker (0xEF), all submission paths | N/A | `go-bt/v2`, `docs/misc/BIP-239.md` |
| 8 | testmempoolaccept analogue | false | — | — | `services/rpc/Server.go` |
| 9 | Fee estimation | false | — | — | `services/rpc/Server.go` |
| 10 | Mempool query / filtering | partial | `getrawmempool` (JSON-RPC) | Basic Auth | `services/rpc/handlers.go` |
| 11 | Double-spend detection / notification | partial | Kafka topic `rejectedtx`; no WebSocket DS channel | N/A | `services/validator/Validator.go` |

(Sections 1–11 follow, plus appendices.)

## 1. JSON-RPC service

### Summary

The Teranode JSON-RPC service is a concurrent HTTP/1.x server implemented in Go under `services/rpc/`. It listens on port **9292** by default (configured via `TERANODE_RPC_PORT = 9292` in `settings.conf`; the listener URL defaults to `http://:${TERANODE_RPC_PORT}`). All requests are made via HTTP POST to the root path `/`. The authentication model is **HTTP Basic Auth** with two tiers: an **admin** user (`rpc_user` / `rpc_pass`) who may call any method, and an optional **limited** user (`rpc_limit_user` / `rpc_limit_pass`) who may only call methods enumerated in the `rpcLimited` set. Credentials are stored as SHA-256 hashes of the base64-encoded `"Basic user:pass"` string and compared with constant-time operations. No cookie-file, JWT, or API-key auth is present in the RPC service itself (a separate dashboard UI at `services/asset/` uses session cookies, but that is unrelated to the JSON-RPC endpoint). The service implements a Bitcoin SV–compatible JSON-RPC 1.0 wire format (`"jsonrpc":"1.0"`, responses contain `result`, `error`, and `id` fields). Default configured credentials in `settings.conf` are `bitcoin:bitcoin`.

### Findings table

Methods are enumerated from the `rpcHandlersBeforeInit` map (`services/rpc/Server.go:156-222`). The "Auth" column uses **A** = admin-only (not in `rpcLimited`) and **L** = available to limited users (in `rpcLimited`). "Impl" = handler has a real implementation; "Unimpl" = routed to `handleUnimplemented` which returns `ErrRPCUnimplemented (-1)`.

| Method | Signature (params) | Auth | Impl | Source ref |
|---|---|---|---|---|
| `createrawtransaction` | `(inputs []TransactionInput, amounts map[string]float64, locktime *int64)` | L | Impl | `services/rpc/Server.go:159`, `services/rpc/handlers.go:606` |
| `debuglevel` | `(levelSpec string)` | A | Unimpl | `services/rpc/Server.go:160` |
| `decoderawtransaction` | `(hexTx string)` | L | Unimpl | `services/rpc/Server.go:161` |
| `decodescript` | `(hexScript string)` | L | Unimpl | `services/rpc/Server.go:162` |
| `estimatefee` | `(numBlocks int64)` | L | Unimpl | `services/rpc/Server.go:163` |
| `generate` | `(numBlocks uint32)` | A | Impl | `services/rpc/Server.go:164`, `services/rpc/handlers.go:842` |
| `generatetoaddress` | `(numBlocks int32, address string, maxTries *int32)` | A | Impl | `services/rpc/Server.go:165`, `services/rpc/handlers.go:915` |
| `getaddednodeinfo` | `(dns bool, node *string)` | A | Unimpl | `services/rpc/Server.go:166` |
| `getbestblock` | `()` | L | Unimpl | `services/rpc/Server.go:167` |
| `getbestblockhash` | `()` | L | Impl | `services/rpc/Server.go:168`, `services/rpc/handlers.go:439` |
| `getblock` | `(hash string, verbosity *uint32=1)` | L | Impl | `services/rpc/Server.go:169`, `services/rpc/handlers.go:87` |
| `getblockbyheight` | `(height uint32, verbosity *uint32=1)` | A | Impl | `services/rpc/Server.go:170`, `services/rpc/handlers.go:163` |
| `getblockchaininfo` | `()` | L | Impl | `services/rpc/Server.go:171`, `services/rpc/handlers.go:1363` |
| `getblockcount` | `()` | L | Unimpl | `services/rpc/Server.go:172` |
| `getblockhash` | `(index int64)` | L | Impl | `services/rpc/Server.go:173`, `services/rpc/handlers.go:204` |
| `getblockheader` | `(hash string, verbose *bool=true)` | L | Impl | `services/rpc/Server.go:174`, `services/rpc/handlers.go:229` |
| `getblocktemplate` | `(request *TemplateRequest)` | A | Unimpl | `services/rpc/Server.go:175` |
| `getcfilter` | `(hash string, filterType wire.FilterType)` | L | Unimpl | `services/rpc/Server.go:176` |
| `getcfilterheader` | `(hash string, filterType wire.FilterType)` | L | Unimpl | `services/rpc/Server.go:177` |
| `getchaintips` | `()` | A | Impl | `services/rpc/Server.go:178`, `services/rpc/handlers.go:2621` |
| `getconnectioncount` | `()` | A | Unimpl | `services/rpc/Server.go:179` |
| `getcurrentnet` | `()` | L | Unimpl | `services/rpc/Server.go:180` |
| `getdifficulty` | `()` | L | Impl | `services/rpc/Server.go:181`, `services/rpc/handlers.go:1319` |
| `getgenerate` | `()` | A | Unimpl | `services/rpc/Server.go:182` |
| `gethashespersec` | `()` | A | Unimpl | `services/rpc/Server.go:183` |
| `getheaders` | `(blockLocators []string, hashStop string)` | L | Unimpl | `services/rpc/Server.go:184` |
| `getinfo` | `()` | L | Impl | `services/rpc/Server.go:185`, `services/rpc/handlers.go:1525` |
| `getmempoolinfo` | `()` | L | Unimpl | `services/rpc/Server.go:186` |
| `getmininginfo` | `()` | A | Impl | `services/rpc/Server.go:187`, `services/rpc/handlers.go:2331` |
| `getnettotals` | `()` | L | Unimpl | `services/rpc/Server.go:188` |
| `getnetworkhashps` | `(blocks *int32, height *int32)` | L | Unimpl | `services/rpc/Server.go:189` |
| `getpeerinfo` | `()` | A | Impl | `services/rpc/Server.go:190`, `services/rpc/handlers.go:1089` |
| `getrawmempool` | `(verbose *bool=false)` | L | Impl | `services/rpc/Server.go:191`, `services/rpc/handlers.go:1251` |
| `getrawtransaction` | `(txid string, verbose *int=0)` | L | Impl | `services/rpc/Server.go:192`, `services/rpc/handlers.go:491` |
| `gettxout` | `(txid string, vout uint32, includeMempool *bool=true)` | L | Unimpl | `services/rpc/Server.go:193` |
| `gettxoutproof` | `(txids []string, blockHash *string)` | L | Unimpl | `services/rpc/Server.go:194` |
| `help` | `(command *string)` | L | Impl (stub) | `services/rpc/Server.go:195`, `services/rpc/handlers.go:1856` |
| `node` | `(subCmd NodeSubCmd, target string, connectSubCmd *string)` | A | Unimpl | `services/rpc/Server.go:196` |
| `ping` | `()` | A | Unimpl | `services/rpc/Server.go:197` |
| `invalidateblock` | `(blockHash string)` | A | Impl | `services/rpc/Server.go:198`, `services/rpc/handlers.go:1742` |
| `isbanned` | `(ipOrSubnet string)` | A | Impl | `services/rpc/Server.go:199`, `services/rpc/handlers.go:1928` |
| `listbanned` | `()` | A | Impl | `services/rpc/Server.go:200`, `services/rpc/handlers.go:2006` |
| `clearbanned` | `()` | A | Impl | `services/rpc/Server.go:201`, `services/rpc/handlers.go:2112` |
| `reconsiderblock` | `(blockHash string)` | A | Impl | `services/rpc/Server.go:202`, `services/rpc/handlers.go:1799` |
| `searchrawtransactions` | `(...)` | L | Unimpl | `services/rpc/Server.go:203` |
| `sendrawtransaction` | `(hexTx string, allowHighFees *bool=false)` | L | Impl | `services/rpc/Server.go:204`, `services/rpc/handlers.go:757` |
| `setban` | `(ipOrSubnet, command string, banTime *int64, absolute *bool)` | A | Impl | `services/rpc/Server.go:205`, `services/rpc/handlers.go:2168` |
| `setgenerate` | `(generate bool, genProcLimit *int=-1)` | A | Unimpl | `services/rpc/Server.go:206` |
| `stop` | `()` | A | Impl | `services/rpc/Server.go:207`, `services/rpc/Server.go:565` |
| `submitblock` | `(hexBlock string, options *SubmitBlockOptions)` | L | Unimpl | `services/rpc/Server.go:208` |
| `uptime` | `()` | L | Unimpl | `services/rpc/Server.go:209` |
| `validateaddress` | `(address string)` | L | Unimpl | `services/rpc/Server.go:210` |
| `verifychain` | `(checkLevel *int32=3, checkDepth *int32=288)` | A | Unimpl | `services/rpc/Server.go:211` |
| `verifymessage` | `(address, signature, message string)` | L | Unimpl | `services/rpc/Server.go:212` |
| `verifytxoutproof` | `(proof string)` | L | Unimpl | `services/rpc/Server.go:213` |
| `version` | `()` | L | Impl | `services/rpc/Server.go:214`, `services/rpc/Server.go:577` |
| `getminingcandidate` | `(provideCoinbaseTx *bool=false, verbosity *uint32=0)` | L | Impl | `services/rpc/Server.go:216`, `services/rpc/handlers.go:995` |
| `submitminingsolution` | `(miningSolution MiningSolution)` | L | Impl | `services/rpc/Server.go:217`, `services/rpc/handlers.go:1668` |
| `freeze` | `(txid string, vout int, utxoHash string)` | A | Impl | `services/rpc/Server.go:220`, `services/rpc/handlers.go:2401` |
| `unfreeze` | `(txid string, vout int, utxoHash string)` | A | Impl | `services/rpc/Server.go:221`, `services/rpc/handlers.go:2450` |
| `reassign` | `(oldTxid string, oldVout int, oldUtxoHash, newUtxoHash string)` | A | Impl | `services/rpc/Server.go:222`, `services/rpc/handlers.go:2500` |

Wallet commands (routed to `handleAskWallet` returning `ErrRPCNoWallet`): `addmultisigaddress`, `backupwallet`, `createencryptedwallet`, `createmultisig`, `dumpprivkey`, `dumpwallet`, `encryptwallet`, `getaccount`, `getaccountaddress`, `getaddressesbyaccount`, `getbalance`, `getnewaddress`, `getrawchangeaddress`, `getreceivedbyaccount`, `getreceivedbyaddress`, `gettransaction`, `gettxoutsetinfo`, `getunconfirmedbalance`, `getwalletinfo`, `importprivkey`, `importwallet`, `keypoolrefill`, `listaccounts`, `listaddressgroupings`, `listlockunspent`, `listreceivedbyaccount`, `listreceivedbyaddress`, `listsinceblock`, `listtransactions`, `listunspent`, `lockunspent`, `move`, `sendfrom`, `sendmany`, `sendtoaddress`, `setaccount`, `settxfee`, `signmessage`, `signrawtransaction`, `walletlock`, `walletpassphrase`, `walletpassphrasechange` — source: `services/rpc/Server.go:227-270`.

### Source references

| Claim | Path:line |
|---|---|
| Package-level architecture doc | `services/rpc/Server.go:1-43` |
| API semver constant (`1.3.0`) | `services/rpc/Server.go:82-87` |
| Auth timeout (10 s) | `services/rpc/Server.go:93` |
| `rpcHandlersBeforeInit` full map | `services/rpc/Server.go:156-222` |
| `rpcAskWallet` set | `services/rpc/Server.go:227-270` |
| `rpcLimited` set (limited-user allowed methods) | `services/rpc/Server.go:284-331` |
| `RPCServer` struct definition | `services/rpc/Server.go:602-698` |
| `checkAuth` implementation (two-tier, SHA-256, constant-time) | `services/rpc/Server.go:905-936` |
| `WWW-Authenticate: Basic realm="bsvd RPC"` on auth failure | `services/rpc/Server.go:1224` |
| `http.ServeMux` at `HandleFunc("/")` | `services/rpc/Server.go:1294` |
| Auth checked via `checkAuth(r, true)` (always required) | `services/rpc/Server.go:1315` |
| `NewServer` — credential hashing, `rpcMaxClients`, `RPCListenerURL` | `services/rpc/Server.go:1387-1469` |
| `RPCSettings` struct definition | `settings/interface.go:537-548` |
| `TERANODE_RPC_PORT = 9292` | `settings.conf:142` |
| `rpc_listener_url = http://:${TERANODE_RPC_PORT}` | `settings.conf:1016` |
| `rpc_user = bitcoin`, `rpc_pass = bitcoin` | `settings.conf:1024-1026` |
| `rpc_max_clients = 3` | `settings.conf:1022` |
| `RPCSettings` wired in `NewSettings` | `settings/settings.go:479-490` |
| JSON-RPC 1.0 request struct | `services/rpc/bsvjson/jsonrpc.go:123-135` |
| `NewRequest` produces `"jsonrpc":"1.0"` | `services/rpc/bsvjson/jsonrpc.go:181` |
| `Response` struct (`result`, `error`, `id`) | `services/rpc/bsvjson/jsonrpc.go:206-217` |
| Standard JSON-RPC 2.0 error codes (`-32600` to `-32700`) + BSV-specific | `services/rpc/bsvjson/jsonrpcerr.go:1-86` |
| Test helper — Basic `bitcoin:bitcoin`, POST to root URL | `test/utils/helper.go:71-100` |
| VS Code REST client example — port 9292, Basic `bitcoin:bitcoin` | `services/rpc/rpccurl.http:14-16` |

### Auth model

The RPC service implements **HTTP Basic Authentication only** with no support for cookie files, JWT, or API keys at the JSON-RPC endpoint.

**Two-tier model:**

1. **Admin** credentials (`rpc_user` / `rpc_pass` in `settings.conf`). The SHA-256 hash is stored in `RPCServer.authsha`. Admin users may call any implemented method including `stop`, `generate`, `invalidateblock`, `setban`, `freeze`, `unfreeze`, `reassign`. Source: `services/rpc/Server.go:615-617`, `services/rpc/Server.go:1438-1441`.
2. **Limited** credentials (`rpc_limit_user` / `rpc_limit_pass`, optional, no default). Hash stored in `RPCServer.limitauthsha`. Limited users may only invoke methods listed in `rpcLimited` (`services/rpc/Server.go:284-331`). Attempting an admin-only method returns `{"error":{"code":-32602,"message":"limited user not authorized for this method"}}`. Source: `services/rpc/Server.go:619-621`, `services/rpc/Server.go:1444-1447`, `services/rpc/Server.go:1177-1183`.

Limited is checked first; if matches, `isAdmin=false`; otherwise admin is checked; if neither matches and `require=true`, `401 Unauthorized` with `WWW-Authenticate: Basic realm="bsvd RPC"`. Source: `services/rpc/Server.go:905-936`, `services/rpc/Server.go:1222-1226`.

**No TLS is implemented in the RPC server itself.** Plain HTTP only. Source: `services/rpc/Server.go:1285-1290`.

### Gaps / ambiguities

1. **`getblockbyheight` auth level** — present in `rpcHandlersBeforeInit` but absent from `rpcLimited`, making it admin-only by the gate at `services/rpc/Server.go:1177`. Undocumented BSV extension behaviour.
2. **`getchaintips` listed in both** `rpcHandlersBeforeInit` (real handler) and `rpcUnimplemented` (`services/rpc/Server.go:274`); the real handler wins, the second entry is dead code.
3. **`getrawmempool` semantics** — Teranode has no traditional mempool (delegates to validator/block-assembly services). Behaviour under load not determinable from code alone.
4. **`getrawtransaction` proxies through Asset HTTP** at `http://localhost:8090/api/v1/tx/{txid}/hex`. RPC failure if Asset service is down. Source: `services/rpc/handlers.go:501-517`.
5. **TLS** — `NewServer` doc comment claims TLS validation but no `tls.Config` or `ListenAndServeTLS` exists. Plain HTTP.
6. **`rpc_max_clients` defaults** — Go default is 1 (`settings/settings.go:484`), `settings.conf` overrides to 3. Heavy concurrent test load may serialise.
7. **`rpc_quirks=true`** by default (`settings/settings.go:485`); strict JSON-RPC 2.0 notifications without `id` get no response.
8. **Cleartext credentials on the wire** in absence of TLS unless an external proxy is in front.

### Implementation notes for SP3

**Connecting:**
- Default URL: `http://<host>:9292/` (root path, POST only).
- Port via `TERANODE_RPC_PORT`; default `9292` (`settings.conf:142`).
- Multi-node Docker: node1→`19292`, node2→`29292`, node3→`39292` (`settings.conf:1017-1019`).

**Authenticating:** HTTP Basic Auth `Authorization: Basic <base64(user:pass)>`. Default `bitcoin:bitcoin`. `Content-Type: application/json`.

**Request format (JSON-RPC 1.0 compatible):**
```json
{"method":"getbestblockhash","params":[],"id":"any"}
```
The `"jsonrpc"` field is optional; `"id"` should always be set.

**Response format:**
```json
{"result":"<value>","error":null,"id":"any"}
```
Error: `{"result":null,"error":{"code":<int>,"message":"<string>"},"id":"any"}`. Codes follow Bitcoin Core conventions; see `services/rpc/bsvjson/jsonrpcerr.go`.

**Timeout:** Per-call timeout `rpc_timeout` (default `30s`, `settings/settings.go:488`). Timeout response: `{"code":-30,"message":"RPC call timed out after 30 seconds"}`.

**Connection limit:** `rpc_max_clients=3`; excess gets `HTTP 503`. Read timeout 10s for handshake.


## 2. REST / Asset HTTP API

### Summary

The Teranode Asset service exposes a REST HTTP API using the **Echo v4** framework. By default it listens on **`:8090`** (configurable via the `asset_httpListenAddress` setting; env override `ASSET_HTTP_PORT`). All data endpoints are grouped under the API prefix **`/api/v1`** (configurable via `asset_apiPrefix`; default value `"/api/v1"`). A second, separate legacy-compatibility group is mounted at `/rest`. The server supports both plain HTTP and TLS; the mode is selected by the `securityLevelHTTP` setting (0 = HTTP). Response signing via Ed25519 is optional and controlled by `asset_sign_http_responses`.

### Findings table

| # | Endpoint | Method | Path | Auth | Source ref |
|---|----------|--------|------|------|------------|
| 1 | Transaction by TXID (binary) | GET | `/api/v1/tx/{hash}` | None | `services/asset/httpimpl/http.go:185` |
| 2 | Transaction by TXID (hex) | GET | `/api/v1/tx/{hash}/hex` | None | `services/asset/httpimpl/http.go:186` |
| 3 | Transaction by TXID (JSON) | GET | `/api/v1/tx/{hash}/json` | None | `services/asset/httpimpl/http.go:187` |
| 4 | Batch transactions (binary POST) | POST | `/api/v1/txs` | None | `services/asset/httpimpl/http.go:190` |
| 5 | Transaction metadata (JSON) | GET | `/api/v1/txmeta/{hash}/json` | None | `services/asset/httpimpl/http.go:193` |
| 6 | Raw transaction metadata (binary) | GET | `/api/v1/txmeta_raw/{hash}` | None | `services/asset/httpimpl/http.go:195` |
| 7 | Raw transaction metadata (hex) | GET | `/api/v1/txmeta_raw/{hash}/hex` | None | `services/asset/httpimpl/http.go:196` |
| 8 | Raw transaction metadata (JSON) | GET | `/api/v1/txmeta_raw/{hash}/json` | None | `services/asset/httpimpl/http.go:197` |
| 9 | Block by hash (binary) | GET | `/api/v1/block/{hash}` | None | `services/asset/httpimpl/http.go:233` |
| 10 | Block by hash (hex) | GET | `/api/v1/block/{hash}/hex` | None | `services/asset/httpimpl/http.go:234` |
| 11 | Block by hash (JSON) | GET | `/api/v1/block/{hash}/json` | None | `services/asset/httpimpl/http.go:235` |
| 12 | Block forks | GET | `/api/v1/block/{hash}/forks` | None | `services/asset/httpimpl/http.go:236` |
| 13 | N consecutive blocks from hash (binary) | GET | `/api/v1/blocks/{hash}` | None | `services/asset/httpimpl/http.go:227` |
| 14 | N consecutive blocks from hash (JSON) | GET | `/api/v1/blocks/{hash}/json` | None | `services/asset/httpimpl/http.go:229` |
| 15 | Paginated block list | GET | `/api/v1/blocks` | None | `services/asset/httpimpl/http.go:224` |
| 16 | Single block header by hash (binary) | GET | `/api/v1/header/{hash}` | None | `services/asset/httpimpl/http.go:220` |
| 17 | Single block header by hash (hex) | GET | `/api/v1/header/{hash}/hex` | None | `services/asset/httpimpl/http.go:221` |
| 18 | Single block header by hash (JSON) | GET | `/api/v1/header/{hash}/json` | None | `services/asset/httpimpl/http.go:222` |
| 19 | Best (chain-tip) block header (binary) | GET | `/api/v1/bestblockheader` | None | `services/asset/httpimpl/http.go:252` |
| 20 | Best block header (hex) | GET | `/api/v1/bestblockheader/hex` | None | `services/asset/httpimpl/http.go:253` |
| 21 | Best block header (JSON) | GET | `/api/v1/bestblockheader/json` | None | `services/asset/httpimpl/http.go:254` |
| 22 | Multiple consecutive headers from hash (binary) | GET | `/api/v1/headers/{hash}` | None | `services/asset/httpimpl/http.go:207` |
| 23 | Multiple consecutive headers (hex) | GET | `/api/v1/headers/{hash}/hex` | None | `services/asset/httpimpl/http.go:208` |
| 24 | Multiple consecutive headers (JSON) | GET | `/api/v1/headers/{hash}/json` | None | `services/asset/httpimpl/http.go:209` |
| 25 | UTXO by UTXO hash (binary) | GET | `/api/v1/utxo/{hash}` | None | `services/asset/httpimpl/http.go:246` |
| 26 | UTXO by UTXO hash (hex) | GET | `/api/v1/utxo/{hash}/hex` | None | `services/asset/httpimpl/http.go:247` |
| 27 | UTXO by UTXO hash (JSON) | GET | `/api/v1/utxo/{hash}/json` | None | `services/asset/httpimpl/http.go:248` |
| 28 | All UTXOs for a transaction (JSON) | GET | `/api/v1/utxos/{hash}/json` | None | `services/asset/httpimpl/http.go:250` |
| 29 | Search (hash or height) | GET | `/api/v1/search` | None | `services/asset/httpimpl/http.go:240` |
| 30 | Block locator | GET | `/api/v1/block_locator` | None | `services/asset/httpimpl/http.go:225` |
| 31 | Merkle proof by TXID (binary) | GET | `/api/v1/merkle_proof/{hash}` | None | `services/asset/httpimpl/http.go:256` |
| 32 | Merkle proof (hex/JSON) | GET | `/api/v1/merkle_proof/{hash}/hex`, `/api/v1/merkle_proof/{hash}/json` | None | `services/asset/httpimpl/http.go:257-258` |
| 33 | Catchup status | GET | `/api/v1/catchup/status` | None | `services/asset/httpimpl/http.go:335` |
| 34 | Peer list | GET | `/api/v1/peers` | None | `services/asset/httpimpl/http.go:338` |
| 35 | Block stats | GET | `/api/v1/blockstats` | None | `services/asset/httpimpl/http.go:241` |
| 36 | Last N blocks | GET | `/api/v1/lastblocks` | None | `services/asset/httpimpl/http.go:244` |
| 37 | Block subtrees (JSON) | GET | `/api/v1/block/{hash}/subtrees/json` | None | `services/asset/httpimpl/http.go:238` |
| 38 | Legacy block (binary, REST compat) | GET | `/rest/block/{hash}.bin` | None | `services/asset/httpimpl/http.go:180` |
| 39 | Legacy block (API compat) | GET | `/api/v1/block_legacy/{hash}` | None | `services/asset/httpimpl/http.go:231` |
| 40 | Headers to common ancestor (deprecated) | GET | `/api/v1/headers_to_common_ancestor/{hash}` | None | `services/asset/httpimpl/http.go:212-214` |
| 41 | Headers from common ancestor | GET | `/api/v1/headers_from_common_ancestor/{hash}` | None | `services/asset/httpimpl/http.go:216-218` |
| 42 | Block invalidate / revalidate | POST | `/api/v1/block/invalidate`, `/api/v1/block/revalidate` | Dashboard auth (POST only, when dashboard enabled) | `services/asset/httpimpl/http.go:330-332` |
| 43 | FSM state, events, states | GET/POST | `/api/v1/fsm/state`, `/api/v1/fsm/events`, `/api/v1/fsm/states` | None (GET); Dashboard auth (POST) | `services/asset/httpimpl/http.go:310-313` |
| 44 | Liveness probe | GET | `/alive` | None | `services/asset/httpimpl/http.go:164` |
| 45 | Readiness/health | GET | `/health` | None | `services/asset/httpimpl/http.go:168` |

**Address history: absent.** No `/address/` route registered anywhere in `services/asset/httpimpl/`. Search: `grep -rn "address.*history\|/address\|GetAddress" services/asset/` — zero results.

### Source references

- `services/asset/httpimpl/http.go` — All route registrations (lines 164–361); settings loading; signing logic.
- `services/asset/httpimpl/Readmode.go:28-48` — `BINARY_STREAM`/`HEX`/`JSON` constants.
- `services/asset/httpimpl/GetTransaction.go:133-201` — Transaction handler; JSON shape; error codes.
- `services/asset/httpimpl/GetBlock.go:35-357` — `GetBlockByHash` and `GetBlockByHeight` handlers; `BlockExtended` JSON schema.
- `services/asset/httpimpl/GetBlockHeader.go:90-148` — Single-header handler; 80-byte binary layout.
- `services/asset/httpimpl/GetBlockHeaders.go:112-203` — Bulk-header handler; `n` query param (default 100, max 1000).
- `services/asset/httpimpl/GetBestBlockHeader.go:82-120` — Chain-tip header handler.
- `services/asset/httpimpl/GetUTXO.go:82-133` — UTXO-by-hash handler; JSON shape.
- `services/asset/httpimpl/GetUTXOsByTXID.go:121-226` — Per-output UTXO scan; `UTXOItem` JSON schema.
- `services/asset/httpimpl/GetTransactions.go:78-191` — Batch POST handler; binary-only streaming response.
- `services/asset/httpimpl/GetBlocks.go:101-154` — Paginated block list; `ExtendedResponse` shape.
- `services/asset/httpimpl/helpers.go:13-98` — `Pagination`/`ExtendedResponse` structs; `getLimitOffset` defaults/max.
- `services/asset/httpimpl/sendError.go:14-83` — Standard error envelope `{status, code, error}`.
- `services/asset/httpimpl/Search.go:94-178` — Search handler; error code scheme.
- `services/asset/httpimpl/get_catchup_status.go:12-70` — Catchup status JSON shape.
- `services/asset/Server.go:47-286` — Server struct; `HTTPListenAddress` consumption; Init/Start wiring.
- `settings/settings.go:156-164` — Defaults: `:8090`, `/api/v1`, `http://localhost:8090/api/v1`, signing disabled.

### Response format

Three content representations negotiated via URL suffix, not `Accept` header:

| Suffix | Content-Type | Format |
|--------|-------------|--------|
| (none) | `application/octet-stream` | Raw binary (Bitcoin wire) |
| `/hex` | `text/plain` | Lower-case hex string |
| `/json` | `application/json` | Pretty-printed JSON (2-space indent) |

Block headers binary body is exactly **80 bytes** (`services/asset/httpimpl/GetBlockHeader.go:51-63`).

**Pagination** for listing endpoints: `?offset=N&limit=M` (default 20, max 100). Envelope: `{"data": [...], "pagination": {"offset": N, "limit": N, "totalRecords": N}}` (`services/asset/httpimpl/helpers.go:13-31`).

**Response signing** (optional): `X-Signature` header containing Ed25519 signature when `asset_sign_http_responses=true` (`services/asset/httpimpl/http.go:436-449`).

### Gaps / ambiguities

1. **Block-by-height HTTP route absent.** Handler `GetBlockByHeight` is implemented (`services/asset/httpimpl/GetBlock.go:136-206`) but not registered in the router. Indirect via `GET /api/v1/search?q={height}` (returns hash, not block).
2. **`GET /api/v1/balance` is a stale comment.** Line 69 of `http.go` mentions it in godoc but no route is registered; no `GetBalance` exists.
3. **Address history completely absent.** UTXO lookup requires the caller to compute `util.UTXOHash(txid, vout, script, satoshis)`.
4. **`/api/v1/txs` and `/:hash/txs` marked for removal** (`services/asset/httpimpl/http.go:189`).
5. **`/api/v1/headers_to_common_ancestor/:hash` deprecated** (`services/asset/httpimpl/http.go:211`).
6. **Dashboard auth scope:** POST endpoints require session-cookie auth only when `dashboard.enabled=true`; otherwise unauthenticated.
7. **`/api/v1/block_legacy/{hash}` returns pre-subtree serialisation,** documented for eventual removal.

### Implementation notes for SP3

**Base URL:** `http://<node>:8090/api/v1` (default; `ASSET_HTTP_PORT` env override).

**Headers:** None required. CORS permissive. Gzip applied server-side.

**Content negotiation:** URL suffix only. Append nothing for binary, `/hex` for hex, `/json` for JSON.

**Error format:** Echo default `{"message": "..."}` for most errors; `sendError()` returns `{"status": <int>, "code": <int32>, "error": "..."}`. Tests handle both. Source: `services/asset/httpimpl/sendError.go:14-29`.

**Response signing:** If enabled, verify `X-Signature` header (hex Ed25519) over the raw resource hash before consuming.

**Pagination:** `?offset=N&limit=M` (max 100).

**Hash format:** 64-char lowercase hex, reversed byte order (Bitcoin display convention). Wrong length → HTTP 400.

**Batch-TX POST:** `POST /api/v1/txs` body is concatenated 32-byte hashes (no separator). Response: concatenated raw transactions in `application/octet-stream`. Order not guaranteed.


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
- `services/asset/centrifuge_impl/centrifuge.go:1-673` — Centrifuge server: channels, auth middleware, P2P listener, message relay
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
> **Discrepancy noted.** `settings/settings.go:157` sets `asset_centrifugeListenAddress` default to `:8892`, but the Centrifuge WebSocket handler is mounted on the shared Asset HTTP server at `:8090` via `services/asset/centrifuge_impl/centrifuge.go:221`. The `:8892` value is dead code in the live path. Agent 3 (Notifications) and Agent 9 (Settings appendix) both flag this. Clients must connect on `:8090`.

- `model/model.pb.go:29-36` — `NotificationType` enum; `NotificationType_NotUsed = 3`

### Reconnect / catch-up semantics

There is **no message-level backfill**. The Centrifuge channel `SubscribeOptions{}` at `services/asset/centrifuge_impl/centrifuge.go:151-158` are zero-valued — no `HistorySize`, no `HistoryTTL`, no `RecoveryMode`. Messages missed during disconnect are permanently lost.

The only warm-start behaviour is the **cached `node_status` snapshot**: on every new WebSocket connection to the Asset service, `OnConnect` publishes the most recent cached `node_status` to the `node_status` channel before live events arrive (`services/asset/centrifuge_impl/centrifuge.go:172-191`).

On the raw P2P WebSocket (`/p2p-ws`), `sendInitialNodeStatuses` (`services/p2p/HandleWebsocket.go:230-248`) sends a fresh `node_status` to every newly registered channel.

The P2P-to-Asset connection auto-reconnects on a 1-second polling loop (`services/asset/centrifuge_impl/centrifuge.go:276-300`). When that connection drops, the cached node status is cleared; new WebSocket upgrades are rejected with HTTP 503 until the P2P connection is re-established (`services/asset/centrifuge_impl/centrifuge.go:543-552`).

The `centrifuge-go` client library (`services/asset/centrifuge_impl/client/client.go:61`) handles automatic client-side reconnection.

### Gaps / ambiguities

1. **`ping` channel has no producer.** Registered (`services/asset/centrifuge_impl/centrifuge.go:152`), wired (`services/asset/centrifuge_impl/centrifuge.go:587`), but no code emits a `type:"ping"` message in non-test code.
2. **`mining_on` channel has no producer.** `NotificationType_NotUsed` (formerly MiningOn, `model/model.pb.go:32`) is deprecated.
3. **`asset_centrifugeListenAddress` vs actual port.** Setting defaults to `:8892` (`settings/settings.go:157`) but Centrifuge handler is added to the shared Asset HTTP server (`:8090`). The `addr` argument to `centrifugeServer.Start` is dead code in the live path. Clients use `:8090`.
4. **No token / JWT auth.** `authMiddleware` injects `UserID:"42"` unconditionally (`services/asset/centrifuge_impl/centrifuge.go:556`).
5. **REST `subscribe` / `unsubscribe`** at `/subscribe?client=<id>` and `/unsubscribe?client=<id>` exist (`services/asset/centrifuge_impl/centrifuge.go:222-223`) as alternative subscription, not used by reference client.
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

**Message dispatch:** `type` discriminator routes to channel. Server dispatches by lowercased type (`services/asset/centrifuge_impl/centrifuge.go:370`).

**Heartbeat:** Server sends WebSocket `PingMessage` every **25 seconds** (`DefaultWebsocketPingInterval`, `services/asset/centrifuge_impl/websocket.go:17`). Clients respond with `PongMessage` within `pongWait ≈ 27.8s` (`services/asset/centrifuge_impl/websocket.go:174-178`). The `centrifuge-go` client handles this automatically.

**Preferred client:** `github.com/centrifugal/centrifuge-go` (already in `go.mod:23`). Instantiate:
```go
client := centrifuge.NewJsonClient("ws://<host>:8090/connection/websocket", centrifuge.Config{})
```
Matches `services/asset/centrifuge_impl/client/client.go:61`.


## 4. P2P listener

### Summary

Teranode operates **two separate P2P listener surfaces** on different transports and ports — the source of the "P2P gateway" terminology in project documentation:

1. **Legacy listener** (`services/legacy/`): speaks the **standard BSV-wire P2P protocol**. TCP port **8333** (mainnet), **18333** (testnet/teratestnet), **18444** (regtest). Port comes from `chaincfg.Params.DefaultPort`; overridable via `legacy_config_port`. Performs Bitcoin P2P handshake (`version`/`verack`) using `go-wire` serialisation, protocol version 70016. Peer discovery via DNS seeds (`seed.bitcoinsv.io` mainnet, `testnet-seed.bitcoinsv.io` testnet) plus optional `--addpeer`/`--connect`.
2. **Teranode-native listener** (`services/p2p/`): speaks **libp2p** (GossipSub/Kademlia-DHT), not Bitcoin wire. TCP port **9905** by default (`P2P_PORT = 9905` in `settings.conf`; gRPC/HTTP control plane on 9906). Protocol ID `/teranode/bitcoin/<network>/1.0.0`. Discovery via DHT + bootstrap peers (teranode-bootstrap.bsvb.tech:9901); no DNS seeds.

The legacy service translates between standard wire and Teranode's internal Kafka/gRPC architecture — the gateway described in project docs.

### Findings table

| Property | Value | Source ref |
|---|---|---|
| Legacy listen port (mainnet) | TCP 8333 | go-chaincfg@v1.4.0/params.go:255 |
| Legacy listen port (testnet/teratestnet) | TCP 18333 | `go-chaincfg@v1.4.0/params.go:532,628` |
| Legacy listen port (regtest) | TCP 18444 | go-chaincfg@v1.4.0/params.go:449 |
| Legacy listen port (STN) | TCP 9333 | go-chaincfg@v1.4.0/params.go:370 |
| Legacy port override | `legacy_config_port` / `--port` | `services/legacy/config.go:146`, `services/legacy/peer_server.go:2685-2687` |
| Teranode-native P2P port | TCP 9905 (default) | `settings.conf:134` |
| Teranode-native gRPC/HTTP port | 9906 | `settings.conf:133` |
| Mainnet magic bytes | `0xe8f3e1e3` | go-wire@v1.0.6/protocol.go:178 |
| Testnet magic bytes | `0xf4f3e5f4` | go-wire@v1.0.6/protocol.go:184 |
| TeraTestNet magic bytes | `0x0c09010d` | go-wire@v1.0.6/protocol.go:187 |
| RegTestNet magic bytes | `0xfabfb5da` | go-wire@v1.0.6/protocol.go:181 |
| Wire protocol version (max) | 70016 | go-wire@v1.0.6/protocol.go:17 |
| Min acceptable protocol version | 209 | `services/legacy/peer/peer.go:44` |
| Version message format | Standard `MsgVersion` | go-wire@v1.0.6/msg_version.go:29-56 |
| User agent (legacy) | `/teranode-legacy-p2p:0.13.0` | `services/legacy/peer_server.go:87,91`; `services/legacy/version/version.go:19-21` |
| Services bits (full node) | `SFNodeNetwork \| SFNodeBloom \| SFNodeCF \| SFNodeBitcoinCash` | `services/legacy/peer_server.go:63-64` |
| Required services for outbound | `SFNodeNetwork` | `services/legacy/peer_server.go:68` |
| User-agent filter | Inbound peers banned if UA does not contain `"Bitcoin SV"` or `"BSV"` | `services/legacy/peer_server.go:541-549` |
| DNS seeds (mainnet) | `seed.bitcoinsv.io` | go-chaincfg@v1.4.0/params.go:256-258 |
| DNS seeds (testnet) | `testnet-seed.bitcoinsv.io` | go-chaincfg@v1.4.0/params.go:533-535 |
| DNS seeds (regtest/STN/teratestnet) | None (empty) | `go-chaincfg@v1.4.0/params.go:371,452,633` |
| DNS seed disable flag | `--nodnsseed` / `DisableDNSSeed` | `services/legacy/config.go:155` |
| libp2p bootstrap peers | teranode-bootstrap.bsvb.tech:9901 (prod), teranode-bootstrap-stage.bsvb.tech:9901 (teratestnet/dev) | `settings.conf:812-813` |
| libp2p protocol ID | `/teranode/bitcoin/<network>/1.0.0` | `services/p2p/Server.go:301` |
| libp2p topic prefix | `teranode/bitcoin/1.0.0/<network>` | go-chaincfg@v1.4.0/params.go:254 |
| libp2p transport | TCP via multiaddr; DHT (Kademlia); GossipSub | go-p2p-message-bus@v0.1.3/client.go:21-31 |
| libp2p discovery | DHT (server mode) + optional mDNS (default off) + bootstrap + static peers | `services/p2p/Server.go:321-325`; `settings.conf:388,392` |

### Source references

- go-chaincfg@v1.4.0/params.go:251-363 — `MainNetParams`, `TestNetParams`, `RegressionNetParams`, `TeraTestNetParams`, `StnParams`
- go-wire@v1.0.6/protocol.go:169-199 — `BitcoinNet` constants; `ServiceFlag` definitions
- go-wire@v1.0.6/msg_version.go:29-56 — `MsgVersion` struct
- `services/legacy/params.go:49-95` — Maps `chaincfg` to legacy service
- `services/legacy/config.go:144-146,155` — `Listeners`, `DisableDNSSeed`
- `services/legacy/Server.go:304-308,316-325` — `Init()` builds listen addresses from `activeNetParams.DefaultPort`
- `services/legacy/peer_server.go:63-68,87-91,534,541-549,2108,2223-2233,2685-2687,2956-2973` — `defaultServices`, user agent, listener setup
- `services/legacy/version/version.go:19-21` — `AppMajor=0`, `AppMinor=13`, `AppPatch=0`
- `services/legacy/connmgr/seed.go:35-79` — `SeedFromDNS()` iterates `chainParams.DNSSeeds`
- `services/p2p/Server.go:176-179,298-302,316-325,362-367` — `p2pPort` config, `bitcoinProtocolVersion`, `p2pMessageBus.Config`, topic name
- go-p2p-message-bus@v0.1.3/client.go:21-31 — libp2p imports (libp2p, kad-dht, pubsub, mdns)
- `settings.conf:133-134,370-372,447,809-813,888,913,941-949` — port and peer settings
- `compose/docker-compose-3blasters.yml:174,182` — Docker port mappings confirm 9905

### Gaps / ambiguities

1. **Gateway/translation layer** — described in `services/legacy/Server.go:18-19` but distributed across `peer_server.go`, netsync handlers, Kafka producers; no consolidated file. SP3 must probe each listener separately.
2. **Port 9905 vs 9906** — 9905 is libp2p TCP listener; 9906 is the Echo HTTP / gRPC control plane. Without `p2p_listen_addresses` and `p2p_port` both set, libp2p may auto-detect.
3. **TeraTestNet DNS seeds absent** — relies entirely on static peers / bootstrap multiaddrs.
4. **libp2p protocol version separate from Bitcoin protocol version** — `1.0.0` libp2p, `70016` Bitcoin wire. External BSV nodes touch only the legacy layer.
5. **UPnP** — present (`services/legacy/upnp.go`) but disabled by default.
6. **ProtocolVersion 70016 supports extended messages (>4GB)** — BSV-specific extension; encoding details not documented in repo.

### Implementation notes for SP3

**Legacy listener (port 8333 / 18333 / 18444):**

- Transport: **TCP only** (`net.Listen("tcp4", ...)` and `tcp6`, `services/legacy/peer_server.go:2551-2575`).
- Handshake: **full Bitcoin P2P version handshake required** to avoid disconnect. Probe must reply with valid `version` (protocol ≥209, UA containing `"Bitcoin SV"` or `"BSV"`, otherwise IP gets banned `services/legacy/peer_server.go:541-549`). Then `verack`. Port-open check insufficient.
- Magic bytes per network: mainnet `0xe8f3e1e3`, testnet `0xf4f3e5f4`, teratestnet `0x0c09010d`.
- Services bits: advertise at minimum `SFNodeNetwork`.
- Recommended: use `go-wire` library directly to serialise `MsgVersion` / `MsgVerAck`.

**Teranode-native listener (port 9905):**

- Transport: **TCP via libp2p multiaddr** (multistream-select, noise encryption, yamux mux). Raw TCP fails to negotiate.
- A valid probe requires `go-libp2p` host, connecting to multiaddr `/ip4/<host>/tcp/9905`, verifying protocol ID `/teranode/bitcoin/<network>/1.0.0`.
- For SP3, **port-open check (TCP SYN) is sufficient** to confirm process is listening; full libp2p dial only needed for protocol verification. Consider `go-p2p-message-bus.NewClient` with `bitcoinProtocolVersion` string.


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


## 7. Extended transaction format

### Summary

**Present: `true`.**

Teranode implements the **Transaction Extended Format (TEF)**, specified in **BIP-239** (authored by Simon Ordish and Siggi Oskarsson, Status: Proposal, 2022-11-09). The spec ships in-tree at `docs/misc/BIP-239.md`. The format is fully implemented via `github.com/bsv-blockchain/go-bt/v2` (v2.5.0, `go.mod:17`). Every production submission path — gRPC `ProcessTransaction`, HTTP `POST /tx`, batch `POST /txs` — accepts both standard and extended formats. The validator auto-extends standard-format transactions by fetching missing UTXO data from the internal store; the proto comment ("must be extended") is aspirational, not enforced at the wire boundary.

### Findings table

| Field | Description | Source ref |
|---|---|---|
| **EF marker** | 6-byte `0x00 0x00 0x00 0x00 0x00 0xEF` immediately after the 4-byte version field. Signals extended format. | `docs/misc/BIP-239.md:51`; go-bt/v2@v2.5.0/tx.go:541 |
| **Recognition / parsing** | `bt.NewTxFromBytes()` reads version, then inputCount VarInt. If 0/0 it reads 4 more bytes; if `0xEF` (BE) it sets `tx.extended = true` and continues with real inputCount. | go-bt/v2@v2.5.0/tx.go:142-162 |
| **`IsExtended()` predicate** | Returns true if `extended` flag set, **or** every input has non-nil `PreviousTxScript`. | go-bt/v2@v2.5.0/tx.go:299-316 |
| **Extended input fields** | After `SequenceNumber` each input carries `PreviousTxSatoshis` (uint64 LE, 8 bytes) + VarInt script length + `PreviousTxScript`. | `docs/misc/BIP-239.md:71-82`; go-bt/v2@v2.5.0/input.go:99-127 |
| **`ExtendedBytes()` serialiser** | Produces the BIP-239 byte sequence including marker. `SerializeBytes()` calls `ExtendedBytes()` iff `IsExtended()`, else falls back to standard `Bytes()`. | go-bt/v2@v2.5.0/tx.go:379-393 |
| **Parsing entry point** | `bt.NewTxFromBytes(transactionData)` at `services/validator/Server.go:457` (validator HTTP/gRPC) and `services/propagation/Server.go:866` (propagation gRPC). Format-agnostic. | `services/validator/Server.go:457`; `services/propagation/Server.go:866` |
| **Validator auto-extension** | `Validator.Validate()` calls `tx.IsExtended()` at line 430; if false → `getTransactionInputBlockHeightsAndExtendTx()` queries UTXO store and sets `tx.SetExtended(true)`. | `services/validator/Validator.go:428-440,930` |
| **Fee calculation requires extension** | `calculateFee()` in legacy netsync errors if `!tx.IsExtended()`. Extension must precede fee checks. | `services/legacy/netsync/handle_block.go:812-814` |
| **Storage format** | Storage uses `SerializeBytes()` (potentially non-extended). UTXO store is the authoritative source for extra fields. Block persister writes non-extended. | `docs/references/services/blockpersister_reference.md:409`; `docs/references/glossary.md:40` |

### Source references

| File | Lines | Purpose |
|---|---|---|
| `docs/misc/BIP-239.md` | 1-90 | Full spec |
| `docs/references/glossary.md` | 40 | Definitive dual-format acceptance statement |
| `docs/howto/submitting_transactions.md` | 60-112, 376-378, 440 | User guide: both formats accepted |
| `docs/references/protobuf_docs/propagationProto.md` | 139 | gRPC service description: "must be extended" note |
| `services/propagation/propagation_api/propagation_api.proto` | 23-25 | Proto comment: "must be extended; coinbase not allowed" |
| `services/propagation/propagation_api/propagation_api_grpc.pb.go` | 39, 98 | Generated stub repeating same comment |
| `services/propagation/Server.go` | 849-956 | `processTransaction` + `processTransactionInternal` — actual wire path; no IsExtended guard |
| `services/validator/Validator.go` | 428-440, 694, 721-931, 943-955, 967-976 | Auto-extension logic |
| `go-bt/v2@v2.5.0/tx.go` | 140-195, 299-393, 530-570 | EF parsing, `IsExtended`, `ExtendedBytes`, `SerializeBytes` |
| `go-bt/v2@v2.5.0/input.go` | 33-34, 52-127, 227-244 | `PreviousTxSatoshis`, `PreviousTxScript`; `readFrom(extended bool)` |
| `test/consensus/helpers.go` | 32-43 | `CreateExtendedTx()` |
| `test/e2e/daemon/ready/sendrawtransaction_test.go` | 47 | E2E submits `tx.ExtendedBytes()` to RPC |
| `test/e2e/daemon/ready/smoke_test.go` | 116, 331-333, 416, 646, 770, 862, 866, 955, 1344, 1493, 1578 | Smoke tests submit `tx.ExtendedBytes()` |
| `services/legacy/netsync/handle_block.go` | 812-814 | Fee calc asserts extended |

### Backward compatibility

**Yes.** Same endpoints accept standard (non-extended) Bitcoin transaction format. Glossary at `docs/references/glossary.md:40`: "Teranode accepts transactions in both standard Bitcoin format and Extended Format. When standard format transactions are received, Teranode automatically extends them during validation by retrieving input data from the UTXO store."

Submission guide at `docs/howto/submitting_transactions.md:440`: "Teranode accepts both standard and extended transaction formats." Auto-extension code at `services/validator/Validator.go:428-440` confirms.

The proto comment at `services/propagation/propagation_api/propagation_api.proto:23-25` says "must be extended" but `processTransactionInternal` (`services/propagation/Server.go:908-956`) has no `IsExtended()` guard. Comment is stale or aspirational.

### Gaps / ambiguities

1. **Proto comment vs code mismatch** — `services/propagation/propagation_api/propagation_api.proto:24` and generated stub say "must be extended" but no runtime enforcement. CLIENT-2 should not rely on rejection of non-extended.
2. **No feature flag / build tag.** Always active; go-bt always parses both.
3. **Round-trip storage is non-extended.** Asset Service `GET /tx/{hash}` returns non-extended bytes. No endpoint round-trips EF verbatim.
4. **Coinbase cannot be extended.** Legacy netsync skips coinbase extension (`services/legacy/netsync/handle_block.go:717`). CLIENT-2 should use non-coinbase only.
5. **Spec status** — BIP-239 is "Proposal", no finalized BIP number recorded.

### Implementation notes for SP3 / CLIENT-2

**Constructing an extended-format transaction** (using `github.com/bsv-blockchain/go-bt/v2` v2.5.0):

```go
input.PreviousTxSatoshis = <satoshi_value>   // uint64
input.PreviousTxScript   = <locking_script>  // *bscript.Script
txBytes := tx.ExtendedBytes()
```

`IsExtended()` becomes true automatically when `PreviousTxScript` is non-nil on every input.

**Endpoints that accept it:**
- HTTP (Propagation): `POST /tx`, body = `ExtendedBytes()`, `Content-Type: application/octet-stream` (`services/propagation/Server.go:654`).
- gRPC (Propagation): `ProcessTransaction` with `ProcessTransactionRequest{Tx: tx.ExtendedBytes()}` (`services/propagation/propagation_api/propagation_api.proto:25`).
- HTTP (Validator, internal): `POST /tx` — same byte format, fallback for large txs (`services/validator/Server.go:850`).

**Round-trip:** Not retrievable as extended. CLIENT-2 should verify acceptance on submission (HTTP 200 / gRPC OK) rather than expect retrieved bytes to carry the EF marker.

**Test pattern** (from `test/e2e/daemon/ready/smoke_test.go:116`):
```go
txHex := hex.EncodeToString(newTx.ExtendedBytes())
// POST txHex to propagation /tx, expect 200
```


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
- **Alternative:** Direct gRPC access to validator with `skip_utxo_creation=true, add_tx_to_block_assembly=false` works as approximation but bypasses some UTXO checks (`services/validator/Validator.go:540-558`).
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
| Latency | No SLA in code. Validator Kafka producer buffer 10000 (`services/validator/Validator.go:161-165`). Only configured delay is `DoubleSpendWindow` (default 0ms). | `services/validator/Validator.go:161`; `settings/settings.go:31-32` |
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

- **At mempool time** (no blocks yet): the **first-seen** valid transaction spending a UTXO is kept with `conflicting=false`; later attempts are rejected outright and never stored (`services/validator/Validator.go:490-542`).
- **After a reorg:** `ProcessConflicting()` runs five-phase commit. The **winning-chain transaction** gets `conflicting=false` (`stores/utxo/process_conflicting.go:164`); the losing-chain transaction gets `conflicting=true`. Winning = chain with most cumulative PoW.

There is no explicit "likely to confirm" label in the API surface. The test suite uses `VerifyConflictingInUtxoStore(t, false, tx)` to assert non-conflicting (`test/sequentialtest/double_spend/double_spend_test.go:224-258`).

### Gaps / ambiguities

| Gap | Detail |
|---|---|
| **No client subscription for double-spend** | Only out-of-band channel is Kafka `rejectedtx` topic and its P2P gossip broadcast. No dedicated WebSocket / webhook for clients. RPC WebSocket types in `bsvjson/chainsvrwsntfns.go` are declared (`txaccepted`, `redeemingtx`) but no `dsdetected` path fires. |
| **`ErrTxInvalidDoubleSpend` never raised at runtime** | Defined (`errors/Error_types.go:57`) and tested in unit tests only. `grep` found no production call-site. Production paths raise `ErrSpent` or `ErrTxConflicting`. |
| **`DoubleSpendWindow` defaults to 0** | Default `0ms` makes dequeue hold-off a no-op. Test harness logs but does not enforce minimum (`test/tnb/tnb2_daemon_test.go:55-58`). |
| **Rejected-tx P2P message is informational** | When a peer's P2P server receives `RejectedTxMessage` via gossip, the handler logs and does nothing (`services/p2p/server_helpers.go:327-329`). No mechanism to notify subscribed wallets / SPV clients. |
| **No latency commitment** | No SLA between detection and P2P publication. Path: validator → Kafka producer (buffered 10000) → P2P handler → libp2p Publish. Only documented delay is `DoubleSpendWindow`. |
| **Commented-out test** | `testSingleDoubleSpendNotMinedForLong` disabled waiting for issue #2853 (`double_spend_test.go:73-74,124-126,175-177`). |
| **`testMarkAsConflictingMultipleSameBlock` not implemented** | `t.Errorf("...not implemented")` (`test/sequentialtest/double_spend/double_spend_test.go:285-287`). Multiple conflicting txs in same block untested. |

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


## Appendix A — Settings & default ports

### Summary: how config is loaded

Teranode uses `github.com/ordishs/gocore` (v1.0.81) for all configuration. There are **no command-line flags** for individual settings — all config flows through gocore's `Configuration` object.

**Loading sequence** (gocore/config.go:197-280):

1. `.env` file — loaded via `godotenv` before any conf file. Path overridable via `SETTINGS_ENV_FILE` (default `.env` in cwd).
2. `settings.conf` — mandatory base file. Searches upward from binary dir then cwd. Fatal if found but unreadable; warns and continues if absent.
3. `settings_test.conf` — optional override, loaded silently.
4. `settings_local.conf` — optional local override, takes precedence over `settings.conf`.
5. Environment variables — highest priority. `os.LookupEnv(key)` tried first (gocore/config.go:561).

**Context system** (gocore/config.go:224-236): `SETTINGS_CONTEXT` (default `"dev"`) selects active context. Optional `SETTINGS_APPLICATION` further qualifies. Conf files use dot-notation `key.context` and `key.context.application`; most-specific match wins.

**Canonical defaults** live in two places:
- `settings.conf` — bare `key = value`.
- `settings/settings.go` — Go fallback as second arg to `getString/getBool/getInt`.

**Precedence (high → low):**
```
env var > settings_local.conf > settings_test.conf > settings.conf (most-specific context match) > Go code default
```

### Default ports table

All port constants in `settings.conf:115-144` under `# @group: PORTS compact`. `PORT_PREFIX` (default `""`) is prepended in Docker multi-node setups.

| Service | Default port | `.conf` key / constant | Go-code default | Source ref |
|---|---|---|---|---|
| JSON-RPC (HTTP) | **9292** | `TERANODE_RPC_PORT` / `rpc_listener_url=http://:9292` | `""` (fatal if unset) | `settings.conf:142`, `settings/settings.go:487`, `services/rpc/Server.go:1455` |
| Asset REST/HTTP | **8090** | `ASSET_HTTP_PORT` / `asset_httpListenAddress=:8090` | `:8090` | `settings.conf:117,217`, `settings/settings.go:161` |
| Asset Centrifuge (notifications) | **8892** (declared) — actually mounted on Asset HTTP `:8090` | `CENTRIFUGE_PORT` / `asset_centrifugeListenAddress=:8892` | `:8892` | `settings.conf:123,202`, `settings/settings.go:157` |
| Blockchain gRPC | **8087** | `BLOCKCHAIN_GRPC_PORT` / `blockchain_grpcListenAddress=:8087` | `:8087` | `settings.conf:118,313`, `settings/settings.go:228` |
| Blockchain HTTP | **8082** | `BLOCKCHAIN_HTTP_PORT` / `blockchain_httpListenAddress=:8082` | `:8082` | `settings.conf:119,320`, `settings/settings.go:229` |
| Block Assembly gRPC | **8085** | `BLOCK_ASSEMBLY_GRPC_PORT` / `blockassembly_grpcListenAddress=:8085` | `:8085` | `settings.conf:120,262`, `settings/settings.go:198` |
| Block Persister HTTP | **8083** | `BLOCK_PERSISTER_HTTP_PORT` / `blockPersister_httpListenAddress=:8083` | `:8083` | `settings.conf:121,229`, `settings/settings.go:170` |
| Block Validation gRPC | **8088** | `BLOCK_VALIDATION_GRPC_PORT` / `blockvalidation_grpcListenAddress=:8088` | `:8088` | `settings.conf:122,406`, `settings/settings.go:241` |
| Subtree Validation gRPC | **8086** | `SUBTREE_VALIDATION_GRPC_PORT` / `subtreevalidation_grpcListenAddress=:8086` | `:8089`* | `settings.conf:141`, `settings/settings.go:425` |
> **Discrepancy noted.** `settings.conf:141` defines `SUBTREE_VALIDATION_GRPC_PORT=8086` but `settings/settings.go:425` Go fallback is `:8089`. The conf value wins at runtime; a node started without `settings.conf` would unexpectedly bind on `:8089`.

| Validator gRPC | **8081** | `VALIDATOR_GRPC_PORT` / `validator_grpcListenAddress=:8081` | `:8081` | `settings.conf:143,1482`, `settings/settings.go:299` |
| Validator HTTP | **8834** | `VALIDATOR_HTTP_PORT` / `validator_httpListenAddress=:8834` | `""` | `settings.conf:144,1490`, `settings/settings.go:308` |
| Propagation gRPC | **8084** | `PROPAGATION_GRPC_PORT` / `propagation_grpcListenAddress=:8084` | `""` | `settings.conf:139,989`, `settings/settings.go:477` |
| Propagation HTTP | **8833** | `PROPAGATION_HTTP_PORT` / `propagation_httpListenAddress=:8833` | `""` | `settings.conf:140,1004`, `settings/settings.go:471` |
| P2P libp2p TCP | **9905** | `P2P_PORT` / `p2p_port=9905` | `9906`* | `settings.conf:134,913`, `settings/settings.go:372` |
| P2P gRPC | **9904** | `P2P_GRPC_PORT` / `p2p_grpcListenAddress=:9904` | `:9906`* | `settings.conf:132,867`, `settings/settings.go:367` |
> **Discrepancy noted.** `settings.conf:132` defines `P2P_GRPC_PORT=9904` but `settings/settings.go:367` Go fallback is `:9906`. Conf value wins. Note that `:9906` is also the P2P HTTP port (`settings.conf:133`), so the Go fallback would collide.

| P2P HTTP | **9906** | `P2P_HTTP_PORT` / `p2p_httpListenAddress=:9906` | `""` | `settings.conf:133,880`, `settings/settings.go:369` |
| P2P Bootstrap | **9901** | `P2P_BOOTSTRAP_PORT` | — | `settings.conf:131` |
| Coinbase gRPC | **8093** | `COINBASE_GRPC_PORT` / `coinbase_grpcListenAddress=:8093` | `""` | `settings.conf:124,486`, `settings/settings.go:403` |
| Coinbase P2P | **9907** | `P2P_PORT_COINBASE` / `p2p_port_coinbase` | `9906` | `settings.conf:135,916`, `settings/settings.go:417` |
| Alert P2P | **9908** | `ALERT_P2P_PORT` | `9908` | `settings.conf:116`, `settings/settings.go:153` |
| Legacy gRPC | **8099** | `LEGACY_GRPC_PORT` / `legacy_grpcListenAddress=:8099` | `""` | `settings.conf:129,687`, `settings/settings.go:459` |
| Legacy HTTP | **8098** | `LEGACY_HTTP_PORT` / `legacy_httpListenAddress=:8098` | `""` | `settings.conf:130,699` |
| Health check | **8000** | `HEALTH_CHECK_PORT` / `health_check_httpListenAddress=:8000` | `:8000` | `settings.conf:126,603`, `settings/settings.go:56` |
| Profiler / pprof | **9091** | `PROFILE_PORT` / `profilerAddr=:9091` | `""` (disabled if empty) | `settings.conf:137,961`, `settings/settings.go:53` |
| Prometheus metrics | served on `profilerAddr` at path `/metrics` | path `""` (disabled) | — | `settings.conf:965`, `settings/settings.go:55`, `daemon/daemon_services.go:188` |
| Faucet HTTP | **8097** | `FAUCET_HTTP_PORT` / `faucet_httpListenAddress=:8097` | `""` | `settings.conf:125,575`, `settings/settings.go:492` |
| Kafka | **9092** | `KAFKA_PORT=9092` | `9092` | `settings.conf:64`, `settings/settings.go:109` |
| Aerospike | **3000** | `aerospike_port` | `3000` | `settings.conf:173`, `settings/settings.go:139` |
| PostgreSQL | **5432** | `POSTGRES_PORT=5432` | — | `settings.conf:136` |
| Jaeger UDP | **6831** | `JAEGER_PORT=6831` | — | `settings.conf:127` |
| Jaeger HTTP (OTLP) | **4318** | `JAEGER_PORT_HTTP=4318` | `http://localhost:4318` | `settings.conf:128`, `settings/settings.go:43` |

*\*Discrepancy: `settings.conf:141` defines `SUBTREE_VALIDATION_GRPC_PORT=8086` but `settings/settings.go:425` Go fallback is `:8089`. Conf wins. Similarly `P2P_GRPC_PORT` conf is 9904, Go fallback `:9906`. Conf wins.*

### Auth flags table

| Service | Auth mode | Default | Key | Source ref |
|---|---|---|---|---|
| JSON-RPC | HTTP Basic Auth (SHA256), two-tier admin + limited | Required when set; warns if missing | `rpc_user`, `rpc_pass`, `rpc_limit_user`, `rpc_limit_pass` | `settings/settings.go:480-483`, `services/rpc/Server.go:877-933`, `settings.conf:1023-1026` |
| JSON-RPC default credentials | `bitcoin`/`bitcoin` | **Plaintext defaults in conf** | same | `settings.conf:1024-1026` |
| JSON-RPC max clients | 1 (Go) / 3 (conf) | 3 | `rpc_max_clients` | `settings.conf:1022`, `settings/settings.go:484` |
| All gRPC services | TLS disabled by default | `0` (plain) | `security_level_grpc` | `settings/settings.go:64`, `settings.conf:1028` |
| All HTTP services | TLS disabled by default | `0` | `securityLevelHTTP` | `settings/settings.go:46`, `settings.conf:1032` |
| TLS cert/key files | `certs/teranode.crt` / `certs/teranode.key` | when `securityLevelHTTP > 0` | `server_certFile`, `server_keyFile` | `settings.conf:1034-1036`, `settings/settings.go:47-48` |
| gRPC admin API key | Bearer token | `"testkey"` (hard-coded dev value!) | `grpc_admin_api_key` | `settings.conf:597`, `settings/settings.go:66` |
| Asset HTTP response signing | Ed25519 over response | `false` | `asset_sign_http_responses` | `settings/settings.go:163`, `services/asset/httpimpl/http.go:147` |
| Kafka TLS | TLS for Kafka | `false` | `KAFKA_ENABLE_TLS`, `KAFKA_TLS_SKIP_VERIFY` | `settings.conf:87-89`, `settings/settings.go:125-126` |

### Feature flags table

| Flag | Default | Effect | Source ref |
|---|---|---|---|
| `network` | `mainnet` (bare); `regtest` for `dev`/`docker`; `teratestnet` for `teratestnet` | Selects Bitcoin network params | `settings.conf:778-794`, `settings/settings.go:17` |
| `use_cgo_verifier` | `true` | Inject BDK secp256k1 native verifier (faster) | `daemon/daemon_native.go:12`, `settings/settings.go:60` |
| `useLocalValidator` | `false` | In-process validator vs gRPC | `settings/settings.go:312`, `daemon/daemon_stores.go:142` |
| `blockassembly_disabled` | `false` | Disables Block Assembly entirely | `settings.conf:252-253`, `settings/settings.go:196` |
| `blockassembly_useDynamicSubtreeSize` | Go: `false`; conf: `true` | Dynamic subtree Merkle item count | `settings.conf:293`, `settings/settings.go:220` |
| `double_spend_window_millis` | `0` | DAH filter window in subtree processor (`0` = disabled) | `settings.conf:567`, `settings.go:31-32,216`, `services/blockassembly/subtreeprocessor/SubtreeProcessor.go:569` |
| `blockvalidation_fail_fast_validation` | Go: `true`; conf: `false` | Subtree validation uses txmeta cache only | `settings.conf:394`, `settings/settings.go:177` |
| `blockvalidation_optimistic_mining` | `true` (Go) | Allow block-assembly optimistic execution | `settings/settings.go:259` |
| `blockvalidation_useCatchupWhenBehind` | `false` | Enter catch-up when behind block assembly | `settings/settings.go:266` |
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
| `dashboard_enabled` | `true` | Enables dashboard UI via Asset centrifuge | `settings.conf:558`, `settings/settings.go:495` |
| `asset_centrifuge_disable` | `false` | Disable real-time notification server | `settings.conf:207`, `settings/settings.go:158` |
| `acceptnonstdoutputs` | `true` | Accept non-standard tx outputs | `settings/settings.go:89` |
| `excessiveblocksize` | conf: `10737418240` (10 GB); Go: `4294967296` (4 GB) | Max accepted block size | `settings.conf:571`, `settings/settings.go:71` |
| `blockmaxsize` | conf bare: `0` (unlimited) | Max mined block size | `settings.conf:364-366`, `settings/settings.go:22` |
| `minminingtxfee` | conf bare: `0.00000001` BSV/kB; `0` for `dev`/`docker`/`teratestnet` | Minimum fee rate | `settings.conf:771-775`, `settings/settings.go:76` |
| `p2p_dht_mode` | conf: `server`; local: `client` | DHT mode | `settings.conf:855`, `settings/settings.go:388` |
| `p2p_enable_mdns` | `false` | mDNS discovery | `settings/settings.go:392` |
| `p2p_allow_private_ips` | `false` | Connect to RFC1918 | `settings/settings.go:393` |
| `fsm_state_restore` | `false` | Restore FSM from snapshot | `settings.conf:584`, `settings/settings.go:233` |

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

1. **`http_sign_response` vs `asset_sign_http_responses` key mismatch.** `settings.conf:612` sets `http_sign_response=true` but Go reads `asset_sign_http_responses` (`settings/settings.go:163`, default `false`). Conf entry **silently ignored**. Effective `SignHTTPResponses=false` unless explicit override.
> **Discrepancy noted.** `settings.conf:612` sets `http_sign_response=true` but the Go code reads `asset_sign_http_responses` (`settings/settings.go:163`, default `false`). The `settings.conf` key is silently ignored; effective value is `SignHTTPResponses=false` unless explicitly overridden via env or `settings_local.conf`.

2. **`blockvalidation_fail_fast_validation` conf/code disagreement.** `settings.conf:394` is `false`; `settings/settings.go:177` Go fallback is `true`. Conf wins normally.
3. **`blockvalidation_optimistic_mining` absent from conf.** Go default `true` (`settings/settings.go:259`). Controlled only by env var or local.conf.
4. **`blockvalidation_useCatchupWhenBehind` absent from conf.** Go default `false` (`settings/settings.go:266`).
5. **`testmempoolaccept` completely absent** from RPC handler registry.
6. **`estimatefee` and `getmempoolinfo` stubbed** with `handleUnimplemented`.
7. **`grpc_admin_api_key` default is `"testkey"`** in `settings.conf:597` — plaintext dev credential. Production deployments must override.
8. **Profiler/Prometheus disabled by default** in operator context (`profilerAddr.operator` not set).
9. **`SUBTREE_VALIDATION_GRPC_PORT=8086` (conf) vs `:8089` (Go fallback)** — conf wins; node configured without conf would unexpectedly listen on 8089.


## Appendix B — Discovery methodology

See `docs/discovery-method.md` for the search strategy used by each
discovery agent and instructions for refreshing this document.
