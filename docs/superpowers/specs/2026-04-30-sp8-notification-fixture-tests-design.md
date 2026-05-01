# SP8 — Notification + Fixture Tests (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP8 / 11 — CLIENT-1, CLIENT-3, PC-2, IBD-2
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-30
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1–SP7
**Status:** awaiting user review

---

## 1. Purpose

Land four Critical tests, three of which require substantial test-fixture work (PC-2: ≥30 historical-script fixtures; IBD-2: ≥10 historical-UTXO fixtures) plus a notification-stream stress test (CLIENT-3) and a 1-hour observation test (CLIENT-1).

After SP8, 6 of 8 Critical rows have measurements. Remaining: PC-1 (7-day parallel observation) and INTER-1 (14-day mixed-network) — both SP9 territory.

## 2. Scope

### In scope

- `tests/client1.go` — CLIENT-1 (Critical): connect, subscribe, broadcast 50 txs, force disconnect + reconnect, verify catch-up.
- `tests/client3.go` — CLIENT-3 (Critical): 500-tx controlled-order broadcast, verify all delivered, ordered, dependent-tx causal order, reconnect catch-up.
- `tests/pc2.go` — PC-2 (Critical): load ≥30 fixture txs, submit to both backends, compare accept/reject via `internal/compare`.
- `tests/ibd2.go` — IBD-2 (Critical): load ≥10 UTXO-spend fixture txs, same comparison method.
- `tests/testdata/historical_scripts.yaml` — ≥30 PC-2 fixtures.
- `tests/testdata/historical_utxos.yaml` — ≥10 IBD-2 fixtures.
- `cmd/gen-fixtures/` — small Go tool that generates the two fixture files programmatically. Run via `make gen-fixtures`.
- `internal/teranode/notifications.go` — small extension if needed: a way to force-close the connection from a test. Likely the existing `Close()` is enough.
- `cmd/teranode-acceptance/register.go` — register all 16 tests.
- `scripts/sp8-done-check.sh`.

### Out of scope

- PC-1 7-day parallel observation — SP9.
- INTER-1 14-day mixed-network — SP9.
- PERF-1 throughput ramp — SP9.
- Real-testnet fixture sourcing — synthetic regtest fixtures per Q1=A; SP10 may add a `funding.testnet_fixtures_path` path.
- IBD-1 — already DOCUMENTATION_REVIEW (excluded from automated tests per matrix).
- Reaching above 30 PC-2 fixtures — SP10 polish if more depth is wanted.

## 3. Architecture

```
suite.Run(ctx)
    │
    ├── tests.RunCLIENT1     subscribe + broadcast 50 + force reconnect + verify
    ├── tests.RunCLIENT3     500-tx ordered broadcast; verify notification stream
    ├── tests.RunPC2         load 30+ fixtures; submit each to both backends; compare
    └── tests.RunIBD2        load 10+ UTXO-spend fixtures; submit; compare
```

Each follows the SP5/SP6/SP7 shape (verbatim source-plan comment block, AcceptanceChecks, deriveStatus).

### 3.1 Fixture generator (`cmd/gen-fixtures/`)

Two reasons to generate programmatically rather than hand-author:
- Reproducibility — anyone with the repo can regenerate the fixtures and verify them.
- Maintenance — adding a 31st PC-2 fixture is a one-line code change, not a hex-literal hand-edit.

Tool layout:

```go
// cmd/gen-fixtures/main.go
//
//   $ ./bin/gen-fixtures --out tests/testdata/
//
// Writes historical_scripts.yaml (PC-2) and historical_utxos.yaml (IBD-2)
// using a fixed regtest-network WIF + deterministic UTXO sources so the
// fixtures are byte-identical across runs.

package main

func main() {
    // ... seed a deterministic key
    // ... generate 30+ PC-2 fixtures (6 per 5 categories)
    // ... generate 10+ IBD-2 fixtures
    // ... write YAML
}
```

Each fixture entry looks like:

```yaml
- id: pc2-p2sh-001
  category: complex-p2sh
  description: "P2SH wrapping a 2-of-3 multisig with redeemScript at exactly 520 bytes"
  hex_tx: "0100000001..."
  expected_valid: true
  expected_category: "ACCEPTED"
  provenance: "synthetic SP8 fixture; constructed via cmd/gen-fixtures"
  notes: "exercises the BSV 520-byte redeemScript boundary"
```

