// internal/testrunner/report.go
package testrunner

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

// RunHeader is the run-level metadata block in the report.
type RunHeader struct {
	StartedAt         time.Time       `json:"started_at"`
	FinishedAt        time.Time       `json:"finished_at"`
	Network           string          `json:"network"`
	ShortMode         bool            `json:"short_mode"`
	ToolVersion       string          `json:"tool_version"`
	TeranodeVersion   string          `json:"teranode_version,omitempty"`
	SVNodeVersion     string          `json:"svnode_version,omitempty"`
	ReviewerOverrides *overrides.File `json:"reviewer_overrides,omitempty"`
}

// RequirementRow is one row in the FR/NFR table.
type RequirementRow struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"` // "FR" or "NFR"
	Title        string            `json:"title"`
	MatrixStatus string            `json:"matrix_status"`
	CoveredBy    []string          `json:"covered_by"`
	ResultStatus string            `json:"result_status"`
	Evidence     map[string]string `json:"evidence,omitempty"`
	Note         string            `json:"note,omitempty"`
	PartialNote  string            `json:"partial_note,omitempty"`
}

// TestEnvironmentRow is one row in the TE table.
type TestEnvironmentRow struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	MatrixStatus string `json:"matrix_status"`
	Note         string `json:"note,omitempty"`
}

// TestCaseRow is one row in the TC/NEW table.
type TestCaseRow struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Severity        string  `json:"severity"`
	MatrixStatus    string  `json:"matrix_status"`
	Result          *Result `json:"result,omitempty"`
	ExclusionReason string  `json:"exclusion_reason,omitempty"`
}

// RiskRow is one row in the R table.
type RiskRow struct {
	ID               string            `json:"id"`
	Description      string            `json:"description"`
	MitigationStatus string            `json:"mitigation_status"`
	MitigatingTests  []string          `json:"mitigating_tests"`
	EvidenceStatus   map[string]string `json:"evidence_status,omitempty"`
}

// SummaryCounts reproduces the build-doc §9.1 summary block.
type SummaryCounts struct {
	RequirementsTotal       int `json:"requirements_total"`
	RequirementsSatisfied   int `json:"requirements_satisfied"`
	RequirementsDeferred    int `json:"requirements_deferred"`
	RequirementsFailed      int `json:"requirements_failed"`
	TestCasesInSourcePlan   int `json:"test_cases_total_in_source_plan"`
	TestCasesInScope        int `json:"test_cases_in_scope"`
	NewTestsAdded           int `json:"new_tests_added"`
	NewTestsPassed          int `json:"new_tests_passed"`
	CriticalPass            int `json:"critical_pass"`
	CriticalFail            int `json:"critical_fail"`
	ImportantPass           int `json:"important_pass"`
	ImportantFail           int `json:"important_fail"`
	AdvisoryPass            int `json:"advisory_pass"`
	AdvisoryFail            int `json:"advisory_fail"`
	RisksMitigated          int `json:"risks_mitigated"`
	RisksPartiallyMitigated int `json:"risks_partially_mitigated"`
	RisksNotMitigated       int `json:"risks_not_mitigated"`
}

// ReportModel is the in-memory representation that all three reporters
// emit from. It is the result of BuildReportModel — never constructed
// directly.
type ReportModel struct {
	Run             RunHeader            `json:"run"`
	Verdict         Verdict              `json:"verdict"`
	Requirements    []RequirementRow     `json:"requirements"`
	TestEnvironment []TestEnvironmentRow `json:"test_environment"`
	TestCases       []TestCaseRow        `json:"test_cases"`
	Risks           []RiskRow            `json:"risks"`
	Summary         SummaryCounts        `json:"summary"`
}

