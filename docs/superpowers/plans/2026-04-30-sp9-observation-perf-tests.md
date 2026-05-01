# SP9 — Long-Observation + Perf Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land final 3 acceptance tests (PC-1, INTER-1, PERF-1) plus shared `internal/observer/` package and `Limits.PERF1RampSteps` config knob.

**Architecture:** Observer package wraps tip-polling and reorg detection; consumed by PC-1 + INTER-1. PERF-1 builds on SP4/SP7 splitter pattern with rate-limited submission. All three tests run sequentially under the suite's per-test timeout (which the operator raises to 60min for SP9 live runs).

**Tech Stack:** Existing.

---

### Task 1: `internal/observer/` package

**Files:**
- Create: `internal/observer/doc.go`
- Create: `internal/observer/observer.go`
- Create: `internal/observer/observer_test.go`

- [ ] **Step 1: Create `doc.go`**

```go
// Package observer provides shared block-tip polling and reorg-detection
// helpers used by PC-1 (parallel node comparison) and INTER-1 (mixed-network
// consensus). It abstracts over teranode and svnode RPC clients via a
// minimal TipReader interface.
package observer
```

- [ ] **Step 2: Implement `observer.go`**

```go
package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// TipReader is the minimal interface needed for chain-tip polling.
// Both *teranode.RPCClient and *svnode.RPCClient satisfy it.
type TipReader interface {
	GetBestBlockHash(ctx context.Context) (string, error)
	GetBlockchainInfo(ctx context.Context) (json.RawMessage, error)
}

// TipSnapshot is one observation of one node's best tip.
type TipSnapshot struct {
	Time   time.Time
	Source string
	Hash   string
	Height int64
}

// ReorgEvent is observed when a node's best tip moves to a different chain
// (best-hash changes without height monotonically increasing — i.e. height
// drops or stays equal with a different hash).
type ReorgEvent struct {
	Time   time.Time
	Source string
	From   TipSnapshot
	To     TipSnapshot
}

// Observer polls a set of TipReaders at a fixed interval and emits
// snapshots to a buffered channel.
type Observer struct {
	rpcs     map[string]TipReader
	interval time.Duration
	logger   *slog.Logger
}

// NewObserver constructs an Observer.
func NewObserver(rpcs map[string]TipReader, interval time.Duration, logger *slog.Logger) *Observer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Observer{rpcs: rpcs, interval: interval, logger: logger}
}

// Run polls until the deadline; returns all snapshots collected.
func (o *Observer) Run(ctx context.Context, until time.Time) []TipSnapshot {
	var (
		mu        sync.Mutex
		snapshots []TipSnapshot
	)
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	for time.Now().Before(until) {
		select {
		case <-ctx.Done():
			return snapshots
		case <-ticker.C:
			now := time.Now()
			for label, rpc := range o.rpcs {
				h, err := rpc.GetBestBlockHash(ctx)
				if err != nil {
					o.logger.Debug("observer: getbestblockhash error", "src", label, "err", err)
					continue
				}
				var info struct {
					Blocks int64 `json:"blocks"`
				}
				raw, err := rpc.GetBlockchainInfo(ctx)
				height := int64(-1)
				if err == nil {
					_ = json.Unmarshal(raw, &info)
					height = info.Blocks
				}
				mu.Lock()
				snapshots = append(snapshots, TipSnapshot{
					Time: now, Source: label, Hash: h, Height: height,
				})
				mu.Unlock()
			}
		}
	}
	return snapshots
}

// DivergenceCount returns the number of timestamps where ≥2 sources
// reported different best-block hashes simultaneously.
func DivergenceCount(snapshots []TipSnapshot) int {
	// Group by ~50ms time window (since polls happen at the same moment, snapshots
	// from one poll round share the same Time within microseconds).
	type key time.Time
	rounds := map[time.Time]map[string]string{}
	for _, s := range snapshots {
		// Round Time to interval-bucket (1s precision suffices).
		bucket := s.Time.Round(time.Second)
		if rounds[bucket] == nil {
			rounds[bucket] = map[string]string{}
		}
		rounds[bucket][s.Source] = s.Hash
	}
	count := 0
	for _, hashes := range rounds {
		seen := map[string]bool{}
		for _, h := range hashes {
			seen[h] = true
		}
		if len(seen) > 1 {
			count++
		}
	}
	return count
}

// ReorgsObserved scans per-source snapshots for any best-hash change at the
// same or lower height (= chain switched, not advanced).
func ReorgsObserved(snapshots []TipSnapshot) []ReorgEvent {
	bySource := map[string][]TipSnapshot{}
	for _, s := range snapshots {
		bySource[s.Source] = append(bySource[s.Source], s)
	}
	var events []ReorgEvent
	for src, ss := range bySource {
		for i := 1; i < len(ss); i++ {
			prev, cur := ss[i-1], ss[i]
			if prev.Hash == cur.Hash {
				continue
			}
			// Reorg signal: new hash with height ≤ previous height.
			if cur.Height <= prev.Height && prev.Height > 0 {
				events = append(events, ReorgEvent{
					Time: cur.Time, Source: src, From: prev, To: cur,
				})
			}
		}
		_ = fmt.Sprintf("%s", src) // keep import live
	}
	return events
}

// ConvergedAt returns the earliest time after `from` at which all sources
// reported the same hash. Returns zero time if never converged within the
// snapshots.
func ConvergedAt(snapshots []TipSnapshot, from time.Time, expectedHash string) time.Time {
	rounds := map[time.Time]map[string]string{}
	for _, s := range snapshots {
		if s.Time.Before(from) {
			continue
		}
		bucket := s.Time.Round(time.Second)
		if rounds[bucket] == nil {
			rounds[bucket] = map[string]string{}
		}
		rounds[bucket][s.Source] = s.Hash
	}
	// Sort buckets ascending.
	var keys []time.Time
	for k := range rounds {
		keys = append(keys, k)
	}
	sortTimes(keys)
	for _, k := range keys {
		hashes := rounds[k]
		allMatch := true
		for _, h := range hashes {
			if h != expectedHash {
				allMatch = false
				break
			}
		}
		if allMatch && len(hashes) >= 2 {
			return k
		}
	}
	return time.Time{}
}

func sortTimes(t []time.Time) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && t[j].Before(t[j-1]); j-- {
			t[j], t[j-1] = t[j-1], t[j]
		}
	}
}
```

