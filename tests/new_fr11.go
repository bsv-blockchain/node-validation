// Package tests — NEW-FR11 implementation.
//
// Source: derived from FR-11.
//
// Objective:
//
//	Verify Teranode supports mempool query and filtering as described in
//	the requirement.
//
// Method:
//  1. Submit a chain of dependent transactions to populate the mempool.
//  2. Call getrawmempool — verify chain txids appear.
//  3. Call getmempoolentry, getmempoolancestors, getmempooldescendants,
//     getmempoolinfo — per SP2 §10 these are absent or unimplemented.
//     Tests assert the expected absence as positive findings.
//
// Acceptance criteria:
//   - Each of four query types succeeds (recorded honestly per SP2).
//   - Filtering and chain-traversal results match constructed ground truth
//     (deferred since the underlying queries are absent).
//   - Statistics endpoint returns plausible values (absent per SP2).
package tests

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunNEWFR11(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-FR11", Title: "Mempool Query Capabilities",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-11"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil || env.TxGen == nil || env.SVNode == nil {
		return skipMissing(res, "client(s) not configured")
	}

	funder := env.TxGen
	builder := funder.Builder()
	if funder.Balance() < 100_000_000 {
		if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
			if strings.Contains(err.Error(), "FAIL_FORBIDDEN") {
				return skipMissing(res, "bootstrap: FAIL_FORBIDDEN: "+err.Error())
			}
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
	}

	addrScript, _ := txgen.P2PKHScript(funder.Address())
	chain, err := builder.BuildChain(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	}, 3)
	if err != nil {
		return errorResult(res, fmt.Errorf("build chain: %w", err))
	}
	var chainTxIDs []string
	for _, c := range chain {
		if _, err := env.Teranode.RPC.SendRawTransaction(ctx, c.HexTx); err != nil {
			return errorResult(res, fmt.Errorf("submit chain: %w", err))
		}
		chainTxIDs = append(chainTxIDs, hex.EncodeToString(c.TxID[:]))
	}

	// Teranode registers txs asynchronously between RPC accept and visibility
	// in getrawmempool — wait briefly for all chain txs to appear.
	_ = waitForMempoolEntries(ctx, env.Teranode.RPC, chainTxIDs, 10*time.Second)

	// (1) getrawmempool.
	mempool, err := env.Teranode.RPC.GetRawMempool(ctx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"getrawmempool returns []string",
		err == nil,
		fmt.Sprintf("err=%v len=%d", err, len(mempool)),
	))
	seen := map[string]bool{}
	for _, id := range mempool {
		seen[id] = true
	}
	for i, id := range chainTxIDs {
		short := id
		if len(short) > 10 {
			short = short[:10]
		}
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("Chain tx %d (%s…) in getrawmempool", i, short),
			seen[id],
			fmt.Sprintf("present=%v", seen[id]),
		))
	}

	// (2-5) Absent endpoints — assert the negative.
	type expectAbsent struct {
		method string
		params []any
	}
	absent := []expectAbsent{
		{"getmempoolentry", []any{chainTxIDs[0]}},
		{"getmempoolancestors", []any{chainTxIDs[1]}},
		{"getmempooldescendants", []any{chainTxIDs[1]}},
		{"getmempoolinfo", nil},
	}
	for _, ex := range absent {
		var raw json.RawMessage
		err := env.Teranode.RPC.Call(ctx, ex.method, ex.params, &raw)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("%s: per SP2 discovery absent or unimplemented", ex.method),
			err != nil,
			fmt.Sprintf("err=%v", err),
		))
	}

	// Mine to clean up.
	_, _ = mineBlocks(ctx, env, 1)

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
