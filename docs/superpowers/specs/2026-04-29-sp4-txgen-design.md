# SP4 — Transaction Generator (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP4 / 11 — Transaction Generator
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-29
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1 (skeleton), SP2 (discovery), SP3 (clients — `svnode.RPCClient.Call` for `sendtoaddress`)
**Parallel to:** SP4-DOCKER (compose stack)
**Status:** awaiting user review

---

## 1. Purpose

Build `internal/txgen/`: a typed Go package that produces signed BSV transactions on demand for SP5+ tests. Bootstraps initial UTXOs by calling `sendtoaddress` against an SV Node with wallet support. Maintains a thread-safe in-memory UTXO set, supports all common script shapes (P2PKH, P2MS multi-sig, P2SH, OP_RETURN data carrier), and computes fees from sat/byte rates.

SP4 is unit-tests-only. Real broadcast behaviour is exercised in SP5+ against the SP4-DOCKER stack.

## 2. Scope

### In scope

- `internal/txgen/funder.go` — wallet state: WIF, key, address, UTXO set, balance tracking, mutex.
- `internal/txgen/builder.go` — request → signed transaction (any of the supported shapes).
- `internal/txgen/coinselect.go` — greedy first-fit coin selection.
- `internal/txgen/fee.go` — size estimation + sat/byte → fee math.
- `internal/txgen/bootstrap.go` — wraps `svnode.RPCClient.Call("sendtoaddress", ...)` to obtain initial UTXOs.
- `internal/txgen/types.go` — public types (`UTXO`, `Output`, `BuildRequest`, `BuildResult`, etc.).
- Comprehensive unit tests using `libsv/go-bt/v2` and `libsv/go-bk` with injected UTXOs (no live network).
- Wiring into `Env.TxGen` (currently a `TxGenerator` interface placeholder from SP1).

### Out of scope

