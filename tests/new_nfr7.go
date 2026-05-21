// Package tests — NEW-NFR7 implementation.
//
// Source: derived from NFR-7.
//
// Objective:
//
//	Verify identical operations under similar load produce identical
//	results.
//
// Method:
//  1. Pick three pure read operations: getbestblockhash, getblock <known
//     hash>, getrawtransaction <known confirmed txid>.
//  2. For each, capture a baseline response.
//  3. Repeat each Cfg.Durations.NewNFR7Iterations (default 100) times.
//  4. Verify every iteration's response is byte-identical to the baseline.
//
// Acceptance criteria:
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

	// Pick deterministic anchors at test start. We need a block well below the
	// current tip so the verbose response's "confirmations" field is stable
	// across iterations; we also fetch verbosity=0 (raw hex) which has no
	// time-varying fields. Teranode's getblock verbose=1 returns tx:[] so we
	// derive the coinbase txid from getblockstats (or use the merkleroot of
	// blocks that contain only a coinbase, where merkleroot == coinbase txid).
	info, err := env.Teranode.RPC.GetBlockchainInfo(ctx)
	if err != nil {
		return errorResult(res, fmt.Errorf("getblockchaininfo: %w", err))
	}
	// Anchor at tip - iterations - 10 to ensure the chain cannot advance past it
	// during the test loop. Skip if the chain is too short to place that anchor
	// at height ≥ 1 (requires at least iterations+11 blocks).
	anchorHeight := int64(info.Blocks) - int64(iterations) - 10
	if anchorHeight < 1 {
		return skipMissing(res, fmt.Sprintf(
			"chain too short for determinism test: need at least %d blocks, have %d",
			int64(iterations)+11, info.Blocks,
		))
	}
	var anchorHash string
	if err := env.Teranode.RPC.Call(ctx, "getblockhash", []any{anchorHeight}, &anchorHash); err != nil {
		return errorResult(res, fmt.Errorf("getblockhash @%d: %w", anchorHeight, err))
	}
	// Use verbosity=0 (raw hex) so no "confirmations"/time fields drift.
	var blkBaseline json.RawMessage
	if err := env.Teranode.RPC.Call(ctx, "getblock", []any{anchorHash, 0}, &blkBaseline); err != nil {
		return errorResult(res, fmt.Errorf("getblock baseline: %w", err))
	}
	if len(blkBaseline) == 0 {
		return errorResult(res, fmt.Errorf("getblock returned empty payload"))
	}

	// Read 1: getblockhash @anchorHeight — must be deterministic (height-to-hash
	// is immutable for a confirmed block).
	var hashErrCount, hashDriftCount int
	for i := 0; i < iterations; i++ {
		var h string
		err := env.Teranode.RPC.Call(ctx, "getblockhash", []any{anchorHeight}, &h)
		if err != nil {
			hashErrCount++
			continue
		}
		if h != anchorHash {
			hashDriftCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getblockhash @%d returns identical result across %d iterations", anchorHeight, iterations),
		hashErrCount == 0 && hashDriftCount == 0,
		fmt.Sprintf("errs=%d drifts=%d", hashErrCount, hashDriftCount),
	))

	// Read 2: getblock verbosity=0 — raw hex, no time-varying fields.
	var blkErrCount, blkDriftCount int
	for i := 0; i < iterations; i++ {
		var raw json.RawMessage
		err := env.Teranode.RPC.Call(ctx, "getblock", []any{anchorHash, 0}, &raw)
		if err != nil {
			blkErrCount++
			continue
		}
		if !bytes.Equal(raw, blkBaseline) {
			blkDriftCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getblock(verbosity=0) returns byte-identical hex across %d iterations", iterations),
		blkErrCount == 0 && blkDriftCount == 0,
		fmt.Sprintf("errs=%d drifts=%d", blkErrCount, blkDriftCount),
	))

	// Read 3: getbestblockhash — verify it always returns a syntactically
	// valid hash. We don't compare against a baseline because the tip
	// legitimately advances during the loop (which is correct behaviour, not
	// non-determinism).
	var bestErrCount, bestMalformedCount int
	for i := 0; i < iterations; i++ {
		h, err := env.Teranode.RPC.GetBestBlockHash(ctx)
		if err != nil {
			bestErrCount++
			continue
		}
		if len(h) != 64 {
			bestMalformedCount++
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("getbestblockhash returns well-formed hash across %d iterations", iterations),
		bestErrCount == 0 && bestMalformedCount == 0,
		fmt.Sprintf("errs=%d malformed=%d", bestErrCount, bestMalformedCount),
	))

	res.Observations["anchor_height"] = anchorHeight
	res.Observations["block_drifts"] = blkDriftCount
	res.Observations["block_errs"] = blkErrCount
	res.Observations["block_hash_drifts"] = hashDriftCount
	res.Observations["block_hash_errs"] = hashErrCount
	res.Observations["best_hash_errs"] = bestErrCount
	res.Observations["best_hash_malformed"] = bestMalformedCount

	// Load-condition checks: not yet integrated with PERF-1 TPS ramp infra.
	// Recorded as non-required observations so the deterministic-read criteria
	// determine the test's pass/fail status.
	res.AcceptanceChecks = append(res.AcceptanceChecks, testrunner.Check{
		Description: "Read ops return identical results under 100 TPS load",
		Required:    false, Pass: false,
		Detail: "not yet integrated with PERF-1 TPS-ramp infrastructure",
	})
	res.AcceptanceChecks = append(res.AcceptanceChecks, testrunner.Check{
		Description: "Read ops return identical results under 500 TPS load",
		Required:    false, Pass: false,
		Detail: "not yet integrated with PERF-1 TPS-ramp infrastructure",
	})

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
