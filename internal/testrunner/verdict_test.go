// internal/testrunner/verdict_test.go
package testrunner

import (
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func resultPass(id string, sev matrix.Severity) Result {
	return Result{ID: id, Severity: sev, Status: StatusPass}
}
func resultFail(id string, sev matrix.Severity) Result {
	return Result{ID: id, Severity: sev, Status: StatusFail}
}
func resultNotRun(id string, sev matrix.Severity) Result {
	return Result{ID: id, Severity: sev, Status: StatusNotRun}
}
func resultSkipped(id string, sev matrix.Severity) Result {
	return Result{ID: id, Severity: sev, Status: StatusSkipped}
}

func allCriticalPass() []Result {
	return []Result{
		resultPass("PC-1", matrix.SeverityCritical),
		resultPass("PC-2", matrix.SeverityCritical),
		resultPass("PC-3", matrix.SeverityCritical),
		resultPass("IBD-2", matrix.SeverityCritical),
		resultPass("INTER-1", matrix.SeverityCritical),
		resultPass("INTER-2", matrix.SeverityCritical),
		resultPass("CLIENT-1", matrix.SeverityCritical),
		resultPass("CLIENT-3", matrix.SeverityCritical),
	}
}

func allImportantPass() []Result {
	return []Result{
		resultPass("PERF-1", matrix.SeverityImportant),
		resultPass("OPS-3", matrix.SeverityImportant),
		resultPass("CLIENT-2", matrix.SeverityImportant),
	}
}

func TestVerdict_zeroResultsIsIncomplete(t *testing.T) {
	v := ComputeVerdict(nil, matrix.Load(), overrides.File{})
	if v.Decision != "INCOMPLETE" || v.ExitCode != 3 {
		t.Errorf("want INCOMPLETE/3, got %s/%d", v.Decision, v.ExitCode)
	}
}

func TestVerdict_allGreenButNoOverrides_isIncomplete(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	v := ComputeVerdict(res, matrix.Load(), overrides.File{})
	if v.Decision != "INCOMPLETE" {
		t.Errorf("want INCOMPLETE (IBD-1 doc-review missing), got %s", v.Decision)
	}
}

func TestVerdict_allGreenWithOverrides_isGo(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	ovr := overrides.File{
		Reviewer: "test", Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
	v := ComputeVerdict(res, matrix.Load(), ovr)
	if v.Decision != "GO" || v.ExitCode != 0 {
		t.Errorf("want GO/0, got %s/%d (%s)", v.Decision, v.ExitCode, v.Rationale)
	}
}

func TestVerdict_criticalFailIsNoGo(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	res[0] = resultFail("PC-1", matrix.SeverityCritical)
	v := ComputeVerdict(res, matrix.Load(), overrides.File{})
	if v.Decision != "NO_GO" || v.ExitCode != 1 {
		t.Errorf("want NO_GO/1, got %s/%d", v.Decision, v.ExitCode)
	}
}

func TestVerdict_criticalNotRunIsIncomplete(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	res[0] = resultNotRun("PC-1", matrix.SeverityCritical)
	v := ComputeVerdict(res, matrix.Load(), overrides.File{})
	if v.Decision != "INCOMPLETE" {
		t.Errorf("want INCOMPLETE, got %s", v.Decision)
	}
}

func TestVerdict_importantFailIsConditional(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	res[len(res)-1] = resultFail("CLIENT-2", matrix.SeverityImportant)
	ovr := overrides.File{
		Reviewer: "x", Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
	v := ComputeVerdict(res, matrix.Load(), ovr)
	if v.Decision != "CONDITIONAL_GO" || v.ExitCode != 2 {
		t.Errorf("want CONDITIONAL_GO/2, got %s/%d", v.Decision, v.ExitCode)
	}
}

func TestVerdict_importantSkippedIsAcceptable(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	// CLIENT-2 commonly skips when extended format isn't advertised.
	res[len(res)-1] = resultSkipped("CLIENT-2", matrix.SeverityImportant)
	ovr := overrides.File{
		Reviewer: "x", Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
	v := ComputeVerdict(res, matrix.Load(), ovr)
	if v.Decision != "GO" {
		t.Errorf("Important SKIPPED should still be GO, got %s (%s)", v.Decision, v.Rationale)
	}
}

func TestVerdict_overrideFailIsNoGo(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	ovr := overrides.File{
		Reviewer: "x",
		Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionFail, Artefacts: []string{"a"}, Note: "rejected"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
	v := ComputeVerdict(res, matrix.Load(), ovr)
	if v.Decision != "NO_GO" || v.ExitCode != 1 {
		t.Errorf("override FAIL on Critical doc-review should yield NO_GO/1, got %s/%d (%s)", v.Decision, v.ExitCode, v.Rationale)
	}
}

func TestVerdict_advisoryFailDoesNotDemote(t *testing.T) {
	res := append(allCriticalPass(), allImportantPass()...)
	res = append(res, resultFail("NEW-FR7", matrix.SeverityAdvisory))
	ovr := overrides.File{
		Reviewer: "x", Overrides: map[string]overrides.Override{
			"IBD-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"FR-4":  {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-1": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-8": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
			"NFR-9": {Decision: overrides.DecisionPass, Artefacts: []string{"a"}, Note: "ok"},
		},
	}
	v := ComputeVerdict(res, matrix.Load(), ovr)
	if v.Decision != "GO" {
		t.Errorf("Advisory FAIL should not demote, got %s", v.Decision)
	}
}
