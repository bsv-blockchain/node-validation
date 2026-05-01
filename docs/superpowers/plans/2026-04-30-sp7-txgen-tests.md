# SP7 — Tx-Generation Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land NEW-FR7 (chain depth 25), NEW-NFR7 (idle determinism), INTER-2 (1000-tx propagation, Critical). Adds `Funder.ConfirmMulti` + `Builder.BuildSplitter` to support INTER-2's 1000-UTXO bootstrap.

**Architecture:** Splitter pattern — one tx with N outputs gives the funder N spendable UTXOs. INTER-2 partitions 1000 txs into 333/333/334 across 3 submission patterns. NEW-FR7 uses existing `BuildChain`. NEW-NFR7 is read-only and fast.

**Tech Stack:** Existing.

---

### Task 1: Funder.ConfirmMulti + Builder.BuildSplitter + helper.pollMempoolUntil

**Files:**
- Modify: `internal/txgen/funder.go` (or `coinselect.go` where `Confirm` lives)
- Modify: `internal/txgen/builder.go`
- Modify: `internal/txgen/funder_test.go` (or appropriate test file)
- Modify: `internal/txgen/builder_test.go`
- Modify: `tests/helper.go`
- Modify: `tests/tests_test.go`

- [ ] **Step 1: Add `Funder.ConfirmMulti`**

In the file containing `Confirm` (currently `coinselect.go`), append:

```go
// ConfirmMulti marks `spent` UTXOs as no longer available and registers
// every UTXO in `newOutputs` as spendable. Used by tests that mine
// transactions creating multiple outputs (e.g. the INTER-2 splitter).
func (f *Funder) ConfirmMulti(spent []UTXO, newOutputs []UTXO) {
	f.MarkSpent(spent)
	for _, u := range newOutputs {
		f.AddUTXO(u)
	}
}
```

Replace existing `Confirm` body with a thin wrapper:

```go
// Confirm marks inputs spent and (optionally) adds the change UTXO.
// Tests call this after a successful broadcast.
func (f *Funder) Confirm(spent []UTXO, change *UTXO) {
	if change == nil {
		f.ConfirmMulti(spent, nil)
		return
	}
	f.ConfirmMulti(spent, []UTXO{*change})
}
```

- [ ] **Step 2: Add tests for ConfirmMulti**

```go
// in internal/txgen/coinselect_test.go (or wherever Confirm lives):

func TestConfirmMulti_marksSpentAndAddsAll(t *testing.T) {
	f := newFundedFunder(t, 1_000, 2_000)
	utxos := f.SnapshotUTXOs()
	if len(utxos) != 2 {
		t.Fatalf("setup: utxos=%d", len(utxos))
	}
	newOuts := []UTXO{
		{TxID: [32]byte{0xaa}, Vout: 0, Satoshis: 5_000, Script: utxos[0].Script},
		{TxID: [32]byte{0xaa}, Vout: 1, Satoshis: 6_000, Script: utxos[0].Script},
		{TxID: [32]byte{0xaa}, Vout: 2, Satoshis: 7_000, Script: utxos[0].Script},
	}
	f.ConfirmMulti(utxos, newOuts)
	if got := f.Balance(); got != 18_000 {
		t.Errorf("balance after ConfirmMulti: %d want 18000", got)
	}
	after := f.SnapshotUTXOs()
	if len(after) != 3 {
		t.Errorf("utxo count after ConfirmMulti: %d want 3", len(after))
	}
}

func TestConfirmMulti_emptyNewOutputsActsLikeMarkSpent(t *testing.T) {
	f := newFundedFunder(t, 1_000, 2_000, 3_000)
	utxos := f.SnapshotUTXOs()
	f.ConfirmMulti(utxos[:2], nil)
	if got := f.Balance(); got != 3_000 {
		t.Errorf("balance: %d", got)
	}
}

// Existing TestConfirm_addsChange should continue to pass — Confirm now
// delegates to ConfirmMulti.
```

- [ ] **Step 3: Add `Builder.BuildSplitter`**