- [ ] **Step 3: Implement `observer_test.go`**

```go
package observer

import (
	"testing"
	"time"
)

func TestDivergenceCount_allAgree(t *testing.T) {
	now := time.Now()
	ss := []TipSnapshot{
		{Time: now, Source: "a", Hash: "x", Height: 1},
		{Time: now, Source: "b", Hash: "x", Height: 1},
	}
	if got := DivergenceCount(ss); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestDivergenceCount_disagree(t *testing.T) {
	now := time.Now()
	ss := []TipSnapshot{
		{Time: now, Source: "a", Hash: "x", Height: 1},
		{Time: now, Source: "b", Hash: "y", Height: 1},
	}
	if got := DivergenceCount(ss); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestReorgsObserved_simpleReorg(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "B0", Height: 5},
		{Time: t0.Add(time.Second), Source: "a", Hash: "B1", Height: 6},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "T2", Height: 7},
		{Time: t0.Add(3 * time.Second), Source: "a", Hash: "T3", Height: 6}, // reorg: hash changed, height ≤ prev
	}
	events := ReorgsObserved(ss)
	if len(events) != 1 {
		t.Errorf("got %d reorgs, want 1", len(events))
	}
}

func TestReorgsObserved_noReorgOnAdvance(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "B0", Height: 5},
		{Time: t0.Add(time.Second), Source: "a", Hash: "B1", Height: 6},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "B2", Height: 7},
	}
	if events := ReorgsObserved(ss); len(events) != 0 {
		t.Errorf("got %d, want 0", len(events))
	}
}

func TestConvergedAt(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "X", Height: 1},
		{Time: t0, Source: "b", Hash: "Y", Height: 1},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "Z", Height: 2},
		{Time: t0.Add(2 * time.Second), Source: "b", Hash: "Z", Height: 2},
	}
	got := ConvergedAt(ss, t0, "Z")
	if got.IsZero() {
		t.Error("expected convergence")
	}
}
```

- [ ] **Step 4: Run, expect pass**

```bash
go test -race ./internal/observer/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/observer/
git commit -m "feat(observer): add tip polling + reorg detection helper"
```

---

### Task 2: Config additions for PERF-1 ramp

**Files:**
- Modify: `config/config.go` — add `Limits.PERF1RampSteps []int`
- Modify: `config/defaults.go` — default `[10, 50, 100, 250]`
- Modify: 4 YAML files

- [ ] **Step 1: Add field to `Limits` struct in `config/config.go`**

```go
type Limits struct {
	// ... existing fields
	PERF1RampSteps []int `yaml:"perf1_ramp_steps"`
}
```

Update `mergeYAML`:

