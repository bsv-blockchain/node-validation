// Package tests — NEW-FR7 implementation.
//
// Source: derived from FR-7. Captures R1.
//
// Objective:
//
//	Verify Teranode accepts and propagates chains of dependent unconfirmed
//	transactions to the depth the source plan specifies.
//
// Method:
//  1. Build a chain of Cfg.Limits.FR7ChainDepth (default 25) dependent
//     unconfirmed transactions via Builder.BuildChain.
//  2. Submit each link via Teranode RPC sendrawtransaction; record per-link
//     acceptance.
//  3. Wait briefly for P2P propagation (5s).
//  4. Get SV Node getrawmempool; verify all chain txids are visible.
//  5. Mine 1 block; wait for tip propagation; fetch the block from
//     Teranode REST; verify all chain txs are in it.
//
// Acceptance criteria:
//   - Chain of depth ≥25 fully accepted into Teranode mempool.
//   - Chain visible in SV Node mempool within default_propagation seconds.
//   - All chain members eventually mined without intermediate confirmations.
//   - Behaviour consistent with SV Node (= mempool visibility per spec §4.1).
package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
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
		if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
			if strings.Contains(err.Error(), "FAIL_FORBIDDEN") {
				return skipMissing(res, "bootstrap: Aerospike lock contention: "+err.Error())
			}
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

	blockBytes, err := env.Teranode.REST.GetBlockLegacyBytes(ctx, mined[0])
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