Append to `internal/txgen/builder.go`:

```go
// BuildSplitter constructs a transaction with `n` outputs, each paying
// `satsPerOutput` to the funder's own address. Used by tests that need
// many independent UTXOs (INTER-2 needs 1000).
func (b *Builder) BuildSplitter(n int, satsPerOutput uint64, feeRate uint64) (BuildResult, error) {
	if n < 1 {
		return BuildResult{}, fmt.Errorf("BuildSplitter: n must be ≥1, got %d", n)
	}
	addrScript, err := P2PKHScript(b.funder.Address())
	if err != nil {
		return BuildResult{}, fmt.Errorf("p2pkh script: %w", err)
	}
	outputs := make([]Output, 0, n)
	for i := 0; i < n; i++ {
		outputs = append(outputs, Output{
			Script:      addrScript,
			Satoshis:    satsPerOutput,
			Description: fmt.Sprintf("splitter[%d]", i),
		})
	}
	return b.BuildP2PKH(BuildRequest{Outputs: outputs, FeeRate: feeRate})
}
```

- [ ] **Step 4: Add test for BuildSplitter**

```go
// in internal/txgen/builder_test.go:

func TestBuildSplitter_outputCount(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	addrScript, _ := P2PKHScript(f.Address())
	f.AddUTXO(UTXO{TxID: [32]byte{0xff}, Vout: 0, Satoshis: 1_000_000_000, Script: addrScript})
	res, err := f.Builder().BuildSplitter(50, 100_000, 500)
	if err != nil {
		t.Fatalf("BuildSplitter: %v", err)
	}
	parsed, err := bt.NewTxFromString(res.HexTx)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 50 splitter outputs + maybe 1 change → 50 or 51.
	if n := len(parsed.Outputs); n != 50 && n != 51 {
		t.Errorf("outputs: %d want 50 or 51", n)
	}
	for i := 0; i < 50; i++ {
		if parsed.Outputs[i].Satoshis != 100_000 {
			t.Errorf("output %d sats: %d", i, parsed.Outputs[i].Satoshis)
		}
	}
}

func TestBuildSplitter_zeroErrors(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	if _, err := f.Builder().BuildSplitter(0, 1_000, 500); err == nil {
		t.Error("n=0 should error")
	}
}
```

- [ ] **Step 5: Add `pollMempoolUntil` to `tests/helper.go`**

```go
// pollMempoolUntil polls rpc.GetRawMempool every 250ms until all wantTxIDs
// are present or the timeout passes. Returns the set of txids that were
// observed (subset of wantTxIDs) and whether the full set was matched.
//
// Usable for both teranode.RPCClient and svnode.RPCClient — both expose
// GetRawMempool() ([]string, error) with the same shape.
type mempoolReader interface {
	GetRawMempool(ctx context.Context) ([]string, error)
}

func pollMempoolUntil(ctx context.Context, rpc mempoolReader, wantTxIDs []string, timeout time.Duration) (seen map[string]bool, allSeen bool) {
	seen = make(map[string]bool, len(wantTxIDs))
	want := make(map[string]bool, len(wantTxIDs))
	for _, id := range wantTxIDs {
		want[id] = true
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		mempool, err := rpc.GetRawMempool(ctx)
		if err == nil {
			for _, id := range mempool {
				if want[id] {
					seen[id] = true
				}
			}
			if len(seen) == len(want) {
				return seen, true
			}
		}
		select {
		case <-ctx.Done():
			return seen, false
		case <-ticker.C:
		}
	}
	return seen, false
}
```

`mempoolReader` is an interface so the helper works with both `*teranode.RPCClient` and `*svnode.RPCClient`.

- [ ] **Step 6: Run tests, expect pass**

```bash
go test -race ./internal/txgen/... ./tests/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/txgen/ tests/helper.go
git commit -m "feat(txgen,tests): add ConfirmMulti, BuildSplitter, pollMempoolUntil"
```

---

### Task 2: NEW-FR7

**Files:**
- Create: `tests/new_fr7.go`