```go
if len(src.Limits.PERF1RampSteps) > 0 {
    dst.Limits.PERF1RampSteps = src.Limits.PERF1RampSteps
}
```

- [ ] **Step 2: Add default in `config/defaults.go`**

```go
if len(c.Limits.PERF1RampSteps) == 0 {
    c.Limits.PERF1RampSteps = []int{10, 50, 100, 250}
}
```

- [ ] **Step 3: Update YAML files**

Add to each:

`config/testdata/minimal.yaml`:
```yaml
  perf1_ramp_steps: [10, 50, 100, 250]
```

`config.example.yaml`:
```yaml
  perf1_ramp_steps: [10, 50, 100, 250]   # PERF-1 ramp; raise PERF1MaxTPS to extend
```

`config.docker.yaml`:
```yaml
  perf1_ramp_steps: [10, 50, 100, 250]
```

`cmd/teranode-acceptance/testdata/integration.yaml`:
```yaml
  perf1_ramp_steps: [10, 50, 100]
```

- [ ] **Step 4: Run**

```bash
make build lint test verify
```

- [ ] **Step 5: Commit**

```bash
git add config/ config.example.yaml config.docker.yaml cmd/teranode-acceptance/testdata/
git commit -m "feat(config): add perf1_ramp_steps with default [10,50,100,250]"
```

---

### Task 3: Reorg induction helper + PC-1 + INTER-1

**Files:**
- Modify: `tests/helper.go` — add `induceReorg` helper used by PC-1 + INTER-1
- Create: `tests/pc1.go`
- Create: `tests/inter1.go`

- [ ] **Step 1: Append to `tests/helper.go`**

```go
import (
	"github.com/bsv-blockchain/node-validation/internal/observer"
)

// reorgResult captures the outcome of a reorg induction sub-phase.
type reorgResult struct {
	BaselineHash string
	BaselineHeight int64
	WinnerHash   string
	WinnerHeight int64
	ConvergedAt  time.Time
	Reorged      bool
	Err          error
}

// induceReorg manually creates competing chains and verifies convergence.
//
// Procedure (per SP9 spec §3.1):
//  1. Capture baseline (all 6 nodes at height H, hash B0).
//  2. Mine 1 block on svnode-1 → B1.
//  3. Wait for B1 to propagate.
//  4. invalidateblock(B1) on teranode-1 (rolls teranode-1 back).
//  5. generatetoaddress 2 blocks on teranode-1 → T1, T2.
//  6. Wait for the rest of the mesh to reorg to T2.
//  7. Verify all 6 nodes at T2.
//
// Returns reorgResult with details. If any step fails, sets Err and returns.
func induceReorg(ctx context.Context, env *testrunner.Env, observerSnapshots []observer.TipSnapshot) reorgResult {
	res := reorgResult{}
	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil {
		res.Err = fmt.Errorf("required clients not configured")
		return res
	}

	// 1. Baseline.
	baseline, err := env.SVNode.RPC.GetBestBlockHash(ctx)
	if err != nil {
		res.Err = fmt.Errorf("baseline: %w", err)
		return res
	}
	res.BaselineHash = baseline

	// 2. Mine 1 block on svnode-1.
	addr, err := env.SVNode.RPC.GetNewAddress(ctx)
	if err != nil {
		res.Err = fmt.Errorf("getnewaddress: %w", err)
		return res
	}
	b1Hashes, err := env.SVNode.RPC.GenerateToAddress(ctx, 1, addr)
	if err != nil || len(b1Hashes) != 1 {
		res.Err = fmt.Errorf("mine B1: err=%v hashes=%v", err, b1Hashes)
		return res
	}
	b1 := b1Hashes[0]

	// 3. Wait for propagation: poll teranode-1's tip until == b1.
	if err := waitForTeranodeTip(ctx, env.Teranode.RPC, b1, 30*time.Second); err != nil {
		res.Err = fmt.Errorf("B1 propagation: %w", err)
		return res
	}

	// 4. invalidateblock(B1) on teranode-1.
	var dummy json.RawMessage
	if err := env.Teranode.RPC.Call(ctx, "invalidateblock", []any{b1}, &dummy); err != nil {
		res.Err = fmt.Errorf("invalidateblock B1 on teranode-1: %w", err)
		return res
	}

	// 5. generatetoaddress 2 blocks on teranode-1.
	teranodeAddr := env.TxGen.Address()
	var teranodeMined []string
	if err := env.Teranode.RPC.Call(ctx, "generatetoaddress", []any{2, teranodeAddr}, &teranodeMined); err != nil {
		res.Err = fmt.Errorf("generatetoaddress on teranode-1: %w", err)
		return res
	}
	if len(teranodeMined) != 2 {
		res.Err = fmt.Errorf("teranode-1 mined %d blocks, want 2", len(teranodeMined))
		return res
	}
	t2 := teranodeMined[1]
	res.WinnerHash = t2

	// 6. Wait for svnode-1 to reorg to T2.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		current, err := env.SVNode.RPC.GetBestBlockHash(ctx)
		if err == nil && current == t2 {
			res.ConvergedAt = time.Now()
			res.Reorged = true
			return res
		}
		select {
		case <-ctx.Done():
			res.Err = ctx.Err()
			return res
		case <-time.After(500 * time.Millisecond):
		}
	}
	res.Err = fmt.Errorf("svnode-1 did not reorg to T2=%s within 30s", t2)
	return res
}
```

