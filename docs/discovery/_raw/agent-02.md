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