- [ ] **Step 1: Implement**

```go
// Package tests — NEW-FR7 implementation.
//
// Source: derived from FR-7. Captures R1.
//
// Objective:
//   Verify Teranode accepts and propagates chains of dependent unconfirmed
//   transactions to the depth the source plan specifies.
//
// Method:
//   1. Build a chain of Cfg.Limits.FR7ChainDepth (default 25) dependent
//      unconfirmed transactions via Builder.BuildChain.
//   2. Submit each link via Teranode RPC sendrawtransaction; record per-link
//      acceptance.
//   3. Wait briefly for P2P propagation (5s).
//   4. Get SV Node getrawmempool; verify all chain txids are visible.
//   5. Mine 1 block; wait for tip propagation; fetch the block from
//      Teranode REST; verify all chain txs are in it.
//
// Acceptance criteria (from FR-7):
//   • Chain of depth ≥25 fully accepted into Teranode mempool.
//   • Chain visible in SV Node mempool within default_propagation seconds.
//   • All chain members eventually mined without intermediate confirmations.
//   • Behaviour consistent with SV Node (= mempool visibility per spec §4.1).

package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunNEWFR7(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-FR7", Title: "Unconfirmed Transaction Chain Acceptance",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-7"},
		CapturedRisks:         []string{"R1"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil || env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	depth := env.Cfg.Limits.FR7ChainDepth
	if depth <= 0 {
		depth = 25
	}
	res.Observations["chain_depth"] = depth

	funder := env.TxGen
	builder := funder.Builder()
	if funder.Balance() < 100_000_000 {
		if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
		time.Sleep(2 * time.Second)
	}

	addrScript, _ := txgen.P2PKHScript(funder.Address())

	chain, err := builder.BuildChain(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	}, depth)
	if err != nil {
		return errorResult(res, fmt.Errorf("BuildChain depth=%d: %w", depth, err))
	}

	chainTxIDs := make([]string, 0, len(chain))
	allAccepted := true
	for i, link := range chain {
		txid, err := env.Teranode.RPC.SendRawTransaction(ctx, link.HexTx)
		if err != nil {
			allAccepted = false
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				fmt.Sprintf("Chain link %d (depth %d) accepted by Teranode", i, i+1),
				fmt.Sprintf("err=%v", err),
			))
			break
		}
		chainTxIDs = append(chainTxIDs, txid)
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Full chain of depth %d accepted by Teranode mempool", depth),
		allAccepted && len(chainTxIDs) == depth,
		fmt.Sprintf("accepted=%d/%d", len(chainTxIDs), depth),
	))

	if !allAccepted {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// Wait briefly for P2P propagation, then check SV Node mempool.
	propagation := env.Cfg.Durations.DefaultPropagation
	if propagation <= 0 {
		propagation = 10 * time.Second
	}
	seenSV, allSeenSV := pollMempoolUntil(ctx, env.SVNode.RPC, chainTxIDs, propagation)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("All %d chain txs visible in SV Node mempool within %v", depth, propagation),
		allSeenSV,
		fmt.Sprintf("seen=%d/%d", len(seenSV), depth),
	))

	// Mine and verify all chain txs are confirmed.
	mined, err := mineBlocks(ctx, env, 1)
	if err != nil || len(mined) != 1 {
		return errorResult(res, fmt.Errorf("mine: err=%v hashes=%v", err, mined))
	}
	if err := waitForTeranodeTip(ctx, env.Teranode.RPC, mined[0], 30*time.Second); err != nil {
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"Teranode tip reached mined block within 30s",
			err.Error(),
		))
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	blockBytes, err := env.Teranode.REST.GetBlockBytes(ctx, mined[0])
	if err != nil {
		return errorResult(res, fmt.Errorf("get block bytes: %w", err))
	}
	stdTxIDs, err := parseStandardBlock(blockBytes)
	if err != nil {
		return errorResult(res, fmt.Errorf("parse block: %w", err))
	}
	idSet := make(map[string]bool, len(stdTxIDs))
	for _, id := range stdTxIDs {
		idSet[id] = true
	}
	confirmed := 0
	for _, link := range chain {
		txid := hex.EncodeToString(link.TxID[:])
		if idSet[txid] {
			confirmed++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("All %d chain txs mined into a single block (no intermediate confirmations)", depth),
		confirmed == depth,
		fmt.Sprintf("confirmed=%d/%d", confirmed, depth),
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/new_fr7.go
git commit -m "feat(tests): add NEW-FR7 — Unconfirmed Transaction Chain Acceptance"
```

