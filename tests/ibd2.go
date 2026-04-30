// Package tests — IBD-2 implementation.
//
// Source plan §"IBD Tests" → IBD-2. Captures R3, R4. Severity Critical.
//
// Objective:
//
//	Verify Teranode correctly validates spending of historical UTXOs
//	with edge-case scripts.
//
// Method:
//  1. Load tests/testdata/historical_utxos.yaml (≥10 fixture spend txs).
//  2. Submit each to Teranode and SV Node.
//  3. Compare via internal/compare.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bsv-blockchain/node-validation/internal/compare"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

const fixturePathIBD2 = "tests/testdata/historical_utxos.yaml"

func RunIBD2(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "IBD-2", Title: "Historical UTXO Spend Verification",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-3"},
		CapturedRisks:         []string{"R3", "R4"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil {
		return skipMissing(res, "Teranode or SVNode RPC not configured")
	}

	fixtures, err := LoadFixtures(fixturePathIBD2)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return skipMissing(res, "fixture file not found: "+fixturePathIBD2)
		}
		return errorResult(res, err)
	}
	res.Observations["fixture_count"] = len(fixtures)

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"≥10 IBD-2 fixtures present (per source plan IBD-2)",
		len(fixtures) >= 10,
		fmt.Sprintf("loaded=%d", len(fixtures)),
	))

	matched := 0
	mismatches := []string{}
	for _, f := range fixtures {
		_, terr := env.Teranode.RPC.SendRawTransaction(ctx, f.HexTx)
		_, serr := env.SVNode.RPC.SendRawTransaction(ctx, f.HexTx)
		isMatch, tCat, sCat := compare.CompareCategories(terr, serr)
		if isMatch {
			matched++
		} else {
			mismatches = append(mismatches,
				fmt.Sprintf("%s: teranode=%s svnode=%s", f.ID, tCat, sCat))
		}
	}
	res.Observations["matched"] = matched
	if len(mismatches) > 0 {
		res.Observations["mismatches"] = mismatches[:min(len(mismatches), 10)]
	}

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"100% match on accept/reject decisions across all fixtures",
		matched == len(fixtures),
		fmt.Sprintf("matched=%d/%d mismatches=%d", matched, len(fixtures), len(mismatches)),
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
