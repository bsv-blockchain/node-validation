// internal/testrunner/report_test.go
package testrunner

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func TestBuildReportModel_emptyResultsHasAllRows(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.Network = "testnet"
	model, err := BuildReportModel(env, nil, overrides.File{}, time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC), time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC), "test-version")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	if len(model.Requirements) != 24 {
		t.Errorf("requirements: want 24, got %d", len(model.Requirements))
	}
	if len(model.TestEnvironment) != 3 {
		t.Errorf("test_environment: want 3, got %d", len(model.TestEnvironment))
	}
	if len(model.TestCases) != 24 {
		t.Errorf("test_cases: want 24, got %d", len(model.TestCases))
	}
	if len(model.Risks) != 7 {
		t.Errorf("risks: want 7, got %d", len(model.Risks))
	}
	if model.Verdict.Decision != "INCOMPLETE" {
		t.Errorf("zero-test verdict: want INCOMPLETE, got %s", model.Verdict.Decision)
	}
}

func TestBuildReportModel_completenessInvariant(t *testing.T) {
	env := newTestEnv(t)
	// Sabotage the manifest: drop the last entry.
	env.Manifest.Entries = env.Manifest.Entries[:len(env.Manifest.Entries)-1]
	_, err := BuildReportModel(env, nil, overrides.File{}, time.Now(), time.Now(), "v")
	if err == nil {
		t.Fatal("expected completeness-invariant error")
	}
}

func TestBuildReportModel_riskMitigation(t *testing.T) {
	env := newTestEnv(t)
	res := []Result{
		{ID: "PC-1", Status: StatusPass, Severity: matrix.SeverityCritical},
		{ID: "PC-2", Status: StatusPass, Severity: matrix.SeverityCritical},
		{ID: "IBD-2", Status: StatusFail, Severity: matrix.SeverityCritical},
	}
	model, err := BuildReportModel(env, res, overrides.File{}, time.Now(), time.Now(), "v")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	var r3 RiskRow
	for _, r := range model.Risks {
		if r.ID == "R3" {
			r3 = r
			break
		}
	}
	// R3 covered by PC-1, PC-2, IBD-2; PC-1+PC-2 PASS, IBD-2 FAIL -> PARTIALLY_MITIGATED.
	if r3.MitigationStatus != "PARTIALLY_MITIGATED" {
		t.Errorf("R3 status: want PARTIALLY_MITIGATED, got %s", r3.MitigationStatus)
	}
}