---

### Task 3: NEW-NFR7

**Files:**
- Create: `tests/new_nfr7.go`

- [ ] **Step 1: Implement**

```go
// Package tests — NEW-NFR7 implementation.
//
// Source: derived from NFR-7.
//
// Objective:
//   Verify identical operations under similar load produce identical
//   results.
//
// Method (idle conditions only per SP7 spec §4.2 Q2=A):
//   1. Pick three pure read operations: getbestblockhash, getblock <known
//      hash>, getrawtransaction <known confirmed txid>.
//   2. For each, capture a baseline response.
//   3. Repeat each Cfg.Durations.NewNFR7Iterations (default 100) times.
//   4. Verify every iteration's response is byte-identical to the baseline.
//
// Acceptance criteria (from NFR-7):
//   • Read operations return identical results across iterations.
//   • No load-induced variation — DEFERRED to SP9 with note (PERF-1
//     infrastructure required for 100/500 TPS).
//   • Errors fall into well-defined codes — Pass if no errors observed
//     during the idle calls.

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWNFR7(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-NFR7", Title: "Deterministic Behaviour Under Repeated Operations",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-7"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil {
		return skipMissing(res, "Teranode RPC not configured")
	}

	iterations := env.Cfg.Durations.NewNFR7Iterations
	if iterations <= 0 {
		iterations = 100
	}
	res.Observations["iterations"] = iterations

	// Pick deterministic anchors at test start.
	bestHash, err := env.Teranode.RPC.GetBestBlockHash(ctx)
	if err != nil {
		return errorResult(res, fmt.Errorf("getbestblockhash baseline: %w", err))
	}

	var blkBaseline json.RawMessage
	if err := env.Teranode.RPC.Call(ctx, "getblock", []any{bestHash, 1}, &blkBaseline); err != nil {
		return errorResult(res, fmt.Errorf("getblock baseline: %w", err))
	}
	var parsedBlk struct {
		Tx []string `json:"tx"`
	}
	if err := json.Unmarshal(blkBaseline, &parsedBlk); err != nil || len(parsedBlk.Tx) == 0 {
		return errorResult(res, fmt.Errorf("parse block tx list: err=%v len=%d", err, len(parsedBlk.Tx)))
	}
	knownTxID := parsedBlk.Tx[0]
	rawTxBaseline, err := env.Teranode.RPC.GetRawTransaction(ctx, knownTxID, 0)
	if err != nil {
		return errorResult(res, fmt.Errorf("getrawtransaction baseline: %w", err))
	}

	// Read 1: getbestblockhash.
	var hashErrCount, hashDriftCount int
	for i := 0; i < iterations; i++ {
		h, err := env.Teranode.RPC.GetBestBlockHash(ctx)
		if err != nil {
			hashErrCount++
			continue
		}
		if h != bestHash {
			hashDriftCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getbestblockhash returns identical result across %d iterations", iterations),
		hashErrCount == 0 && hashDriftCount == 0,
		fmt.Sprintf("errs=%d drifts=%d", hashErrCount, hashDriftCount),
	))

	// Read 2: getblock.
	var blkErrCount, blkDriftCount int
	for i := 0; i < iterations; i++ {
		var raw json.RawMessage
		err := env.Teranode.RPC.Call(ctx, "getblock", []any{bestHash, 1}, &raw)
		if err != nil {
			blkErrCount++
			continue
		}
		if !bytes.Equal(raw, blkBaseline) {
			blkDriftCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getblock returns byte-identical JSON across %d iterations", iterations),
		blkErrCount == 0 && blkDriftCount == 0,
		fmt.Sprintf("errs=%d drifts=%d", blkErrCount, blkDriftCount),
	))

	// Read 3: getrawtransaction.
	var rawErrCount, rawDriftCount int
	for i := 0; i < iterations; i++ {
		raw, err := env.Teranode.RPC.GetRawTransaction(ctx, knownTxID, 0)
		if err != nil {
			rawErrCount++
			continue
		}
		if !bytes.Equal(raw, rawTxBaseline) {
			rawDriftCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getrawtransaction returns byte-identical hex across %d iterations", iterations),
		rawErrCount == 0 && rawDriftCount == 0,
		fmt.Sprintf("errs=%d drifts=%d", rawErrCount, rawDriftCount),
	))

	res.Observations["best_hash_drifts"] = hashDriftCount
	res.Observations["block_drifts"] = blkDriftCount
	res.Observations["raw_tx_drifts"] = rawDriftCount
	res.Observations["best_hash_errs"] = hashErrCount
	res.Observations["block_errs"] = blkErrCount
	res.Observations["raw_tx_errs"] = rawErrCount

	// Load-condition checks: deferred per SP7 spec §4.2.
	res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
		"Read ops return identical results under 100 TPS load",
		"deferred to SP9 — requires PERF-1 TPS-ramp infrastructure",
	))
	res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
		"Read ops return identical results under 500 TPS load",
		"deferred to SP9 — requires PERF-1 TPS-ramp infrastructure",
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/new_nfr7.go
git commit -m "feat(tests): add NEW-NFR7 — Deterministic Behaviour (idle conditions)"
```

