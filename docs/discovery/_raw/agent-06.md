## 7. Extended transaction format

### Summary

**Present: `true`.**

Teranode implements the **Transaction Extended Format (TEF)**, specified in **BIP-239** (authored by Simon Ordish and Siggi Oskarsson, Status: Proposal, 2022-11-09). The spec ships in-tree at `docs/misc/BIP-239.md`. The format is fully implemented via `github.com/bsv-blockchain/go-bt/v2` (v2.5.0, `go.mod:17`). Every production submission path — gRPC `ProcessTransaction`, HTTP `POST /tx`, batch `POST /txs` — accepts both standard and extended formats. The validator auto-extends standard-format transactions by fetching missing UTXO data from the internal store; the proto comment ("must be extended") is aspirational, not enforced at the wire boundary.

### Findings table

| Field | Description | Source ref |
|---|---|---|
| **EF marker** | 6-byte `0x00 0x00 0x00 0x00 0x00 0xEF` immediately after the 4-byte version field. Signals extended format. | `docs/misc/BIP-239.md:51`; `go-bt/v2@v2.5.0/tx.go:541` |
| **Recognition / parsing** | `bt.NewTxFromBytes()` reads version, then inputCount VarInt. If 0/0 it reads 4 more bytes; if `0xEF` (BE) it sets `tx.extended = true` and continues with real inputCount. | `go-bt/v2@v2.5.0/tx.go:142-162` |
| **`IsExtended()` predicate** | Returns true if `extended` flag set, **or** every input has non-nil `PreviousTxScript`. | `go-bt/v2@v2.5.0/tx.go:299-316` |
| **Extended input fields** | After `SequenceNumber` each input carries `PreviousTxSatoshis` (uint64 LE, 8 bytes) + VarInt script length + `PreviousTxScript`. | `docs/misc/BIP-239.md:71-82`; `go-bt/v2@v2.5.0/input.go:99-127` |
| **`ExtendedBytes()` serialiser** | Produces the BIP-239 byte sequence including marker. `SerializeBytes()` calls `ExtendedBytes()` iff `IsExtended()`, else falls back to standard `Bytes()`. | `go-bt/v2@v2.5.0/tx.go:379-393` |
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

The proto comment at `propagation_api.proto:23-25` says "must be extended" but `processTransactionInternal` (`services/propagation/Server.go:908-956`) has no `IsExtended()` guard. Comment is stale or aspirational.

### Gaps / ambiguities

1. **Proto comment vs code mismatch** — `propagation_api.proto:24` and generated stub say "must be extended" but no runtime enforcement. CLIENT-2 should not rely on rejection of non-extended.
2. **No feature flag / build tag.** Always active; go-bt always parses both.
3. **Round-trip storage is non-extended.** Asset Service `GET /tx/{hash}` returns non-extended bytes. No endpoint round-trips EF verbatim.
4. **Coinbase cannot be extended.** Legacy netsync skips coinbase extension (`handle_block.go:717`). CLIENT-2 should use non-coinbase only.
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
- gRPC (Propagation): `ProcessTransaction` with `ProcessTransactionRequest{Tx: tx.ExtendedBytes()}` (`propagation_api.proto:25`).
- HTTP (Validator, internal): `POST /tx` — same byte format, fallback for large txs (`services/validator/Server.go:850`).

**Round-trip:** Not retrievable as extended. CLIENT-2 should verify acceptance on submission (HTTP 200 / gRPC OK) rather than expect retrieved bytes to carry the EF marker.

**Test pattern** (from `test/e2e/daemon/ready/smoke_test.go:116`):
```go
txHex := hex.EncodeToString(newTx.ExtendedBytes())
// POST txHex to propagation /tx, expect 200
```