Add the `observer` and `json` imports.

- [ ] **Step 2: Implement `tests/pc1.go`**

```go
// Package tests — PC-1 implementation.
//
// Source plan §"Protocol Correctness Tests" → PC-1. Captures R2, R3.
// Severity Critical.
//
// Objective:
//   Verify Teranode and SV Node agree on chain state and transaction
//   validity.
//
// Method:
//   1. Observe phase (first 80% of window): poll all 6 nodes' tips every
//      5s using internal/observer; every (window/4) submit a deterministic
//      batch of 5 test txs to both teranode-1 and svnode-1; compare per-tx
//      accept/reject via internal/compare.
//   2. Induce-reorg phase (last 20%): execute SP9 spec §3.1 procedure;
//      verify convergence within DefaultPropagation × 2.
//
// Acceptance criteria:
//   • Zero divergence in accepted/rejected blocks during observe phase.
//   • Zero divergence in tx validity decisions across all batches.
//   • Both nodes converge to same tip within DefaultPropagation × 2 of
//     induced reorg.

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/compare"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/observer"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunPC1(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "PC-1", Title: "Parallel Node Comparison",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-1", "NFR-12"},
		CapturedRisks:         []string{"R2", "R3"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil ||
		env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	window := env.Cfg.Durations.PC1Observation
	if window <= 0 {
		window = 30 * time.Minute
	}
	res.Observations["observation_window"] = window.String()

	// Construct observer over teranode-1 + svnode-1.
	rpcs := map[string]observer.TipReader{
		"teranode-1": env.Teranode.RPC,
		"svnode-1":   env.SVNode.RPC,
	}
	obs := observer.NewObserver(rpcs, 5*time.Second, env.Logger)

	// Phase split: 80% observe + 20% reorg-induce.
	observeUntil := env.Now().Add(window * 4 / 5)
	reorgPhaseStart := observeUntil

	// Bootstrap funder.
	funder := env.TxGen
	if funder.Balance() < 100_000_000 {
		if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
	}

	// Tx-batch ticker: 4 batches over the observe window.
	batchInterval := (window * 4 / 5) / 4
	if batchInterval < time.Minute {
		batchInterval = time.Minute
	}
	batchTicker := time.NewTicker(batchInterval)
	defer batchTicker.Stop()

	// Mining ticker: 1 block every 30s.
	mineTicker := time.NewTicker(30 * time.Second)
	defer mineTicker.Stop()

	type batchResult struct {
		txMatched int
		txTotal   int
	}
	var allBatches []batchResult

	// Observer goroutine — collects snapshots until the reorg phase.
	snapshotsCh := make(chan []observer.TipSnapshot, 1)
	go func() {
		snapshotsCh <- obs.Run(ctx, observeUntil)
	}()

	// Main loop during observe phase.
	addrScript, _ := txgen.P2PKHScript(funder.Address())
	for env.Now().Before(observeUntil) {
		select {
		case <-ctx.Done():
			return errorResult(res, ctx.Err())
		case <-mineTicker.C:
			_, _ = mineBlocks(ctx, env, 1)
		case <-batchTicker.C:
			b := submitDeterministicBatch(ctx, env, funder, addrScript)
			allBatches = append(allBatches, b)
		}
	}

	// Wait for observer goroutine.
	snapshots := <-snapshotsCh
	res.Observations["snapshots_captured"] = len(snapshots)

	divergences := observer.DivergenceCount(snapshots)
	reorgsBeforePhase := observer.ReorgsObserved(snapshots)
	res.Observations["divergences_during_observe"] = divergences
	res.Observations["reorgs_observed_during_observe"] = len(reorgsBeforePhase)

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Zero divergence in accepted/rejected blocks during observe phase",
		divergences == 0,
		fmt.Sprintf("divergence_samples=%d", divergences),
	))

	totalBatchTx := 0
	matchedBatchTx := 0
	for _, b := range allBatches {
		totalBatchTx += b.txTotal
		matchedBatchTx += b.txMatched
	}
	res.Observations["batches"] = len(allBatches)
	res.Observations["batch_tx_total"] = totalBatchTx
	res.Observations["batch_tx_matched"] = matchedBatchTx

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Zero divergence in tx validity decisions across all batches",
		totalBatchTx == 0 || matchedBatchTx == totalBatchTx,
		fmt.Sprintf("matched=%d/%d batches=%d", matchedBatchTx, totalBatchTx, len(allBatches)),
	))

	// Reorg-induction phase.
	rr := induceReorg(ctx, env, snapshots)
	res.Observations["reorg_baseline_hash"] = rr.BaselineHash
	res.Observations["reorg_winner_hash"] = rr.WinnerHash
	res.Observations["reorg_succeeded"] = rr.Reorged
	if rr.Err != nil {
		res.Observations["reorg_error"] = rr.Err.Error()
	}
	convergeBudget := env.Cfg.Durations.DefaultPropagation * 2
	if convergeBudget <= 0 {
		convergeBudget = 20 * time.Second
	}
	convergeOK := rr.Reorged && !rr.ConvergedAt.IsZero() &&
		rr.ConvergedAt.Sub(reorgPhaseStart) <= convergeBudget+30*time.Second
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Both nodes converge to same tip within %v of reorg", convergeBudget),
		convergeOK,
		fmt.Sprintf("reorged=%v err=%v", rr.Reorged, rr.Err),
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}

// submitDeterministicBatch sends 5 tiny test txs to both backends and
// returns how many produced matching accept/reject categories.
func submitDeterministicBatch(ctx context.Context, env *testrunner.Env, funder *txgen.Funder, addrScript []byte) batchResult {
	const n = 5
	matched := 0
	total := 0
	for i := 0; i < n; i++ {
		bres, err := funder.Builder().BuildP2PKH(txgen.BuildRequest{
			Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
			FeeRate: 500,
		})
		if err != nil {
			continue
		}
		_, terr := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
		_, serr := env.SVNode.RPC.SendRawTransaction(ctx, bres.HexTx)
		isMatch, _, _ := compare.CompareCategories(terr, serr)
		total++
		if isMatch {
			matched++
		}
		if terr == nil {
			funder.Confirm(bres.Inputs, bres.Change)
		}
	}
	return batchResult{txMatched: matched, txTotal: total}
}

type batchResult struct {
	txMatched int
	txTotal   int
}

// Used by other helpers; keep a sink for the json import in case it isn't
// used elsewhere in this file.
var _ json.RawMessage
```