---

### Task 4: INTER-2

**Files:**
- Create: `tests/inter2.go`

- [ ] **Step 1: Implement**

```go
// Package tests — INTER-2 implementation.
//
// Source plan §"Interoperability Tests" → INTER-2. Captures R1, R2.
// Severity Critical.
//
// Objective:
//   Verify transactions broadcast to one node type reach the other reliably.
//
// Method:
//   1. Build a splitter tx with N outputs (N = Cfg.Limits.INTER2TxCount,
//      default 1000); submit to Teranode; mine 1 block to confirm.
//   2. Build N simple P2PKH txs at 4 fee-rate buckets and 4 size variations.
//   3. Partition: 333 to "SV Node only", 333 to "Teranode only", 334 to "both".
//   4. Submit concurrently (10 goroutines per group).
//   5. Poll each backend's mempool every 250ms for 10s; record per-tx
//      cross-side observation.
//   6. Compute % observed cross-side within Cfg.Durations.DefaultPropagation.
//
// Acceptance criteria (from INTER-2):
//   • ≥99% of "Teranode only" group appears in SV Node mempool within 10s.
//   • ≥99% of "SV Node only" group appears in Teranode mempool within 10s.
//   • "Both" group: each tx accepted by at least one backend; no
//     permanently lost or stuck txs.

package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunINTER2(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "INTER-2", Title: "Cross-Implementation Transaction Propagation",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-5", "NFR-6"},
		CapturedRisks:         []string{"R1", "R2"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil ||
		env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	count := env.Cfg.Limits.INTER2TxCount
	if count <= 0 {
		count = 1000
	}
	res.Observations["tx_count"] = count

	propagation := env.Cfg.Durations.DefaultPropagation
	if propagation <= 0 {
		propagation = 10 * time.Second
	}

	funder := env.TxGen
	builder := funder.Builder()

	// Splitter — need enough sats to fund (count * 100_000) + fee.
	const splitterSatsPerOutput uint64 = 100_000
	target := uint64(count) * splitterSatsPerOutput * 2 // 2x headroom for fee
	if funder.Balance() < target {
		if _, err := funder.Bootstrap(ctx, target); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap %d sat: %w", target, err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
		time.Sleep(2 * time.Second)
	}

	splitter, err := builder.BuildSplitter(count, splitterSatsPerOutput, 500)
	if err != nil {
		return errorResult(res, fmt.Errorf("BuildSplitter: %w", err))
	}
	splitterTxID, err := env.Teranode.RPC.SendRawTransaction(ctx, splitter.HexTx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Splitter tx with %d outputs accepted", count),
		err == nil && splitterTxID != "",
		fmt.Sprintf("err=%v", err),
	))
	if err != nil {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// Mine to confirm splitter; refresh funder UTXO state.
	if _, err := mineBlocks(ctx, env, 1); err != nil {
		return errorResult(res, err)
	}
	time.Sleep(2 * time.Second)

	// Reset funder to know exactly which UTXOs we have. The splitter tx's
	// outputs become our spendable set.
	funder.Reset()
	addrScript, _ := txgen.P2PKHScript(funder.Address())
	newUTXOs := make([]txgen.UTXO, count)
	for i := 0; i < count; i++ {
		newUTXOs[i] = txgen.UTXO{
			TxID:     splitter.TxID,
			Vout:     uint32(i),
			Satoshis: splitterSatsPerOutput,
			Script:   addrScript,
		}
	}
	funder.ConfirmMulti(splitter.Inputs, newUTXOs)

	// Build `count` simple P2PKH txs varying fee rate and size.
	feeRates := []uint64{250, 500, 1000, 2000}
	sizeOuts := []int{1, 2, 5, 10}

	type built struct {
		txid   string
		hex    string
	}
	txs := make([]built, 0, count)
	for i := 0; i < count; i++ {
		feeRate := feeRates[i%len(feeRates)]
		nOut := sizeOuts[i%len(sizeOuts)]
		outs := make([]txgen.Output, nOut)
		for j := 0; j < nOut; j++ {
			outs[j] = txgen.Output{
				Script:   addrScript,
				Satoshis: 1_000,
			}
		}
		bres, err := builder.BuildP2PKH(txgen.BuildRequest{
			Outputs:   outs,
			FeeRate:   feeRate,
			SpendUTXO: &newUTXOs[i],
		})
		if err != nil {
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				fmt.Sprintf("Build tx %d (fee=%d outs=%d)", i, feeRate, nOut),
				err.Error(),
			))
			res.Status = deriveStatus(res.AcceptanceChecks)
			return res
		}
		txs = append(txs, built{
			txid: hex.EncodeToString(bres.TxID[:]),
			hex:  bres.HexTx,
		})
	}

	// Partition into 3 groups: 333/333/334.
	groupTeranodeOnly := txs[:count/3]
	groupSVOnly := txs[count/3 : 2*count/3]
	groupBoth := txs[2*count/3:]

	teranodeOnlyTxIDs := txidsOf(groupTeranodeOnly)
	svOnlyTxIDs := txidsOf(groupSVOnly)
	bothTxIDs := txidsOf(groupBoth)

	res.Observations["teranode_only_count"] = len(groupTeranodeOnly)
	res.Observations["sv_only_count"] = len(groupSVOnly)
	res.Observations["both_count"] = len(groupBoth)

	// Concurrent submission with bounded parallelism.
	submitGroup := func(grp []built, submit func(context.Context, string) (string, error), label string) (sent int) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		for _, t := range grp {
			wg.Add(1)
			sem <- struct{}{}
			go func(tx built) {
				defer wg.Done()
				defer func() { <-sem }()
				if _, err := submit(ctx, tx.hex); err == nil {
					mu.Lock()
					sent++
					mu.Unlock()
				}
			}(t)
		}
		wg.Wait()
		return sent
	}

	teranodeSent := submitGroup(groupTeranodeOnly, env.Teranode.RPC.SendRawTransaction, "teranode-only→teranode")
	svSent := submitGroup(groupSVOnly, env.SVNode.RPC.SendRawTransaction, "sv-only→svnode")

	// "Both" group — submit to Teranode, then SV Node 1ms later.
	var bothSent int
	{
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		for _, t := range groupBoth {
			wg.Add(1)
			sem <- struct{}{}
			go func(tx built) {
				defer wg.Done()
				defer func() { <-sem }()
				_, errT := env.Teranode.RPC.SendRawTransaction(ctx, tx.hex)
				time.Sleep(1 * time.Millisecond)
				_, errS := env.SVNode.RPC.SendRawTransaction(ctx, tx.hex)
				if errT == nil || errS == nil {
					mu.Lock()
					bothSent++
					mu.Unlock()
				}
			}(t)
		}
		wg.Wait()
	}

	res.Observations["teranode_only_submitted"] = teranodeSent
	res.Observations["sv_only_submitted"] = svSent
	res.Observations["both_submitted"] = bothSent

	// Poll mempools.
	seenSV, _ := pollMempoolUntil(ctx, env.SVNode.RPC, teranodeOnlyTxIDs, propagation)
	seenTN, _ := pollMempoolUntil(ctx, env.Teranode.RPC, svOnlyTxIDs, propagation)

	teranodeToSVPct := 100.0 * float64(len(seenSV)) / float64(len(teranodeOnlyTxIDs))
	svToTeranodePct := 100.0 * float64(len(seenTN)) / float64(len(svOnlyTxIDs))

	res.Observations["teranode_to_sv_pct"] = teranodeToSVPct
	res.Observations["sv_to_teranode_pct"] = svToTeranodePct

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("≥99%% of Teranode-only txs reach SV Node within %v", propagation),
		teranodeToSVPct >= 99.0,
		fmt.Sprintf("seen=%d/%d (%.1f%%)", len(seenSV), len(teranodeOnlyTxIDs), teranodeToSVPct),
	))
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("≥99%% of SV-Node-only txs reach Teranode within %v", propagation),
		svToTeranodePct >= 99.0,
		fmt.Sprintf("seen=%d/%d (%.1f%%)", len(seenTN), len(svOnlyTxIDs), svToTeranodePct),
	))
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Both-group: ≥99%% of txs accepted by at least one backend"),
		float64(bothSent)/float64(len(groupBoth)) >= 0.99,
		fmt.Sprintf("submitted=%d/%d", bothSent, len(groupBoth)),
	))

	// Mine to clean up.
	_, _ = mineBlocks(ctx, env, 1)

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}

func txidsOf(txs []struct {
	txid string
	hex  string
}) []string {
	out := make([]string, len(txs))
	for i, t := range txs {
		out[i] = t.txid
	}
	return out
}
```