The fixture YAML is committed to the repo. The generator is deterministic — running `make gen-fixtures` and committing produces no diff if the upstream fixture logic hasn't changed. CI gate: `make verify` runs `gen-fixtures` then `git diff --exit-code` on the testdata files.

### 3.2 PC-2 fixture categories (30+ fixtures)

Six per category × five categories = 30 base. Categories:

1. **Complex P2SH** (6 fixtures):
   - 2-of-3 multisig with redeemScript at 80 / 200 / 520 byte sizes.
   - Edge case: redeemScript with OP_RETURN inside (always-fail).
   - Edge case: redeemScript with non-canonical signatures.
   - P2SH-of-P2SH (nested).

2. **Disabled / restricted opcodes** (6 fixtures):
   - OP_VER (now invalid, always-fail).
   - OP_RESERVED, OP_RESERVED1, OP_RESERVED2 — fail.
   - OP_VERIF / OP_VERNOTIF — fail in branch.
   - OP_RETURN immediately (script fails).

3. **CLEANSTACK violations** (6 fixtures):
   - Script leaves an extra item on stack (post-Genesis: should fail under MANDATORY flags).
   - Script leaves zero items (also fails).
   - Script leaves item with non-canonical-true value.
   - Variations.

4. **MINIMALDATA boundaries** (6 fixtures):
   - Push-data using OP_PUSHDATA1 for ≤75-byte data (non-minimal).
   - Push-data using OP_PUSHDATA2 for ≤255-byte data (non-minimal).
   - Push-data with extra leading zero bytes.
   - Variations.

5. **Historical malleability** (6 fixtures):
   - Sighash with extra OP_NOP padding (post-Genesis: ineffective).
   - Sighash without `SIGHASH_FORKID` (BSV-specific).
   - Variations.

The generator builds these via SP4 builder + raw script construction.

### 3.3 IBD-2 fixture categories (10+ fixtures)

Each fixture is a UTXO-spend tx exercising one of:

1. P2PKH spend with extra-witness-data (CLEANSTACK violation).
2. P2SH spend revealing a 1-of-1 multisig redeem.
3. P2SH spend with mismatched redeem-hash (always-fail).
4. P2MS spend with too-few signatures.
5. P2PKH spend with non-canonical signature DER encoding.
6. Coinbase spend before maturity (always-fail in regtest under the 100-block maturity rule).
7. Spend with locktime in the future (sequence != UINT32_MAX → invalid).
8. Spend with negative satoshis output (always-fail).
9. Spend creating dust output (≤ 546 sat).
10. Spend with input-amount-< output-amount (negative fee, always-fail).

Generator iterates these 10 cases plus optionally adds 5 more variations.

### 3.4 PC-2 / IBD-2 test shape

```go
func RunPC2(ctx context.Context, env *testrunner.Env) testrunner.Result {
    res := testrunner.Result{ID: "PC-2", Severity: matrix.SeverityCritical, ...}
    if env.Teranode == nil || env.Teranode.RPC == nil ||
       env.SVNode == nil || env.SVNode.RPC == nil { return skipMissing(res, ...) }

    fixtures, err := loadFixtures("tests/testdata/historical_scripts.yaml")
    if err != nil { return errorResult(res, err) }
    res.Observations["fixture_count"] = len(fixtures)

    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "≥30 PC-2 fixtures present (per source plan PC-2)",
        len(fixtures) >= 30,
        fmt.Sprintf("loaded=%d", len(fixtures)),
    ))

    matched := 0
    perCategory := map[string]struct{ pass, fail int }{}
    for _, f := range fixtures {
        _, terr := env.Teranode.RPC.SendRawTransaction(ctx, f.HexTx)
        _, serr := env.SVNode.RPC.SendRawTransaction(ctx, f.HexTx)
        m, _, _ := compare.CompareCategories(terr, serr)
        if m { matched++ }
        c := perCategory[f.Category]
        if m { c.pass++ } else { c.fail++ }
        perCategory[f.Category] = c
    }
    res.Observations["matched"] = matched
    res.Observations["per_category"] = perCategory

    res.AcceptanceChecks = append(res.AcceptanceChecks, required(
        "100% match on valid/invalid decisions across fixtures",
        matched == len(fixtures),
        fmt.Sprintf("matched=%d/%d", matched, len(fixtures)),
    ))

    res.Status = deriveStatus(res.AcceptanceChecks)
    return res
}
```