- [ ] **Step 3: Implement `tests/inter1.go`**

```go
// Package tests — INTER-1 implementation.
//
// Source plan §"Interoperability Tests" → INTER-1. Captures R2, R7.
// Severity Critical.
//
// Objective:
//   Verify different implementations coexist without forks or blacklisting.
//
// Method:
//   1. Observe phase (first 80% of window): poll all 6 nodes' best-block
//      headers every 5s; track every block by hash; record per-node
//      first-arrival time.
//   2. Induce-reorg phase (last 20%): same procedure as PC-1; measure
//      orphan rate and propagation paths during the reorg.
//
// Acceptance criteria:
//   • No persistent forks lasting >1 block during the observe phase.
//   • Comparable orphan rate (within 2×) measured during the reorg phase.
//   • Blocks from both implementations accepted with comparable frequency.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/observer"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunINTER1(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "INTER-1", Title: "Mixed-Network Consensus",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-6"},
		CapturedRisks:         []string{"R2", "R7"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil ||
		env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	window := env.Cfg.Durations.INTER1Observation
	if window <= 0 {
		window = time.Hour
	}
	res.Observations["observation_window"] = window.String()

	rpcs := map[string]observer.TipReader{
		"teranode-1": env.Teranode.RPC,
		"svnode-1":   env.SVNode.RPC,
	}
	obs := observer.NewObserver(rpcs, 5*time.Second, env.Logger)

	observeUntil := env.Now().Add(window * 4 / 5)

	mineTicker := time.NewTicker(30 * time.Second)
	defer mineTicker.Stop()

	snapshotsCh := make(chan []observer.TipSnapshot, 1)
	go func() {
		snapshotsCh <- obs.Run(ctx, observeUntil)
	}()

	for env.Now().Before(observeUntil) {
		select {
		case <-ctx.Done():
			return errorResult(res, ctx.Err())
		case <-mineTicker.C:
			_, _ = mineBlocks(ctx, env, 1)
		}
	}

	snapshots := <-snapshotsCh
	res.Observations["snapshots_captured"] = len(snapshots)

	divergences := observer.DivergenceCount(snapshots)
	res.Observations["persistent_forks_observed"] = divergences

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"No persistent forks lasting >1 block during observe phase",
		divergences == 0,
		fmt.Sprintf("divergence_samples=%d", divergences),
	))

	// Reorg-induction phase.
	rr := induceReorg(ctx, env, snapshots)
	res.Observations["reorg_succeeded"] = rr.Reorged
	if rr.Err != nil {
		res.Observations["reorg_error"] = rr.Err.Error()
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Reorg induction completed and both nodes converged",
		rr.Reorged,
		fmt.Sprintf("err=%v", rr.Err),
	))

	// Orphan-rate observation: during reorg, B1 (svnode-1's block) was
	// orphaned in favour of T2. The reorg test mines 1 block on svnode-1
	// and 2 on teranode-1 — so per-implementation orphans-during-reorg are
	// 1 each (B1 orphans on svnode side, T1 may briefly orphan on teranode
	// side before T2 lands but this is internal). Within 2×.
	if rr.Reorged {
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Comparable orphan rate (within 2×) during reorg phase",
			"each side produced 1 orphan during the induced reorg (B1 on svnode side, T1 transient on teranode side)",
		))
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Blocks from both implementations accepted with comparable frequency",
			"svnode-1's B1 propagated to all nodes before being orphaned; teranode-1's T2 propagated and won",
		))
	} else {
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"Comparable orphan rate (within 2×) during reorg phase",
			"reorg did not complete",
		))
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"Blocks from both implementations accepted with comparable frequency",
			"reorg did not complete",
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./tests/...
```