// BuildReportModel walks the manifest and produces a ReportModel that
// honours the completeness invariant: 24 + 3 + 24 + 7 = 58 rows total.
// Returns an error if the manifest itself fails the invariant or if
// per-kind counts are off.
func BuildReportModel(env *Env, results []Result, ovr overrides.File, started, finished time.Time, version string) (ReportModel, error) {
	resultByID := make(map[string]Result, len(results))
	for _, r := range results {
		resultByID[r.ID] = r
	}

	model := ReportModel{
		Run: RunHeader{
			StartedAt:   started,
			FinishedAt:  finished,
			Network:     string(env.Cfg.Network),
			ShortMode:   env.Cfg.Short,
			ToolVersion: version,
		},
	}
	if len(ovr.Overrides) > 0 {
		o := ovr
		model.Run.ReviewerOverrides = &o
	}

	for _, e := range env.Manifest.Entries {
		switch e.Kind {
		case matrix.KindFR, matrix.KindNFR:
			model.Requirements = append(model.Requirements, requirementRow(e, resultByID, ovr))
		case matrix.KindTE:
			model.TestEnvironment = append(model.TestEnvironment, TestEnvironmentRow{
				ID: e.ID, Title: e.Title, MatrixStatus: "EXCLUDED_SETUP", Note: e.Notes,
			})
		case matrix.KindTC, matrix.KindNEW:
			model.TestCases = append(model.TestCases, testCaseRow(e, resultByID))
		case matrix.KindR:
			model.Risks = append(model.Risks, riskRow(e, resultByID, ovr))
		default:
			return ReportModel{}, fmt.Errorf("unknown kind %s for %s", e.Kind, e.ID)
		}
	}

	if got := len(model.Requirements) + len(model.TestEnvironment) + len(model.TestCases) + len(model.Risks); got != 58 {
		return ReportModel{}, fmt.Errorf("completeness invariant: want 58 rows, got %d (req=%d te=%d tc=%d r=%d)",
			got, len(model.Requirements), len(model.TestEnvironment), len(model.TestCases), len(model.Risks))
	}
	if len(model.Requirements) != 24 {
		return ReportModel{}, fmt.Errorf("completeness invariant: want 24 requirement rows, got %d", len(model.Requirements))
	}
	if len(model.TestEnvironment) != 3 {
		return ReportModel{}, fmt.Errorf("completeness invariant: want 3 test_environment rows, got %d", len(model.TestEnvironment))
	}
	if len(model.TestCases) != 24 {
		return ReportModel{}, fmt.Errorf("completeness invariant: want 24 test_case rows, got %d", len(model.TestCases))
	}
	if len(model.Risks) != 7 {
		return ReportModel{}, fmt.Errorf("completeness invariant: want 7 risk rows, got %d", len(model.Risks))
	}

	model.Verdict = ComputeVerdict(results, env.Manifest, ovr)
	model.Summary = computeSummary(model)
	return model, nil
}

func requirementRow(e matrix.Entry, results map[string]Result, ovr overrides.File) RequirementRow {
	row := RequirementRow{
		ID:           e.ID,
		Type:         string(e.Kind),
		Title:        e.Title,
		MatrixStatus: string(e.CoverageStatus),
		CoveredBy:    append([]string(nil), e.CoveredBy...),
		Note:         e.Notes,
		PartialNote:  e.PartialNote,
	}
	switch e.CoverageStatus {
	case matrix.CoverageContractual, matrix.CoverageLongTermObservation, matrix.CoverageDocumentationReview, matrix.CoveragePrivilegedAccess:
		if o, ok := ovr.Overrides[e.ID]; ok {
			row.ResultStatus = string(o.Decision)
		} else {
			row.ResultStatus = "DEFERRED"
		}
		return row
	}

	row.Evidence = map[string]string{}
	allPass := len(e.CoveredBy) > 0
	for _, id := range e.CoveredBy {
		r, ok := results[id]
		if !ok {
			row.Evidence[id] = "NOT_RUN"
			allPass = false
			continue
		}
		row.Evidence[id] = string(r.Status)
		if r.Status != StatusPass {
			allPass = false
		}
	}
	if allPass {
		row.ResultStatus = "PASS"
	} else {
		row.ResultStatus = "PENDING"
	}
	return row
}