IBD-2 follows the same shape against `historical_utxos.yaml`.

### 3.5 CLIENT-1 test shape

Per SP1 spec §7.8:
1. Establish RPC + notification sessions.
2. Subscribe to blocks; record every block received during `Cfg.Durations.CLIENT1Observation` (default 1h, --short 5min).
3. Cross-check via REST every minute (during observation): every block REST returns must also have arrived via subscription.
4. Broadcast 50 txs (over the observation window); verify mempool arrival within 10s, later block inclusion.
5. Mid-run, force the notification stream closed for 60s, then reconnect. Verify catch-up delivers any missed blocks.

```go
func RunCLIENT1(ctx context.Context, env *testrunner.Env) testrunner.Result {
    // ... bootstrap, subscribe, ...
    obs := env.Cfg.Durations.CLIENT1Observation
    if obs <= 0 { obs = 5 * time.Minute }

    // Goroutine 1: tail Notification.Blocks() into seenViaSubscription[hash]bool
    // Goroutine 2: every 60s, call REST for the latest block; verify it's in seenViaSubscription
    // Goroutine 3: every 6s, broadcast a test tx; verify mempool within 10s
    // Mid-run: notif.Close() + sleep 60s + notif.Connect()
    // Final: assertions on counts + catch-up integrity
}
```

To make this fit in `--short` mode (5 min), block production needs to be regular. The test mines 1 block every 30s during observation.

### 3.6 CLIENT-3 test shape

Per SP1 spec §7.10:
1. Subscribe to block + transaction notifications.
2. Generate `Cfg.Limits.CLIENT3TxCount` (default 500) txs on a controlled schedule.
3. Verify every generated tx is observed.
4. Blocks arrive in strictly ascending height order.
5. Dependent transactions arrive after their parents.
6. Simulate midpoint reconnection; verify catch-up.

Note: per SP2 discovery, Centrifuge channels emit `block`, `subtree`, `node_status` — but NOT individual transaction events. Transactions are inferred via subtree events + REST tx fetches.

So for CLIENT-3, "verify every generated tx is observed" becomes:
- Generate 500 txs
- Mine blocks containing them
- Subscribe to subtree events; for each subtree hash, fetch the subtree contents
- Verify all 500 txids appear

This is more complex than the source plan suggests because Teranode's subtree-based architecture means tx-level events aren't first-class. The test reports the architectural difference as an observation and verifies the tx-coverage criterion via subtree expansion.

### 3.7 Notification close + reconnect

Per Q4=A: `notif.Close()` then `notif.Connect()`. The centrifuge-go library will reconnect automatically on errors, so we may need to use `Disconnect()` rather than `Close()` to be able to re-Connect afterwards. SP3's `NotificationClient.Close()` calls `c.client.Disconnect()` — we may need to add a separate `Reconnect()` method that does both, OR construct a fresh `NotificationClient` for the catch-up phase.

Simplest path: construct a second `NotificationClient` after the close-window, and verify the cached `node_status` snapshot it receives matches the one observed before the disconnect.

## 4. Verification & testing strategy

### 4.1 Unit tests

- `cmd/gen-fixtures` has its own tests:
  - Output YAML deserialises cleanly.
  - Generator is deterministic — running twice produces byte-identical output.
  - All 30+ PC-2 entries cover the 5 required categories with ≥6 per category.
- Fixture-loading helper has tests (file not found, malformed YAML).
- The 4 SP8 tests run live only.

### 4.2 SP8 done-check

```bash
make build lint test verify
./scripts/sp{1..7}-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

# Fixture invariant
test -s tests/testdata/historical_scripts.yaml
test -s tests/testdata/historical_utxos.yaml
./bin/gen-fixtures --out tests/testdata/
git diff --exit-code tests/testdata/

# Register asserts 16 tests now
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP8_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only CLIENT-1,CLIENT-3,PC-2,IBD-2 || true
    test -s report.json
    for id in CLIENT-1 CLIENT-3 PC-2 IBD-2; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
    done
    make compose-down
fi
echo "SP8 done-check passed."
```