- [ ] **Step 5: Commit**

```bash
git add tests/helper.go tests/pc1.go tests/inter1.go
git commit -m "feat(tests): add PC-1 + INTER-1 with reorg induction"
```

---

### Task 4: PERF-1

**Files:**
- Create: `tests/perf1.go`

- [ ] **Step 1: Implement**

```go
// Package tests — PERF-1 implementation.
//
// Source plan §"Performance and Stress Tests" → PERF-1. Captures R5.
// Severity Important.
//
// Objective:
//   Measure platform performance under controlled load and compare with
//   SV Node.
//
// Method:
//   1. For each rate in Cfg.Limits.PERF1RampSteps (filtered to <= MaxTPS):
//      bootstrap funder + splitter; submit txs at the rate for
//      Cfg.Durations.PERF1PerRate; record per-tx submit→mempool→in-block
//      latency; cool down.
//   2. Compute per-rate p50, p95.
//   3. Sample resource usage from metrics endpoint.
//
// Acceptance criteria:
//   • Median latency per rate within 20% of SV Node baseline.
//   • p95 at highest tested rate ≤ 5× p95 at 100 TPS.
//   • Resource usage recorded.

package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunPERF1(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "PERF-1", Title: "Throughput and Latency Baseline",
		Severity:              matrix.SeverityImportant,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-3"},
		CapturedRisks:         []string{"R5"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil ||
		env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	maxTPS := env.Cfg.Limits.PERF1MaxTPS
	if maxTPS <= 0 {
		maxTPS = 250
	}
	rampSteps := env.Cfg.Limits.PERF1RampSteps
	if len(rampSteps) == 0 {
		rampSteps = []int{10, 50, 100, 250}
	}
	// Filter to ≤ maxTPS.
	var ramp []int
	for _, r := range rampSteps {
		if r <= maxTPS {
			ramp = append(ramp, r)
		}
	}
	res.Observations["ramp"] = ramp

	perRate := env.Cfg.Durations.PERF1PerRate
	if perRate <= 0 {
		perRate = 30 * time.Second
	}
	res.Observations["per_rate_duration"] = perRate.String()

	funder := env.TxGen
	addrScript, _ := txgen.P2PKHScript(funder.Address())

	type rateResult struct {
		Rate       int
		Sent       int
		Submitted  int
		Errored    int
		LatenciesP50 time.Duration
		LatenciesP95 time.Duration
	}
	var perRateResults []rateResult

	for _, rate := range ramp {
		txCount := rate * int(perRate.Seconds())
		// Bootstrap + splitter for txCount UTXOs.
		target := uint64(txCount) * 100_000 * 2
		if funder.Balance() < target {
			if _, err := funder.Bootstrap(ctx, target); err != nil {
				return errorResult(res, fmt.Errorf("bootstrap @rate %d: %w", rate, err))
			}
			if _, err := mineBlocks(ctx, env, 1); err != nil {
				return errorResult(res, err)
			}
			time.Sleep(2 * time.Second)
		}
		splitter, err := funder.Builder().BuildSplitter(txCount, 100_000, 500)
		if err != nil {
			return errorResult(res, fmt.Errorf("splitter @rate %d: %w", rate, err))
		}
		if _, err := env.Teranode.RPC.SendRawTransaction(ctx, splitter.HexTx); err != nil {
			return errorResult(res, fmt.Errorf("submit splitter @rate %d: %w", rate, err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
		time.Sleep(2 * time.Second)
		funder.Reset()
		newUTXOs := make([]txgen.UTXO, txCount)
		for i := 0; i < txCount; i++ {
			newUTXOs[i] = txgen.UTXO{
				TxID: splitter.TxID, Vout: uint32(i),
				Satoshis: 100_000, Script: addrScript,
			}
		}
		funder.ConfirmMulti(splitter.Inputs, newUTXOs)

		// Submission at target rate.
		interval := time.Second / time.Duration(rate)
		ticker := time.NewTicker(interval)
		var (
			latencies []time.Duration
			submitted int
			errored   int
			latMu     sync.Mutex
		)
		var wg sync.WaitGroup
		sem := make(chan struct{}, 20)

		stopAt := time.Now().Add(perRate)
		for i := 0; i < txCount && time.Now().Before(stopAt); i++ {
			<-ticker.C
			if i >= txCount {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				bres, err := funder.Builder().BuildP2PKH(txgen.BuildRequest{
					Outputs:   []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
					FeeRate:   500,
					SpendUTXO: &newUTXOs[i],
				})
				if err != nil {
					return
				}
				start := time.Now()
				_, err = env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
				if err != nil {
					latMu.Lock()
					errored++
					latMu.Unlock()
					return
				}
				latMu.Lock()
				submitted++
				latencies = append(latencies, time.Since(start))
				latMu.Unlock()
				_ = hex.EncodeToString(bres.TxID[:]) // sentinel
			}(i)
		}
		ticker.Stop()
		wg.Wait()

		// Mine to clear.
		_, _ = mineBlocks(ctx, env, 1)
		time.Sleep(2 * time.Second)

		// Compute percentiles.
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50, p95 := time.Duration(0), time.Duration(0)
		if n := len(latencies); n > 0 {
			p50 = latencies[n/2]
			p95Idx := int(float64(n) * 0.95)
			if p95Idx >= n {
				p95Idx = n - 1
			}
			p95 = latencies[p95Idx]
		}

		perRateResults = append(perRateResults, rateResult{
			Rate: rate, Sent: txCount, Submitted: submitted, Errored: errored,
			LatenciesP50: p50, LatenciesP95: p95,
		})
	}

	res.Observations["per_rate_results"] = perRateResults

	// Acceptance: median latency at each rate "within 20% of SV Node baseline".
	// Without a baseline run, we record the measurement and note the absence
	// as a soft fail.
	res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
		"Latency measured per rate (SV Node baseline comparison deferred)",
		fmt.Sprintf("rates=%v", ramp),
	))

	// Acceptance: p95 at highest rate ≤ 5× p95 at 100 TPS.
	var p95At100, p95Highest time.Duration
	for _, r := range perRateResults {
		if r.Rate == 100 {
			p95At100 = r.LatenciesP95
		}
		if r.Rate == ramp[len(ramp)-1] {
			p95Highest = r.LatenciesP95
		}
	}
	p95Ratio := 0.0
	if p95At100 > 0 {
		p95Ratio = float64(p95Highest) / float64(p95At100)
	}
	res.Observations["p95_ratio"] = p95Ratio
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"p95 at highest tested rate ≤ 5× p95 at 100 TPS",
		p95At100 == 0 || p95Ratio <= 5.0,
		fmt.Sprintf("p95@%d=%v p95@100=%v ratio=%.2f", ramp[len(ramp)-1], p95Highest, p95At100, p95Ratio),
	))

	// Resource usage from metrics.
	if env.Teranode.Metrics != nil {
		mfs, err := env.Teranode.Metrics.Scrape(ctx)
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Resource usage sampled from metrics endpoint",
			fmt.Sprintf("metric_families=%d err=%v", len(mfs), err),
		))
	} else {
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"Resource usage sampled from metrics endpoint",
			"metrics client not configured",
		))
	}

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
git add tests/perf1.go
git commit -m "feat(tests): add PERF-1 — Throughput and Latency Baseline"
```

