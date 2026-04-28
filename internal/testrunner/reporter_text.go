// internal/testrunner/reporter_text.go
package testrunner

import (
	"fmt"
	"io"
	"strings"
)

// WriteText renders a ReportModel as a plain-text report.
func WriteText(w io.Writer, m ReportModel) error {
	out := &strings.Builder{}
	fmt.Fprintf(out, "==============================================\n")
	fmt.Fprintf(out, " Teranode Acceptance Tests\n")
	fmt.Fprintf(out, " Tool version: %s\n", m.Run.ToolVersion)
	fmt.Fprintf(out, " Network:      %s\n", m.Run.Network)
	fmt.Fprintf(out, " Started:      %s\n", m.Run.StartedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(out, " Finished:     %s\n", m.Run.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	if m.Run.ShortMode {
		fmt.Fprintf(out, " Mode:         SHORT (partial evidence)\n")
	}
	fmt.Fprintf(out, "==============================================\n\n")

	fmt.Fprintf(out, "Verdict: %s (exit code %d)\n", m.Verdict.Decision, m.Verdict.ExitCode)
	fmt.Fprintf(out, "  %s\n\n", m.Verdict.Rationale)

	fmt.Fprintf(out, "Summary\n")
	fmt.Fprintf(out, "-------\n")
	fmt.Fprintf(out, "Requirements:        %d (satisfied %d, deferred %d, failed %d)\n",
		m.Summary.RequirementsTotal, m.Summary.RequirementsSatisfied,
		m.Summary.RequirementsDeferred, m.Summary.RequirementsFailed)
	fmt.Fprintf(out, "Test cases:          %d (in scope %d; new %d, passed %d)\n",
		m.Summary.TestCasesInSourcePlan+m.Summary.NewTestsAdded,
		m.Summary.TestCasesInScope, m.Summary.NewTestsAdded, m.Summary.NewTestsPassed)
	fmt.Fprintf(out, "Critical:            pass=%d fail=%d\n", m.Summary.CriticalPass, m.Summary.CriticalFail)
	fmt.Fprintf(out, "Important:           pass=%d fail=%d\n", m.Summary.ImportantPass, m.Summary.ImportantFail)
	fmt.Fprintf(out, "Advisory:            pass=%d fail=%d\n", m.Summary.AdvisoryPass, m.Summary.AdvisoryFail)
	fmt.Fprintf(out, "Risks:               %d (mitigated %d, partial %d, not %d)\n\n",
		7, m.Summary.RisksMitigated, m.Summary.RisksPartiallyMitigated, m.Summary.RisksNotMitigated)

	fmt.Fprintf(out, "Test cases\n----------\n")
	for _, tc := range m.TestCases {
		status := tc.MatrixStatus
		if tc.Result != nil {
			status = string(tc.Result.Status)
		}
		fmt.Fprintf(out, "  %-10s %-10s %s — %s\n", tc.ID, tc.Severity, status, tc.Title)
		if tc.ExclusionReason != "" {
			fmt.Fprintf(out, "             reason: %s\n", tc.ExclusionReason)
		}
		if tc.Result != nil && tc.Result.SkipReason != "" {
			fmt.Fprintf(out, "             skip:   %s\n", tc.Result.SkipReason)
		}
	}
	fmt.Fprintf(out, "\nRisks\n-----\n")
	for _, r := range m.Risks {
		fmt.Fprintf(out, "  %-3s %-22s %s\n", r.ID, r.MitigationStatus, r.Description)
	}

	_, err := io.WriteString(w, out.String())
	return err
}
