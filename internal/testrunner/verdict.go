// internal/testrunner/verdict.go
package testrunner

import (
	"fmt"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

// Verdict is the final go/no-go decision for a run.
type Verdict struct {
	Decision  string `json:"decision"`
	ExitCode  int    `json:"exit_code"`
	Rationale string `json:"rationale"`
}

// ComputeVerdict applies the Decision Framework to a slice of Results,
// taking reviewer overrides into account for documentation-review and
// contractual rows.
func ComputeVerdict(results []Result, m matrix.Manifest, ovr overrides.File) Verdict {
	byID := make(map[string]Result, len(results))
	for _, r := range results {
		byID[r.ID] = r
	}

	// 1. Critical FAIL/ERROR -> NO_GO.
	for _, e := range m.TestCases() {
		if e.Severity != matrix.SeverityCritical || e.TestCaseStatus != matrix.TCInScope {
			continue
		}
		r, ok := byID[e.ID]
		if !ok {
			continue
		}
		if r.Status == StatusFail || r.Status == StatusError {
			return Verdict{
				Decision:  "NO_GO",
				ExitCode:  1,
				Rationale: fmt.Sprintf("Critical test %s reported %s", e.ID, r.Status),
			}
		}
	}

	// 2. Critical NOT_RUN -> INCOMPLETE.
	var notRun []string
	for _, e := range m.TestCases() {
		if e.Severity != matrix.SeverityCritical || e.TestCaseStatus != matrix.TCInScope {
			continue
		}
		r, ok := byID[e.ID]
		if !ok || r.Status == StatusNotRun {
			notRun = append(notRun, e.ID)
		}
	}
	if len(notRun) > 0 {
		return Verdict{
			Decision:  "INCOMPLETE",
			ExitCode:  3,
			Rationale: "Critical tests not run: " + strings.Join(notRun, ", "),
		}
	}

	// 3. Critical entries needing documentation review without override -> INCOMPLETE.
	// Source plan §"Critical Requirements" includes IBD-1 (and by extension FR-4 because
	// that is what IBD-1 verifies). NFR-* contractual / long-term rows are not Critical
	// per source; we still require overrides for any critical row whose CoverageStatus is
	// DOCUMENTATION_REVIEW or whose TestCaseStatus is EXCLUDED_DOCUMENTATION.
	var needsOverride []string
	for _, e := range m.Entries {
		critical := false
		// FR/NFR rows tied to a critical test inherit critical-ness through coverage.
		if (e.Kind == matrix.KindFR || e.Kind == matrix.KindNFR) && e.CoverageStatus == matrix.CoverageDocumentationReview {
			critical = true
		}
		if (e.Kind == matrix.KindTC) && e.Severity == matrix.SeverityCritical && e.TestCaseStatus == matrix.TCExcludedDocumentation {
			critical = true
		}
		if !critical {
			continue
		}
		if o, ok := ovr.Overrides[e.ID]; !ok || o.Decision != overrides.DecisionPass {
			needsOverride = append(needsOverride, e.ID)
		}
	}
	if len(needsOverride) > 0 {
		return Verdict{
			Decision:  "INCOMPLETE",
			ExitCode:  3,
			Rationale: "Documentation review required but not supplied via overrides: " + strings.Join(needsOverride, ", "),
		}
	}

	// 4. Important FAIL/ERROR or NOT_RUN -> CONDITIONAL_GO.
	var importantTrouble []string
	for _, e := range m.TestCases() {
		if e.Severity != matrix.SeverityImportant || e.TestCaseStatus != matrix.TCInScope {
			continue
		}
		r, ok := byID[e.ID]
		if !ok {
			importantTrouble = append(importantTrouble, e.ID+":missing")
			continue
		}
		switch r.Status {
		case StatusFail, StatusError, StatusNotRun:
			importantTrouble = append(importantTrouble, e.ID+":"+string(r.Status))
		}
	}
	if len(importantTrouble) > 0 {
		return Verdict{
			Decision:  "CONDITIONAL_GO",
			ExitCode:  2,
			Rationale: "Important tests fell short: " + strings.Join(importantTrouble, ", "),
		}
	}

	return Verdict{
		Decision:  "GO",
		ExitCode:  0,
		Rationale: "All Critical pass; all Important pass or skipped with reason.",
	}
}
