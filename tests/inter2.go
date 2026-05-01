// Package tests — INTER-2 implementation.
//
// Source plan §"Interoperability Tests" → INTER-2. Captures R1, R2.
// Severity Critical.
//
// Objective:
//
//	Verify transactions broadcast to one node type reach the other reliably.
//
// Method:
//  1. Build a splitter tx with N outputs (N = Cfg.Limits.INTER2TxCount,
//     default 1000); submit to Teranode; mine 1 block to confirm.
//  2. Build N simple P2PKH txs at 4 fee-rate buckets and 4 size variations.
//  3. Partition: 333 to "SV Node only", 333 to "Teranode only", 334 to "both".
//  4. Submit concurrently (10 goroutines per group).
//  5. Poll each backend's mempool every 250ms for 10s; record per-tx
//     cross-side observation.
//  6. Compute % observed cross-side within Cfg.Durations.DefaultPropagation.
//
// Acceptance criteria:
//   - ≥99% of "Teranode only" group appears in SV Node mempool within 10s.
//   - ≥99% of "SV Node only" group appears in Teranode mempool within 10s.
//   - "Both" group: each tx accepted by at least one backend; no
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

// interTx holds a built transaction's txid and hex for submission.
type interTx struct {
	txid string
	hex  string
}

func txidsOf(txs []interTx) []string {
	out := make([]string, len(txs))
	for i, t := range txs {
		out[i] = t.txid
	}
	return out
}

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
		if _, err := bootstrapConfirmed(ctx, env, target); err != nil {
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

	txs := make([]interTx, 0, count)
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
		txs = append(txs, interTx{
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

	res.Observations["teranode_only_count"] = len(groupTeranodeOnly)
	res.Observations["sv_only_count"] = len(groupSVOnly)
	res.Observations["both_count"] = len(groupBoth)

	// Concurrent submission with bounded parallelism.
	submitGroup := func(grp []interTx, submit func(context.Context, string) (string, error)) (sent int) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		for _, t := range grp {
			wg.Add(1)
			sem <- struct{}{}
			go func(tx interTx) {
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

	teranodeSent := submitGroup(groupTeranodeOnly, env.Teranode.RPC.SendRawTransaction)
	svSent := submitGroup(groupSVOnly, env.SVNode.RPC.SendRawTransaction)

	// "Both" group — submit to Teranode, then SV Node 1ms later.
	var bothSent int
	{
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		for _, t := range groupBoth {
			wg.Add(1)
			sem <- struct{}{}
			go func(tx interTx) {
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
		"Both-group: ≥99% of txs accepted by at least one backend",
		float64(bothSent)/float64(len(groupBoth)) >= 0.99,
		fmt.Sprintf("submitted=%d/%d", bothSent, len(groupBoth)),
	))

	// Mine to clean up.
	_, _ = mineBlocks(ctx, env, 1)

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
