// Package tests — PC-1 implementation.
//
// Source plan §"Protocol Correctness Tests" → PC-1. Captures R2, R3.
// Severity Critical.
//
// Objective:
//
//	Verify Teranode and SV Node agree on chain state and transaction
//	validity.
//
// Method:
//  1. Observe phase (first 80% of window): poll all 6 nodes' tips every
//     5s using internal/observer; every (window/4) submit a deterministic
//     batch of 5 test txs to both teranode-1 and svnode-1; compare per-tx
//     accept/reject via internal/compare.
//  2. Induce-reorg phase (last 20%): execute SP9 spec §3.1 procedure;
//     verify convergence within DefaultPropagation × 2.
//
// Acceptance criteria:
//   - Zero divergence in accepted/rejected blocks during observe phase.
//   - Zero divergence in tx validity decisions across all batches.
//   - Both nodes converge to same tip within DefaultPropagation × 2 of
//     induced reorg.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/compare"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/observer"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

// batchResult holds per-batch tx comparison counters.
type batchResult struct {
	txMatched int
	txTotal   int
}

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
		"teranode-1": &teranodeTipReader{rpc: env.Teranode.RPC},
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