func testCaseRow(e matrix.Entry, results map[string]Result) TestCaseRow {
	row := TestCaseRow{
		ID:              e.ID,
		Title:           e.Title,
		Severity:        string(e.Severity),
		MatrixStatus:    string(e.TestCaseStatus),
		ExclusionReason: e.ExclusionReason,
	}
	if e.TestCaseStatus == matrix.TCInScope {
		if r, ok := results[e.ID]; ok {
			rcopy := r
			row.Result = &rcopy
		} else {
			rcopy := Result{
				ID:         e.ID,
				Title:      e.Title,
				Severity:   e.Severity,
				Status:     StatusNotRun,
				SkipReason: "not registered",
			}
			row.Result = &rcopy
		}
	}
	return row
}

func riskRow(e matrix.Entry, results map[string]Result, ovr overrides.File) RiskRow {
	row := RiskRow{
		ID:              e.ID,
		Description:     e.Title,
		MitigatingTests: append([]string(nil), e.CoveredBy...),
		EvidenceStatus:  map[string]string{},
	}
	if len(e.CoveredBy) == 0 {
		row.MitigationStatus = "NOT_MITIGATED"
		return row
	}
	pass, fail, deferred := 0, 0, 0
	for _, id := range e.CoveredBy {
		if r, ok := results[id]; ok {
			row.EvidenceStatus[id] = string(r.Status)
			switch r.Status {
			case StatusPass:
				pass++
			case StatusFail, StatusError:
				fail++
			default:
				deferred++
			}
		} else {
			// Not in results — could be a doc-review row covered by overrides.
			if o, ok := ovr.Overrides[id]; ok && o.Decision == overrides.DecisionPass {
				row.EvidenceStatus[id] = "OVERRIDE_PASS"
				pass++
			} else {
				row.EvidenceStatus[id] = "DEFERRED"
				deferred++
			}
		}
	}
	switch {
	case fail == len(e.CoveredBy):
		row.MitigationStatus = "NOT_MITIGATED"
	case pass == len(e.CoveredBy):
		row.MitigationStatus = "MITIGATED"
	default:
		row.MitigationStatus = "PARTIALLY_MITIGATED"
	}
	return row
}

func computeSummary(m ReportModel) SummaryCounts {
	s := SummaryCounts{
		RequirementsTotal:     len(m.Requirements),
		TestCasesInSourcePlan: 16,
		NewTestsAdded:         8,
	}
	for _, r := range m.Requirements {
		switch r.ResultStatus {
		case "PASS":
			s.RequirementsSatisfied++
		case "DEFERRED":
			s.RequirementsDeferred++
		case "FAIL":
			s.RequirementsFailed++
		}
	}
	for _, tc := range m.TestCases {
		if tc.MatrixStatus == "IN_SCOPE" {
			s.TestCasesInScope++
		}
		if tc.Result == nil {
			continue
		}
		switch tc.Severity {
		case "critical":
			if tc.Result.Status == StatusPass {
				s.CriticalPass++
			} else if tc.Result.Status == StatusFail || tc.Result.Status == StatusError {
				s.CriticalFail++
			}
		case "important":
			if tc.Result.Status == StatusPass {
				s.ImportantPass++
			} else if tc.Result.Status == StatusFail || tc.Result.Status == StatusError {
				s.ImportantFail++
			}
		case "advisory":
			if tc.Result.Status == StatusPass {
				s.AdvisoryPass++
				s.NewTestsPassed++
			} else if tc.Result.Status == StatusFail || tc.Result.Status == StatusError {
				s.AdvisoryFail++
			}
		}
	}
	for _, r := range m.Risks {
		switch r.MitigationStatus {
		case "MITIGATED":
			s.RisksMitigated++
		case "PARTIALLY_MITIGATED":
			s.RisksPartiallyMitigated++
		case "NOT_MITIGATED":
			s.RisksNotMitigated++
		}
	}
	return s
}
