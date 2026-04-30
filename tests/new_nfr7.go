// Package tests — NEW-NFR7 implementation.
//
// Source: derived from NFR-7.
//
// Objective:
//
//	Verify identical operations under similar load produce identical
//	results.
//
// Method (idle conditions only per SP7 spec §4.2 Q2=A):
//  1. Pick three pure read operations: getbestblockhash, getblock <known
//     hash>, getrawtransaction <known confirmed txid>.
//  2. For each, capture a baseline response.
//  3. Repeat each Cfg.Durations.NewNFR7Iterations (default 100) times.
//  4. Verify every iteration's response is byte-identical to the baseline.
//
// Acceptance criteria (from NFR-7):
//   - Read operations return identical results across iterations.
//   - No load-induced variation — DEFERRED to SP9 with note (PERF-1
//     infrastructure required for 100/500 TPS).
//   - Errors fall into well-defined codes — Pass if no errors observed
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
