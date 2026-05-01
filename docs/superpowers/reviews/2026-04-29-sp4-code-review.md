# SP4 — Transaction Generator: code review

**Reviewer:** Claude Code (Senior Code Reviewer hat)
**Date:** 2026-04-28
**Subject:** SP4 implementation in `internal/txgen/`, commits `ccacc24..a74ce3a` (7 SP4 commits after `sp3-complete`).
**Spec:** `docs/superpowers/specs/2026-04-29-sp4-txgen-design.md`
**Plan:** `docs/superpowers/plans/2026-04-29-sp4-txgen.md`
**Verdict:** **Approved with minor issues.** All critical invariants and definition-of-done gates hold. Ship it; tickets noted below for cleanup before SP10 polish.

---

## Critical (must fix)

None. All ten critical invariants from the review brief verify clean:

| # | Invariant | Status |
|---|---|---|
| 1 | All required files exist in `internal/txgen/` | PASS |
| 2 | All five script shapes round-trip through `bt.NewTxFromString` | PASS |
| 3 | Bootstrap returns wrapped `ErrNoWallet` for `-32601`, `-28`, "wallet" message | PASS — `bootstrap.go:75-83` |
| 4 | Coin selection greedy first-fit + dust absorption + `ErrInsufficientFunds` | PASS — `coinselect.go:12-43` |
| 5 | `TestFunder_ConcurrentAddUTXO` 100×10 race-clean | PASS — `-race` clean, balance asserts 1000 |
| 6 | `Env.TxGen` is `*txgen.Funder`, nil-safe; main builds funder only when WIF set | PASS — `internal/testrunner/types.go:65`, `cmd/teranode-acceptance/main.go:63-74` |
| 7 | No live network in any test | PASS — `scriptedRPC` / `fakeRPC` only |
| 8 | Test fixture WIF in separate sub-package; no production import of `testdata` | PASS — verified via grep |
| 9 | Coverage ≥80% on `internal/txgen/...` | PASS — **85.7%** (target 80) |
| 10 | All four done-checks (sp1..sp4) exit 0 | PASS — `./scripts/sp4-done-check.sh` exits 0 |

`make build lint test verify`, `gofmt -l internal/txgen/`, and `go vet ./internal/txgen/...` all clean.

---

## Important (should fix soon)

### I1. `BuildRequest.SpendUTXO` is declared but ignored

