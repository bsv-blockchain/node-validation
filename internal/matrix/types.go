// Package matrix encodes the §3 traceability matrix from the SP1 spec
// as Go data, the single source of truth used by the runner, the
// reporter, and the codegen tools.
package matrix

// Kind is the structural category of a manifest entry.
type Kind string

const (
	KindFR  Kind = "FR"  // Functional Requirement (11)
	KindNFR Kind = "NFR" // Non-Functional Requirement (13)
	KindTE  Kind = "TE"  // Test Environment item (3)
	KindTC  Kind = "TC"  // Source-plan test case (16)
	KindNEW Kind = "NEW" // Test added by this project (8)
	KindR   Kind = "R"   // Risk (7)
)

// CoverageStatus describes how an FR or NFR is verified.
type CoverageStatus string

const (
	CoverageAutomated           CoverageStatus = "AUTOMATED"
	CoverageDocumentationReview CoverageStatus = "DOCUMENTATION_REVIEW"
	CoverageContractual         CoverageStatus = "CONTRACTUAL"
	CoverageLongTermObservation CoverageStatus = "LONG_TERM_OBSERVATION"
	CoveragePrivilegedAccess    CoverageStatus = "PRIVILEGED_ACCESS_REQUIRED"
	CoveragePartial             CoverageStatus = "PARTIAL"
	// CoverageExternal marks a requirement that is satisfied by a separate
	// solution outside this harness (e.g. Arcade / Arc), so it is neither
	// automated here nor pending a reviewer override.
	CoverageExternal CoverageStatus = "COVERED_EXTERNALLY"
)

// TestCaseStatus describes a TC or NEW row's relationship to the suite.
type TestCaseStatus string

const (
	TCInScope               TestCaseStatus = "IN_SCOPE"
	TCExcludedSetup         TestCaseStatus = "EXCLUDED_SETUP"
	TCExcludedDocumentation TestCaseStatus = "EXCLUDED_DOCUMENTATION"
	TCExcludedPrivileged    TestCaseStatus = "EXCLUDED_PRIVILEGED"
	// TCResolvedExternal marks a test case whose requirement is covered by a
	// separate solution (e.g. Arcade / Arc). The case is retired from the
	// suite: it is not registered or executed, and ExclusionReason records
	// the external attribution.
	TCResolvedExternal TestCaseStatus = "RESOLVED_EXTERNAL"
)

// Severity assigns a TC or NEW row to a verdict tier per source plan §"Critical/Important".
type Severity string

const (
	SeverityCritical  Severity = "critical"
	SeverityImportant Severity = "important"
	SeverityAdvisory  Severity = "advisory"
)

// Entry is one row of the traceability matrix. Some fields are
// populated only for some Kinds; see Manifest.Validate for the rules.
type Entry struct {
	ID              string
	Kind            Kind
	Title           string
	CoverageStatus  CoverageStatus
	TestCaseStatus  TestCaseStatus
	Severity        Severity
	CoveredBy       []string
	SatisfiesReqs   []string
	ExclusionReason string
	Notes           string
	PartialNote     string
}