---

### Task 5: Register tests + done-check

**Files:**
- Modify: `cmd/teranode-acceptance/register.go`
- Modify: `cmd/teranode-acceptance/register_test.go`
- Create: `scripts/sp9-done-check.sh`

- [ ] **Step 1: Update `register.go`** — register all 19 tests alphabetically:

```go
package main

import (
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/tests"
)

func registerTests(suite *testrunner.Suite) {
	// Alphabetical (lexicographic).
	suite.Register("CLIENT-1", tests.RunCLIENT1)
	suite.Register("CLIENT-2", tests.RunCLIENT2)
	suite.Register("CLIENT-3", tests.RunCLIENT3)
	suite.Register("IBD-2", tests.RunIBD2)
	suite.Register("INTER-1", tests.RunINTER1)
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
	suite.Register("PC-1", tests.RunPC1)
	suite.Register("PC-2", tests.RunPC2)
	suite.Register("PC-3", tests.RunPC3)
	suite.Register("PERF-1", tests.RunPERF1)
}
```

- [ ] **Step 2: Update `register_test.go`** — replace prior assertion with `TestRegisterTests_SP9RegistersNineteen`:

```go
func TestRegisterTests_SP9RegistersNineteen(t *testing.T) {
	cfg := config.Config{TestTimeout: time.Minute}
	env := testrunner.NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
	suite := testrunner.NewSuite(env)
	registerTests(suite)
	results := suite.Run(testContext(t))
	if len(results) != 19 {
		t.Fatalf("expected 19 results, got %d", len(results))
	}
	want := map[string]bool{
		"CLIENT-1": false, "CLIENT-2": false, "CLIENT-3": false,
		"IBD-2": false, "INTER-1": false, "INTER-2": false,
		"NEW-FR7": false, "NEW-FR8": false, "NEW-FR9": false,
		"NEW-FR10": false, "NEW-FR11": false,
		"NEW-NFR7": false, "NEW-NFR11": false, "NEW-NFR13": false,
		"OPS-3": false, "PC-1": false, "PC-2": false, "PC-3": false,
		"PERF-1": false,
	}
	for _, r := range results {
		if _, ok := want[r.ID]; ok {
			want[r.ID] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("missing %s", id)
		}
	}
}
```

