// internal/testrunner/pipeline_test.go
//
// End-to-end verdict-pipeline tests: Suite.Run → BuildReportModel →
// ComputeVerdict → exit code. Complement to verdict_test.go (which
// covers ComputeVerdict directly).

package testrunner

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func newPipelineEnv(t *testing.T) *Env {
	t.Helper()
	cfg := config.Config{
		Network:     config.NetworkTestnet,
		TestTimeout: 5 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return NewEnv(cfg, logger, matrix.Load(), nil)
}

// allOverridesPassing returns an overrides.File marking the 5 doc-review
// rows as PASS so a fully-green automated run can land at GO.
func allOverridesPassing() overrides.File {
	return overrides.File{
		Reviewer:   "test-reviewer",
		ReviewedAt: time.Now(),
		Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
}

// registerAllPassing registers every in-scope test ID with a no-op
// function returning a synthetic PASS Result. Optional override `id →
// status` lets a caller plant a specific outcome on chosen tests.
func registerAllPassing(suite *Suite, status map[string]Status) {
	m := suite.env.Manifest
	for _, id := range m.InScopeTestIDs() {
		id := id
		s := StatusPass
		if status != nil {
			if explicit, ok := status[id]; ok {
				s = explicit
			}
		}
		suite.Register(id, func(_ context.Context, _ *Env) Result {
			return Result{ID: id, Status: s}
		})
	}
}

func runPipeline(t *testing.T, status map[string]Status, ovr overrides.File) ReportModel {
	t.Helper()
	env := newPipelineEnv(t)
	suite := NewSuite(env)
	registerAllPassing(suite, status)
	results := suite.Run(context.Background())
	model, err := BuildReportModel(env, results, ovr, time.Now(), time.Now(), "test")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	return model
}

func TestPipeline_AllGreenWithOverrides_GO(t *testing.T) {
	model := runPipeline(t, nil, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s, want GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 0 {
		t.Errorf("got exit %d, want 0", model.Verdict.ExitCode)
	}
}

func TestPipeline_AllGreenNoOverrides_INCOMPLETE(t *testing.T) {
	model := runPipeline(t, nil, overrides.File{})
	if model.Verdict.Decision != "INCOMPLETE" {
		t.Errorf("got %s, want INCOMPLETE (Critical doc-review rows unsatisfied)", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 3 {
		t.Errorf("got exit %d, want 3", model.Verdict.ExitCode)
	}
}

func TestPipeline_OneCriticalFail_NO_GO(t *testing.T) {
	model := runPipeline(t, map[string]Status{"PC-1": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "NO_GO" {
		t.Errorf("got %s, want NO_GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 1 {
		t.Errorf("got exit %d, want 1", model.Verdict.ExitCode)
	}
}

func TestPipeline_OneImportantFail_CONDITIONAL(t *testing.T) {
	model := runPipeline(t, map[string]Status{"OPS-3": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "CONDITIONAL_GO" {
		t.Errorf("got %s, want CONDITIONAL_GO", model.Verdict.Decision)
	}
	if model.Verdict.ExitCode != 2 {
		t.Errorf("got exit %d, want 2", model.Verdict.ExitCode)
	}
}

func TestPipeline_ImportantSkipped_StillGO(t *testing.T) {
	// Important SKIPPED is acceptable per source plan ("Must Pass or Have Mitigation Plan").
	model := runPipeline(t, map[string]Status{"CLIENT-2": StatusSkipped}, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s (rationale=%q), want GO — SKIPPED Important is acceptable",
			model.Verdict.Decision, model.Verdict.Rationale)
	}
}

func TestPipeline_AdvisoryFail_NoVerdictDemotion(t *testing.T) {
	model := runPipeline(t, map[string]Status{"NEW-FR7": StatusFail}, allOverridesPassing())
	if model.Verdict.Decision != "GO" {
		t.Errorf("got %s, want GO — Advisory FAIL must not demote", model.Verdict.Decision)
	}
}

func TestPipeline_OverrideFail_NO_GO(t *testing.T) {
	ovr := allOverridesPassing()
	rejected := ovr.Overrides["IBD-1"]
	rejected.Decision = overrides.DecisionFail
	ovr.Overrides["IBD-1"] = rejected
	model := runPipeline(t, nil, ovr)
	if model.Verdict.Decision != "NO_GO" {
		t.Errorf("got %s, want NO_GO — explicit override FAIL should fail Critical doc-review row", model.Verdict.Decision)
	}
}

func TestPipeline_FullModel_HasAllRows(t *testing.T) {
	model := runPipeline(t, nil, allOverridesPassing())
	// Sanity: report shape.
	if got := len(model.Requirements); got != 24 {
		t.Errorf("requirements: %d, want 24", got)
	}
	if got := len(model.TestEnvironment); got != 3 {
		t.Errorf("test_environment: %d, want 3", got)
	}
	if got := len(model.TestCases); got != 24 {
		t.Errorf("test_cases: %d, want 24", got)
	}
	if got := len(model.Risks); got != 7 {
		t.Errorf("risks: %d, want 7", got)
	}
}
