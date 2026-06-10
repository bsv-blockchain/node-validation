// Package tests — NEW-FR8 implementation.
//
// Source: derived from FR-8.
//
// Status: RETIRED. FR-8 (transaction fee estimation) is covered externally
// by Arcade / Arc, not by the Teranode RPC, so this is not a valid test for
// this harness. The manifest marks NEW-FR8 as RESOLVED_EXTERNAL and the test
// is no longer registered in cmd/teranode-acceptance/register.go, so RunNEWFR8
// is not executed in a normal run. The implementation is retained for
// reference only.
//
// Objective:
//
//	Verify Teranode exposes a fee estimation API and that its predictions
//	correlate with observed inclusion latency.
//
// Method:
//  1. Discovery determines the fee-estimation endpoint. Per SP2 §9, the
//     endpoint estimatefee is registered but routes to handleUnimplemented
//     (returns ErrRPCUnimplemented = -1). Test reports FEATURE_NOT_AVAILABLE.
//  2. If the endpoint surprisingly works (drift since SP2), record the
//     response and flag the unexpected positive result.
//
// Acceptance criteria:
//   - Endpoint returns within 1 s.
//   - Estimates reflect recent block inclusion.
//   - Multiple priority levels supported.
//   - Standard-priority accuracy ≥ 80% over a 1-block horizon.
package tests

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWFR8(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-FR8", Title: "Fee Estimation Endpoint Validation",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-8"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil {
		return skipMissing(res, "Teranode RPC not configured")
	}

	var fee float64
	err := env.Teranode.RPC.Call(ctx, "estimatefee", []any{1}, &fee)
	if err != nil {
		if jsonrpc.IsErrorCode(err, -1) {
			res.Status = testrunner.StatusFeatureNotAvailable
			res.SkipReason = "estimatefee returns ErrRPCUnimplemented per SP2 discovery (services/rpc/Server.go:162 routes to handleUnimplemented)"
			res.Observations["err_code"] = -1
			return res
		}
		// Some other error — record as failed check.
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"estimatefee returned a non-unimplemented error",
			fmt.Sprintf("err=%v", err),
		))
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// Surprising — endpoint actually returned a value.
	res.Observations["fee"] = fee
	res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
		"estimatefee returned a value (unexpected per SP2 discovery)",
		fmt.Sprintf("fee=%v", fee),
	))
	res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
		"Multiple priority levels supported (economy/standard/priority)",
		"per SP2 only one priority level exists; cannot verify multi-tier accuracy",
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
