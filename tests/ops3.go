// Package tests — OPS-3 implementation.
//
// Source plan §"Operational and Failure-Mode Tests" → OPS-3.
// Captures risk R6. Acceptance criteria from NFR-10. Severity Important.
//
// Objective:
//
//	Verify TNG can monitor platform health and performance.
//
// Method:
//  1. HTTP-GET metrics_url; verify 200 + parseable Prometheus exposition.
//  2. Verify presence of metric categories: chain tip height, sync status,
//     mempool size, transaction throughput, block validation latency.
//  3. HTTP-GET health_url; verify 200 + parseable status body.
//
// Acceptance criteria:
//   - Metrics endpoint returns 200, valid format.
//   - All five required metric categories present.
//   - Health endpoint returns 200.
//
// Implementation notes:
//   - Uses env.Teranode.Metrics + env.Teranode.Health from SP3.
//   - Skips with reason if either client is nil.
//   - Metric names sourced from SP2 discovery (commit 11f5fa6a8). If the
//     pinned image v0.15.0-beta-2 has renamed any metric, expect FAIL on
//     that specific check; operator updates the constant set in this file.
package tests

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunOPS3(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "OPS-3", Title: "Observability and Monitoring",
		Severity:              matrix.SeverityImportant,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-10"},
		CapturedRisks:         []string{"R6"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.Metrics == nil || env.Teranode.Health == nil {
		return skipMissing(res, "Teranode metrics or health client not configured")
	}

	// (1) /metrics returns 200 + parseable.
	mfs, err := env.Teranode.Metrics.Scrape(ctx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Metrics endpoint returns 200 with parseable Prometheus body",
		err == nil && len(mfs) > 0,
		fmt.Sprintf("scraped %d metric families; err=%v", len(mfs), err),
	))

	// (2) Required metric categories.
	requiredMetrics := []struct{ name, category string }{
		{"teranode_blockassembly_best_block_height", "chain tip height"},
		{"teranode_blockchain_fsm_current_state", "sync status"},
		{"teranode_blockassembly_transactions", "mempool size"},
		{"teranode_validator_transactions_count", "transaction throughput"},
		{"teranode_blockvalidation_validate_block_count", "block validation latency"},
	}
	for _, m := range requiredMetrics {
		_, present := mfs[m.name]
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("Metric %q present (%s)", m.name, m.category),
			present,
			fmt.Sprintf("present=%v", present),
		))
	}

	// (3) Health readiness returns 200 + parseable.
	rep, herr := env.Teranode.Health.Readiness(ctx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Health endpoint returns 200 with JSON-parseable body",
		herr == nil && rep.Status != "",
		fmt.Sprintf("status=%q services=%d err=%v", rep.Status, len(rep.Services), herr),
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