Notice the `built` struct is defined inside `RunINTER2`; the helper `txidsOf` references the same anonymous struct shape. If Go complains about the anonymous struct type mismatch, promote `built` to a package-private named type at file scope:

```go
type interTx struct {
	txid string
	hex  string
}

// txidsOf as named-type version:
func txidsOf(txs []interTx) []string { ... }
```

- [ ] **Step 2: Verify build**

```bash
go build ./tests/...
```

- [ ] **Step 3: Commit**

```bash
git add tests/inter2.go
git commit -m "feat(tests): add INTER-2 — Cross-Implementation Transaction Propagation"
```

---

### Task 5: Register tests + done-check

**Files:**
- Modify: `cmd/teranode-acceptance/register.go`
- Modify: `cmd/teranode-acceptance/register_test.go`
- Create: `scripts/sp7-done-check.sh`

- [ ] **Step 1: Update `register.go`** — register all 12 tests alphabetically

```go
package main

import (
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/tests"
)

func registerTests(suite *testrunner.Suite) {
	// Alphabetical by ID.
	suite.Register("CLIENT-2", tests.RunCLIENT2)
	suite.Register("INTER-2", tests.RunINTER2)
	suite.Register("NEW-FR10", tests.RunNEWFR10)
	suite.Register("NEW-FR11", tests.RunNEWFR11)
	suite.Register("NEW-FR7", tests.RunNEWFR7)
	suite.Register("NEW-FR8", tests.RunNEWFR8)
	suite.Register("NEW-FR9", tests.RunNEWFR9)
	suite.Register("NEW-NFR11", tests.RunNEWNFR11)
	suite.Register("NEW-NFR13", tests.RunNEWNFR13)
	suite.Register("NEW-NFR7", tests.RunNEWNFR7)
	suite.Register("OPS-3", tests.RunOPS3)
	suite.Register("PC-3", tests.RunPC3)
}
```