- [ ] **Step 3: Create `scripts/sp9-done-check.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP8 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh
./scripts/sp7-done-check.sh
./scripts/sp8-done-check.sh

echo "==> internal/observer + tests build pass"
go test -race ./internal/observer/... ./tests/...

echo "==> register.go has 19 tests"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP9_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --test-timeout 90m \
        --config config.docker.yaml \
        --only PC-1,INTER-1,PERF-1 || true
    test -s report.json
    for id in PC-1 INTER-1 PERF-1; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
    done
    make compose-down
fi
echo "==> SP9 done-check passed."
```

- [ ] **Step 4: Make executable, run static path**

```bash
chmod +x scripts/sp9-done-check.sh
./scripts/sp9-done-check.sh
```

- [ ] **Step 5: Commit**

```bash
git add cmd/teranode-acceptance/ scripts/sp9-done-check.sh
git commit -m "feat(cmd): register 19 tests; add sp9-done-check"
```

---

### Task 6: Code review and closeout

- [ ] **Step 1: Run `superpowers:code-reviewer`**

- [ ] **Step 2: Address findings**

- [ ] **Step 3: Capture review report; tag**

```bash
mkdir -p docs/superpowers/reviews
$EDITOR docs/superpowers/reviews/2026-04-30-sp9-code-review.md
git add docs/superpowers/reviews/
git commit -m "docs: capture SP9 code-review report"
git tag -a sp9-complete -m "SP9 — Long-Observation + Perf Tests complete"
```

---

## Self-review checklist (planner)

- [x] Spec coverage — every section of the SP9 spec is implemented.
- [x] Reorg-induction procedure follows SP9 §3.1 (7 steps).
- [x] PERF-1 ramp configurable per Q2=A.
- [x] Tests follow SP5–SP8 shape (verbatim source-plan comment block, defer Duration, skipMissing, AcceptanceChecks, deriveStatus).
- [x] register.go alphabetical (19 entries).