### 4.3 Make targets

Add `gen-fixtures` target:

```makefile
gen-fixtures: build
	./bin/gen-fixtures --out tests/testdata/
```

`make verify` is extended to include `gen-fixtures` + `git diff --exit-code`.

## 5. Definition of done

- All 4 test files exist with verbatim source-plan comment block.
- `cmd/gen-fixtures/main.go` exists; `make gen-fixtures` produces deterministic YAML.
- `tests/testdata/historical_scripts.yaml` has ≥30 entries (≥6 per category).
- `tests/testdata/historical_utxos.yaml` has ≥10 entries.
- `cmd/teranode-acceptance/register.go` registers all 16 tests in alphabetical order.
- `make build lint test verify` exits 0; SP1–SP7 done-checks pass; SP8 static done-check exits 0.
- Code review approves.

## 6. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | 30+ fixtures take serious authoring time | Use fixture generator (`cmd/gen-fixtures`); each new category is a Go function that emits N entries. Hand-authoring 30 hex literals is the alternative — generator is faster and reproducible. |
| B | Synthetic regtest fixtures don't reflect real testnet edge cases | Documented in fixture provenance. SP10 can add a testnet-fixtures path. |
| C | Some "disabled opcodes" categories are post-Genesis re-enabled, leaving fewer truly-failing fixtures | Generator focuses on opcodes that remain invalid (OP_VER, OP_RESERVED, etc.) plus malformed-script cases. |
| D | CLIENT-1's 1h observation is impractical even at --short 5min on local docker | Short mode uses 5min; the test mines 1 block every 30s for ~10 blocks during the window. Acceptable for "stable session" assertion. |
| E | CLIENT-3's per-tx notification check is impossible (no tx events on Centrifuge) | Test infers tx coverage via subtree events + REST expansion. Observation records the architectural difference. |
| F | Mid-run reconnect simulation may leave the test in a bad state if client lib doesn't fully reconnect | Construct a second `NotificationClient` for the catch-up phase rather than re-using the disconnected one. |
| G | Fixture YAML deterministic-generation may break if SP4 txgen API changes between runs | The generator pins specific WIF + UTXO sequence; output should remain stable. CI's `git diff --exit-code` will catch any drift on a re-run. |

## 7. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Synthetic regtest fixtures, generated programmatically | per user (Q1=A) |
| 2 | Ship full ≥30 PC-2 fixtures + ≥10 IBD-2 fixtures | per user (Q2=A) |
| 3 | CLIENT-1 honours `Cfg.Durations.CLIENT1Observation` verbatim | per user (Q3=C) |
| 4 | CLIENT-3 reconnect uses Close+Connect (or fresh client) | per user (Q4=A) |
| 5 | Fixture generator at `cmd/gen-fixtures/`; output committed; `make verify` enforces no-drift | drafter |
| 6 | PC-2 categories: complex P2SH, disabled opcodes, CLEANSTACK, MINIMALDATA, malleability — 6 fixtures each | drafter |
| 7 | IBD-2 categories: 10 distinct edge-case spend types | drafter |
| 8 | Tx-level notification verification in CLIENT-3 done via subtree expansion (Teranode architecture) | drafter — matches SP2 discovery §3 |
| 9 | Reconnect simulation builds a fresh NotificationClient post-disconnect rather than reusing | drafter — simpler, more robust |

## 8. Out-of-scope reminders

SP8 doesn't ship: 7d/14d observations (SP9), PERF-1 throughput (SP9), testnet-fixture sourcing (SP10), expansion to ≥50 PC-2 fixtures (SP10 polish).

After SP8, verdict math (best case live):
- Critical: 6 PASS (PC-2, PC-3, IBD-2, INTER-2, CLIENT-1, CLIENT-3), 2 NOT_RUN (PC-1, INTER-1) → still INCOMPLETE.
- Important: 2 PASS, 1 NOT_RUN.
- Advisory: 8/8 measured.

So SP8 alone can't flip to GO; SP9 is needed for the remaining 2 Critical observations + PERF-1.