- [ ] **Step 2: Update `register_test.go`** — replace with `TestRegisterTests_SP7RegistersTwelve`

```go
func TestRegisterTests_SP7RegistersTwelve(t *testing.T) {
	cfg := config.Config{TestTimeout: time.Minute}
	env := testrunner.NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
	suite := testrunner.NewSuite(env)
	registerTests(suite)
	results := suite.Run(testContext(t))
	if len(results) != 12 {
		t.Fatalf("expected 12 results, got %d", len(results))
	}
	wantIDs := map[string]bool{
		"CLIENT-2": false, "INTER-2": false,
		"NEW-FR7": false, "NEW-FR8": false, "NEW-FR9": false,
		"NEW-FR10": false, "NEW-FR11": false,
		"NEW-NFR7": false, "NEW-NFR11": false, "NEW-NFR13": false,
		"OPS-3": false, "PC-3": false,
	}
	for _, r := range results {
		if _, ok := wantIDs[r.ID]; ok {
			wantIDs[r.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing result for %s", id)
		}
	}
}
```

(Replace the prior `TestRegisterTests_SP6RegistersNine`.)

- [ ] **Step 3: Create `scripts/sp7-done-check.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP6 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh

echo "==> tests/ + internal/txgen/ build and unit tests pass"
go test -race ./tests/... ./internal/txgen/...

echo "==> register.go registers tests"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP7_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only NEW-FR7,NEW-NFR7,INTER-2 || true
    test -s report.json
    for id in NEW-FR7 NEW-NFR7 INTER-2; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi
echo "==> SP7 done-check passed."
```

