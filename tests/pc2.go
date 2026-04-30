// Package tests — PC-2 implementation.
//
// Source plan §"Protocol Correctness Tests" → PC-2. Captures R3.
// Severity Critical.
//
// Objective:
//
//	Verify Teranode correctly handles historical edge cases and script
//	execution flags.
//
// Method:
//  1. Load tests/testdata/historical_scripts.yaml (≥30 fixtures across
//     5 categories: complex-p2sh, restricted-opcodes, cleanstack,
//     minimaldata, malleability).
//  2. For each fixture, submit to Teranode and SV Node.
//  3. Compare accept/reject + rejection-category via internal/compare.
//
// Acceptance criteria (from PC-2):
//   - 100% match on valid/invalid decisions for all fixtures.
//   - Rejection categories match where applicable.
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

const fixturePathPC2 = "tests/testdata/historical_scripts.yaml"

func RunPC2(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "PC-2", Title: "Historical Script and Consensus Regression",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-1", "FR-3"},
		CapturedRisks:         []string{"R3"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil {
		return skipMissing(res, "Teranode or SVNode RPC not configured")
	}

	fixtures, err := LoadFixtures(fixturePathPC2)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return skipMissing(res, "fixture file not found: "+fixturePathPC2)
		}
		return errorResult(res, err)
	}
	res.Observations["fixture_count"] = len(fixtures)

	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"≥30 PC-2 fixtures present (per source plan PC-2)",
		len(fixtures) >= 30,
		fmt.Sprintf("loaded=%d", len(fixtures)),
	))

	matched := 0
	mismatches := []string{}
	perCategoryMatched := map[string]int{}
	perCategoryTotal := map[string]int{}

	for _, f := range fixtures {
		_, terr := env.Teranode.RPC.SendRawTransaction(ctx, f.HexTx)
		_, serr := env.SVNode.RPC.SendRawTransaction(ctx, f.HexTx)
		isMatch, tCat, sCat := compare.CompareCategories(terr, serr)
		perCategoryTotal[f.Category]++
		if isMatch {
			matched++
			perCategoryMatched[f.Category]++
		} else {
			short := f.ID
			mismatches = append(mismatches,
				fmt.Sprintf("%s: teranode=%s svnode=%s", short, tCat, sCat))
		}
	}
	res.Observations["matched"] = matched
	res.Observations["per_category"] = map[string]any{}
	pc := res.Observations["per_category"].(map[string]any)
	for cat, total := range perCategoryTotal {
		pc[cat] = fmt.Sprintf("%d/%d", perCategoryMatched[cat], total)
	}
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
