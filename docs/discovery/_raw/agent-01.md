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