- [ ] **Step 4: Make executable, run static path**

```bash
chmod +x scripts/sp7-done-check.sh
./scripts/sp7-done-check.sh
```

- [ ] **Step 5: Commit**

```bash
git add cmd/teranode-acceptance/ scripts/sp7-done-check.sh
git commit -m "feat(cmd): register 12 tests; add sp7-done-check"
```

---

### Task 6: Code review and closeout

- [ ] **Step 1: Run `superpowers:code-reviewer`**

Verify:
- All 3 test files exist with verbatim source-plan comment block.
- `Funder.ConfirmMulti` exists; `Confirm` is now a thin wrapper.
- `Builder.BuildSplitter` exists with passing tests.
- `pollMempoolUntil` in `tests/helper.go`.
- `register.go` has 12 tests in alphabetical order.
- `make build lint test verify` exits 0; SP1-SP6 done-checks pass; SP7 static done-check exits 0.

- [ ] **Step 2: Address findings**

- [ ] **Step 3: Capture review report; tag**

```bash
mkdir -p docs/superpowers/reviews
$EDITOR docs/superpowers/reviews/2026-04-30-sp7-code-review.md
git add docs/superpowers/reviews/
git commit -m "docs: capture SP7 code-review report"
git tag -a sp7-complete -m "SP7 — Tx-Generation Tests complete"
```

---

## Self-review checklist (planner)

- [x] Spec coverage — every section of the SP7 spec is implemented.
- [x] No placeholders — every code block contains real, runnable code.
- [x] All 3 tests follow SP5/SP6 shape.
- [x] INTER-2 default tx count 1000 per Q3=A.
- [x] NEW-NFR7 idle-only with deferred-load checks per Q2=A.
- [x] NEW-FR7 SV-Node parity = mempool visibility per Q1=A.
- [x] register.go alphabetical (lexicographic; CLIENT-2 < INTER-2 < NEW-FR10 < NEW-FR11 < NEW-FR7 < NEW-FR8 < NEW-FR9 < NEW-NFR11 < NEW-NFR13 < NEW-NFR7 < OPS-3 < PC-3).