`types.go:30` documents `SpendUTXO *UTXO` as "optional explicit input (e.g. NEW-FR7 chain depth: spend a specific output)" — but `BuildP2PKH` never reads it. `SelectInputs` always greedy-picks. This is a deviation from spec §3 and harmless today (BuildChain's iterative `Confirm` happens to consume the prior change as the only available UTXO when started from a fresh funder), but it's a footgun for future callers who set the field expecting it to be honoured.

**Fix options:**
- Honour it: in `BuildP2PKH`, if `req.SpendUTXO != nil`, skip selection and use that UTXO directly (computing fee/change against it).
- Or remove the field and document the workaround (call `Reset()` then `AddUTXO()`).

I'd take option 1 — it's six lines of code and matches the design intent for chain-depth tests where you must spend a *specific* prior output.

### I2. `Funder.wif` field stored but never read

`funder.go:16` and `:45` keep `wifStr` as `f.wif` but no method ever consults it. Dead state. Either expose it (low value — exposing a WIF is a security smell) or drop the field. Recommendation: drop.

### I3. Reported `fee` is a lie when dust absorbed

`coinselect.go:34-37`: when change is dust, the function returns `fee = feeNoChange` and `change = 0`. The actual satoshis the miner receives is `acc - target`, which can be substantially higher than `feeNoChange` (because the dust silently went to fees). Callers that log or audit fees will see a misleading number.

**Fix:** when absorbing dust, set `fee = acc - target`, not `feeNoChange`. The current test asserts only `change == 0`, so the fix won't break tests, but add an assertion that `fee >= feeNoChange` to lock the contract.

### I4. `BuildResult.Inputs` returned by `BuildP2PKH` reference selected UTXOs by their **original** TxID — but `Funder.MarkSpent` matches on TxID+Vout. Confusingly, `BuildChain` calls `Confirm(res.Inputs, res.Change)` immediately, before broadcast. If a test wants to compare against the broadcast tx's inputs, it can — but the in-memory state-flip happens before any RPC dance. Not a bug; document explicitly that `Confirm` is "speculative-confirm for testing" so a future reader doesn't assume it requires a broadcast first.

### I5. `MarkSpent` slice aliasing is correct but fragile

`coinselect.go:50` does `out := f.state.utxos[:0]` then iterates `f.state.utxos` while appending into `out`. The two slices share the backing array; the algorithm is safe because `len(out) <= i` always, but the pattern is hard to audit and one careless future edit breaks it. Replace with `out := make([]UTXO, 0, len(f.state.utxos))`. The allocation cost is negligible for the funder's scale.

---

## Minor / suggestions

### M1. `script.go:21` re-parses the address it just decoded

`P2PKHScript`:
```go
a, err := bscript.NewAddressFromString(addr)   // parses the address
...
s, err := bscript.NewP2PKHFromAddress(a.AddressString)   // re-parses the string
```
Throws away `*Address` except its string, then `NewP2PKHFromAddress` re-decodes. If the libsv API has a `bscript.NewP2PKHFromAddressObject(*Address)`, use it; otherwise the redundancy is cosmetic.

### M2. `NewFunder` hardcodes mainnet (`true`) for address derivation

`funder.go:36`: `bscript.NewAddressFromPublicKey(pk, true)`. The spec mentions regtest/testnet WIFs being supported. For SP4 tests this doesn't matter (the public-test-vector WIF is a mainnet privkey=1 fixture), but if SP5+ ever loads a testnet WIF, the address bytes will mismatch. Either:
- Parse the WIF prefix to choose mainnet vs testnet automatically (libsv's `wif.DecodeWIF` returns the network), or
- Plumb a network parameter through `NewFunder`.

Defer to SP10 if regtest/testnet support is genuinely needed.

### M3. `Funder.PrivateKey()` exposes raw `*bec.PrivateKey`

Necessary for the Builder, but document the security implication: any code with a `*Funder` reference can sign arbitrary transactions. The spec calls this out (decision-locked). A doc comment on `PrivateKey()` saying "exposed for the in-package Builder; do not pass `*Funder` across trust boundaries" would be one line of text and worth it.

### M4. Test fixture `FundedUTXO` script comment is misleading

`testdata/fixtures.go:30-33`: comment says "the funder's key won't validate, but tests that inject this UTXO and then try to build transactions spending it will use the funder's own address script." The second clause is confusing — the *signer* (libsv unlocker) doesn't actually validate the locking script against the key, it just fills an unlocking script. The comment could be rewritten as "tests that need signature-validity must use a UTXO whose locking script encodes the funder's address — see `TestBuildP2PKH_realKeySigns`."

### M5. `bootstrap.go` parses `value: float64` then converts to `uint64(v.Value * 1e8)`

Floating-point sat conversion can lose precision for large values. `1.5 * 1e8 == 150_000_000` is exact, but `2.1 * 1e8 == 209999999.99999997` rounds to `209_999_999`. Bitcoin Core encodes amounts as 64-bit fixed-point and the JSON-RPC text representation is decimal; safer to parse `value` as `json.Number` and use `strconv` to scale by 1e8 with integer math. Low risk for SP4 (synthetic test responses use clean numbers), but worth noting before any production path leans on `Bootstrap`.

### M6. `bscript.Op0` for OP_FALSE in OP_RETURN

`script.go:34` uses `bscript.Op0` (0x00). Conventionally OP_RETURN data outputs are written `OP_FALSE OP_RETURN <data>` — `Op0` *is* OP_FALSE in BSV consensus, so byte-correct. Just worth a doc-comment for readers who'd expect `OpFALSE`.

### M7. `BuildOpReturn` puts the OP_RETURN as **first** output

That choice is consistent across `BuildP2MS`, `BuildP2SH`, `BuildOpReturn` (all prepend the special script before passing to `BuildP2PKH`). Functionally fine. Just document so callers don't pass `req.Outputs[0]` expecting it to be index 0 in the resulting tx — it'll be at index 1.

### M8. `TestBootstrap_walletNotEnabled` overlaps semantics

`-28` ("Wallet not loaded") triggers `ErrNoWallet` because `isMethodNotFound` checks `rpcErr.Code == -32`. That's a typo — it should be `-28` or the special bsvjson "wallet" code. As written, the function returns `true` for any code in `(-33, -32, -32601)`-style range mismatch. Inspecting the code: it specifically checks `== -32`, which doesn't match `-28`. So *why* does the test pass? Because the message contains "wallet" (`"Wallet not loaded"`) and `strings.Contains(strings.ToLower(rpcErr.Message), "wallet")` short-circuits to true. The `-32` branch is effectively dead. Either delete the `-32` case (it's not a real bsvjson code) or replace with the actual wallet-disabled code.

---

## Strengths (what was done well)

1. **Spec coverage is complete.** Every section of the design spec §3–§7 has a concrete, tested implementation. The plan was followed faithfully task-by-task.
2. **Test design is excellent.** Use of `scriptedRPC` for sequencing two-call flows (sendtoaddress → getrawtransaction) keeps tests deterministic and brittle in the right way (assert on call order, fail loudly on mismatch).
3. **Error sentinels with `errors.Is` plumbing** are correct throughout: `fmt.Errorf("%w: ...", ErrInsufficientFunds)` and `("%w: ...", ErrNoWallet)` round-trip through `errors.Is` at the test boundary.
4. **The `FR-7` chain-depth-25 test is real.** It's not a smoke test — it iterates 25 dependent builds, each `Confirm`'d into the funder's state, and asserts the chain shape. A future regression in fee estimation or change handling would surface here immediately.
5. **Concurrency story is honest.** A single mutex protecting both reads and writes; `snapshotUTXOs` returns a copy. The 100×10 stress test exercises the actual hot path and would catch any future "simplification" that drops the lock.
6. **Coverage is meaningfully high (85.7%) without being padded.** The uncovered lines are mostly error-branch returns from libsv API calls that can't be triggered from tests without dependency injection (e.g. `AppendOpcodes` only fails on malformed opcodes).
7. **`Env.TxGen` nil-safety lands cleanly** — main.go constructs only when `Funding.WIF` is set, and the type is a concrete `*txgen.Funder` rather than the SP1 placeholder interface, so SP5+ tests that need it can do `if env.TxGen == nil { skip }` without an interface gymnastics.
8. **Public-test-vector WIF (privkey=1)** is a clean choice. It's the most-published private key on the planet; nobody can mistake it for production credentials. Comment in `fixtures.go` makes the warning explicit.

---

## Spec coverage cross-check

| Spec § | Implementation | Verified |
|---|---|---|
| §3 architecture (Funder/Builder co-located, decoupled) | `funder.go` + `builder.go`, `Funder.Builder()` constructor returns the bridge | YES |
| §4 Bootstrap flow (sendtoaddress + getrawtransaction) | `bootstrap.go:25,44` | YES |
| §5 Fee handling (`EstimateSize`, `ComputeFee`) | `fee.go` with byte-level constants | YES |
| §6 Coin selection (greedy first-fit + dust absorption) | `coinselect.go:12-43` | YES |
| §7.1 BuildP2PKH | `builder.go:29` | YES |
| §7.2 BuildP2MS | `builder.go:105` | YES (paying TO multisig; spending OUT-OF-SCOPE per spec) |
| §7.3 BuildP2SH | `builder.go:119` | YES |
| §7.4 BuildOpReturn (100 KB cap) | `builder.go:90` + `script.go:30` | YES |
| §7.5 BuildChain (FR-7, depth ≥1) | `builder.go:134` + `TestBuildChain_depth25` | YES |
| §9 Wiring `Env.TxGen` | `internal/testrunner/types.go:65`, `cmd/teranode-acceptance/main.go:63-78` | YES |
| §10 Definition of done | `scripts/sp4-done-check.sh` exits 0 | YES |

No gaps.

---

## Practical sanity checks (commanded)

```
$ gofmt -l internal/txgen/
(nothing — clean)
$ go vet ./internal/txgen/...
(no output — clean)
$ go test -race -coverprofile=/tmp/sp4cov.out ./internal/txgen/...
ok  github.com/bsv-blockchain/node-validation/internal/txgen  coverage: 85.7%
$ ./scripts/sp4-done-check.sh
==> SP4 done-check passed.
$ grep -rn "TODO\|FIXME\|XXX" internal/txgen/ (excluding _test.go)
(nothing)
$ grep -rn "testdata" cmd/ internal/ | grep -v _test.go | grep -v "internal/txgen/testdata"
(only unrelated config testdata directories — no production txgen-testdata import)
```

All four done-checks (sp1, sp2, sp3, sp4) exit 0 in the same run.

---

## Recommendation

**Approve.** Tag `sp4-complete` and proceed to SP5. Track I1 (`SpendUTXO` honouring) and I3 (dust-fee reporting) as small follow-ups; both are straightforward and won't affect the SP5+ tests since callers don't currently rely on either field. I2 (dead `wif`) and I5 (alias slice) are housekeeping for SP10 polish.

The implementation is faithful to the design, honest about its limits (signing is exercised but multi-sig spending is explicitly out of scope, as the spec said), and the test design is the kind that catches regressions rather than padding coverage. Coverage at 85.7% with the *right* lines covered is genuinely better than 95% with smoke tests.

One minor insult, as is house style: the `bscript.Op1 - 1 + byte(n)` arithmetic in `script.go:97` works because the BSV opcodes happen to be contiguous, but reading it cold gave me a brief moment of "wait, are we sure" — comments earn their keep here.