- Live broadcast — done by SP5+ tests via `teranode.RPCClient.SendRawTransaction`.
- Tracking confirmation status across reorgs — funder operates at mempool resolution.
- Watch-tower duties: rebroadcast on drop, fee bump, RBF (BSV doesn't do RBF anyway).
- Persistent UTXO set across runs — funder is in-memory; tests start fresh.

## 3. Architecture

```
test (SP5+)
    │
    │  txgen.NewFunder(svnode.RPC, fundingWIF, address, logger)
    ▼
┌──────────────────────┐         calls sendtoaddress
│      Funder          │────────────────────────────────────────►  SV Node RPC
│  - mu sync.Mutex     │
│  - utxos []UTXO      │           returns txid
│  - key *bsvec.PrivKey│        ◄────────────────────────────────
│  - address string    │
└──────────┬───────────┘
           │
           │ funder.Bootstrap(ctx, satoshis) registers a new UTXO
           │ funder.SelectInputs(amount) returns inputs + change
           │
           ▼
┌──────────────────────┐
│      Builder         │  Build(req) → signed hex tx, txid, change UTXO
│  - Funder            │
└──────────────────────┘
           │
           │  test broadcasts via teranode.RPCClient.SendRawTransaction(hex)
           │  on success: funder.Confirm(txid) marks inputs spent + adds change
           ▼
       (SP3 clients)
```

`Funder` and `Builder` are decoupled but co-located. Tests typically `funder, _ := txgen.NewFunder(...)` then `builder := funder.Builder()`.

### Key types

```go
type UTXO struct {
    TxID       chainhash.Hash
    Vout       uint32
    Satoshis   uint64
    Script     []byte
    BlockHeight int64    // 0 = unconfirmed
}

type Output struct {
    Script      []byte    // raw locking script (caller builds via libsv/go-bk)
    Satoshis    uint64
    Description string    // diagnostic only
}

type BuildRequest struct {
    Outputs   []Output
    FeeRate   uint64    // sat/kB
    SpendUTXO *UTXO     // optional explicit input (e.g. NEW-FR7 chain depth: spend a specific output)
}

type BuildResult struct {
    TxID    chainhash.Hash
    HexTx   string
    Inputs  []UTXO
    Change  *UTXO       // nil if no change output (round number)
}

type Funder struct {
    rpc     *svnode.RPCClient   // for Bootstrap
    wif     string
    key     *bsvec.PrivateKey
    address string
    mu      sync.Mutex
    utxos   []UTXO
    logger  *slog.Logger
}

type Builder struct {
    funder *Funder
}
```

### Public API surface

```go
func NewFunder(rpc *svnode.RPCClient, wif string, logger *slog.Logger) (*Funder, error)
func (f *Funder) Address() string
func (f *Funder) Balance() uint64
func (f *Funder) Bootstrap(ctx context.Context, satoshis uint64) error
func (f *Funder) AddUTXO(u UTXO)                 // for tests / direct injection
func (f *Funder) SelectInputs(target uint64) ([]UTXO, error)
func (f *Funder) Confirm(txid chainhash.Hash, change *UTXO)  // mark inputs spent + add change
func (f *Funder) Reset()                          // for tests
func (f *Funder) Builder() *Builder

func (b *Builder) BuildP2PKH(req BuildRequest) (BuildResult, error)
func (b *Builder) BuildP2MS(req BuildRequest, m int, pubkeys [][]byte) (BuildResult, error)
func (b *Builder) BuildP2SH(req BuildRequest, redeemScript []byte) (BuildResult, error)
func (b *Builder) BuildOpReturn(req BuildRequest, dataPayload []byte) (BuildResult, error)
func (b *Builder) BuildChain(req BuildRequest, depth int) ([]BuildResult, error)  // FR-7

// Convenience helpers exposed for test ergonomics:
func P2PKHScript(addr string) ([]byte, error)
func OpReturnScript(data []byte) []byte
func MultisigScript(m int, pubkeys [][]byte) []byte
func P2SHScript(redeemScript []byte) []byte
```

## 4. Bootstrap flow

`Funder.Bootstrap(ctx, satoshis)`:

1. Calls `f.rpc.Call(ctx, "sendtoaddress", []any{f.address, float64(satoshis)/1e8}, &txid)` against SV Node.
2. Calls `f.rpc.Call(ctx, "getrawtransaction", []any{txid, 1}, &decoded)` to find the output paying to `f.address`.
3. Constructs a `UTXO` with `BlockHeight: 0`, adds to the funder's set under mutex.
4. Returns the new UTXO.

If SV Node lacks wallet support (`sendtoaddress` returns `-32601 method not found` or similar), `Bootstrap` returns a clean error so SP5+ tests can skip with a documented reason.

For tests that don't have SV Node wallet (most unit tests), `funder.AddUTXO(...)` lets a test inject a fake UTXO directly.

## 5. Fee handling

```go
// fee.go
func EstimateSize(numInputs int, outputs []Output) int
func ComputeFee(numInputs int, outputs []Output, satPerKB uint64) uint64
```

Size estimate per input: 148 bytes for P2PKH (typical) or 73*m + n*34 for m-of-n multisig signatures (rough; SP9 can refine if PERF-1 wants tight numbers).

Per output: `8 + 1 + len(script)` bytes (8 for value, 1 for script-length VarInt under 253).

Plus 10 bytes overhead (version, input count VarInt, output count VarInt, locktime).

Fee = `(estimatedSize * satPerKB) / 1000`, rounded up.

## 6. Coin selection

`coinselect.go` implements greedy first-fit:

1. Iterate `f.utxos` in insertion order.
2. Accumulate inputs until `sum >= target + estimated_fee`.
3. If we exceed target by more than dust threshold, emit a change output.
4. Dust threshold: 546 sats (BSV convention).

If the available balance is insufficient, return `ErrInsufficientFunds`.

## 7. Builder operations

### 7.1 `BuildP2PKH`

Standard pay-to-pubkey-hash. Caller supplies destination address(es) via `Outputs[i].Script`. Signs each input with `key.Sign()` over the BIP-143-style sighash (BSV uses BIP-143 fork-id sighash since Magnetic anchor — `libsv/go-bt/v2` handles this).

### 7.2 `BuildP2MS`

Bare multisig (`OP_M <pk1> <pk2> ... <pkN> OP_N OP_CHECKMULTISIG`). For inputs spending an existing P2MS UTXO, the funder must already know which keys to sign with (out of scope for SP4 — caller injects pre-signed UTXOs in such tests). For SP4 the typical use is constructing multisig outputs paying *to* a multisig script; spending those is not exercised in the in-scope tests (PC-2 / IBD-2 spending complex scripts come from fixtures, not from txgen).

### 7.3 `BuildP2SH`

Standard pay-to-script-hash (`OP_HASH160 <hash> OP_EQUAL`). Input signing requires the redeem script. Caller supplies via `BuildP2SH(req, redeemScript)`.

### 7.4 `BuildOpReturn`

OP_RETURN data carrier. Output script: `OP_FALSE OP_RETURN <push data>`. Caller-supplied data; SP4 caps at 100 KB to keep tests bounded.

### 7.5 `BuildChain` (NEW-FR7)

Builds a chain of `depth` dependent transactions: tx1 spends a starting UTXO, tx2 spends tx1's change output, ..., txN spends txN-1's change. Each tx is independently signable (the funder's private key controls every change output). Returns `[]BuildResult` in order.

After broadcast, the test calls `funder.Confirm(...)` for each in order, simulating the chain progressing.

## 8. Verification & testing strategy

All unit tests, no live network. Coverage target ≥80%.

### 8.1 Test harness

A test fixture WIF — clearly marked "do not use for real funds" — provides a deterministic key. The fixture file at `internal/txgen/testdata/fixtures.go` exports:

```go
package testdata

const TestWIFMainnet = "L1aW4aubDFB7yfras2S1mN3bqg9nwySY8nkoLmJebSLD5BWv3ENZ" // mainnet, never used
const TestWIFRegtest = "cVjzvdHGfQDtBT2YjMxnAmfgYqf6XwHsLY1xBUtJqDk9pKr8gNRk" // regtest

// FundedUTXO returns a synthetic UTXO controlled by TestWIFRegtest with the given satoshis.
func FundedUTXO(satoshis uint64) txgen.UTXO { ... }
```

### 8.2 Test cases

| Test | What it pins down |
|---|---|
| `TestNewFunder_validWIF` | Constructor with regtest WIF; address derives correctly |
| `TestNewFunder_badWIF` | Returns error for malformed WIF |
| `TestSelectInputs_singleUTXOSufficient` | Greedy selection picks first match |
| `TestSelectInputs_multipleUTXOsRequired` | Selection accumulates two UTXOs |
| `TestSelectInputs_insufficientFunds` | Returns ErrInsufficientFunds |
| `TestSelectInputs_threadSafe` | Run 100 concurrent SelectInputs; no double-spend |
| `TestBuildP2PKH_singleOutputCorrectFee` | Build, parse with go-bt, verify fee == request.FeeRate * size |
| `TestBuildP2PKH_changeOutputCreated` | Excess funds → change output paying back to f.address |
| `TestBuildP2PKH_dustChangeAddedToFee` | Change < 546 sats absorbed into fee |
| `TestBuildOpReturn_dataPreserved` | Round-trip: data in, parse output, bytes match |
| `TestBuildOpReturn_dataTooLarge` | >100 KB → error |
| `TestBuildP2MS_2of3` | Output script matches `OP_2 <pk1> <pk2> <pk3> OP_3 OP_CHECKMULTISIG` |
| `TestBuildP2SH_redeemScriptHashed` | Output script is `OP_HASH160 <ripemd160(sha256(redeem))> OP_EQUAL` |
| `TestBuildChain_25depth` | NEW-FR7's depth=25 case; each tx valid, each spends prior change |
| `TestConfirm_marksSpent` | After Confirm, inputs no longer in utxos; change is |
| `TestSign_verifies` | Signed tx passes script verification with `bscript.VerifySignature` (libsv) |
| `TestBootstrap_happyPath` | Mock SV Node RPC → returns txid; UTXO added to set |
| `TestBootstrap_methodNotFound` | Mock SV Node returns -32601 → clean error containing "wallet not available" |

### 8.3 SP4 done-check

```bash
make build lint test verify
./scripts/sp{1,2,3}-done-check.sh
go test -race -coverprofile=cov.out ./internal/txgen/...
total=$(go tool cover -func=cov.out | tail -1 | awk '{print $3}' | tr -d %)
awk -v t="$total" -v th=80 'BEGIN { exit !(t >= th) }'
echo "SP4 done-check passed."
```

## 9. Wiring into Env

`internal/testrunner/types.go` `TxGenerator` interface gets a typed concrete:

```go
type Env struct {
    // ...
    Teranode *teranode.Clients
    SVNode   *svnode.Clients
    TxGen    *txgen.Funder    // changed from interface placeholder
}
```

`cmd/teranode-acceptance/main.go` constructs after svnode clients:

```go
var txGen *txgen.Funder
if cfg.Funding.WIF != "" && svnodeClients != nil && svnodeClients.RPC != nil {
    txGen, err = txgen.NewFunder(svnodeClients.RPC, cfg.Funding.WIF, logger)
    if err != nil {
        fmt.Fprintln(stderr, err)
        return 4
    }
}
env.TxGen = txGen
```

When WIF is empty, `env.TxGen` is nil; tests skip with `"funding WIF not configured"`. Cleanly nil-safe.

## 10. Definition of done

- All files in `internal/txgen/` exist with `*_test.go` siblings.
- `make build lint test verify` exits 0.
- SP{1,2,3} done-checks still pass.
- SP4 done-check exits 0; coverage ≥80%.
- `Env.TxGen` is typed `*txgen.Funder`; main.go constructs from config.
- Code review (`superpowers:code-reviewer`) approves.

## 11. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | `sendtoaddress` not available on SV Node in some test environments | Bootstrap returns a clean "wallet not available" error; tests that need txgen skip; SP4-DOCKER ensures wallet is enabled in compose |
| B | `libsv/go-bt/v2` BIP-143 sighash subtleties for P2SH/P2MS | Tests verify signed txs round-trip through `bscript.VerifySignature` |
| C | Coin selection is greedy not optimal; could fragment UTXOs | YAGNI for SP4; PERF-1 may revisit if it affects throughput |
| D | OP_RETURN size cap at 100 KB is arbitrary | Documented; INTER-2 / NEW-FR7 don't push that limit |
| E | Concurrent funders sharing the same WIF would double-spend | One funder per WIF; documented; `TestSelectInputs_threadSafe` covers single-funder concurrent use |
| F | Test fixture WIF ends up in production by accident | WIF is only in `testdata/fixtures.go` package, never imported by `cmd/`; lint check could enforce — defer to SP10 |

## 12. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Bootstrap via SV Node `sendtoaddress` | per user (Q1) |
| 2 | All tx types: P2PKH, P2MS, P2SH, OP_RETURN | per user (Q2) |
| 3 | Unit tests only; live coverage in SP5+ via SP4-DOCKER | per user (Q3) |
| 4 | Greedy first-fit coin selection | drafter — YAGNI |
| 5 | OP_RETURN 100 KB cap | drafter — bounded for tests |
| 6 | Funder + Builder co-located, decoupled | drafter |
| 7 | Test fixture WIF kept in `internal/txgen/testdata/` | drafter |
| 8 | `Env.TxGen` becomes `*txgen.Funder` (concrete pointer, nil-safe) | drafter, mirrors SP3 pattern |
| 9 | `chainhash.Hash` from libsv/go-bt for txids — already pulled by SP3 transitively | drafter |
| 10 | SP4-DOCKER tracked as separate sub-project; SP4 unaffected | per user |

## 13. Out-of-scope reminders

SP4 produces a signed-transaction factory. It does **not**:
- Broadcast.
- Watch the network.
- Persist state across runs.
- Manage multiple funding wallets per process.
- Implement transaction fee bumping (BSV doesn't have RBF).
- Handle reorgs.
