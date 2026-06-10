package matrix

// Manifest holds the ordered traceability matrix.
type Manifest struct {
	Entries []Entry
}

func manifest() Manifest {
	return Manifest{Entries: []Entry{
		// ---- Functional Requirements (FR-1 .. FR-11) ----
		{
			ID: "FR-1", Kind: KindFR,
			Title:          "Full Bitcoin SV Protocol Compliance",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"PC-1", "PC-2"},
		},
		{
			ID: "FR-2", Kind: KindFR,
			Title:          "Consistent Transaction and Block Formats",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"PC-3", "CLIENT-2"},
		},
		{
			ID: "FR-3", Kind: KindFR,
			Title:          "Script Interpreter Parity",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"PC-2", "IBD-2"},
		},
		{
			ID: "FR-4", Kind: KindFR,
			Title:          "Historical Chain Validation Evidence",
			CoverageStatus: CoverageDocumentationReview,
			CoveredBy:      []string{},
			Notes:          "IBD-1 in source plan; not an automated test. Reviewer override required.",
		},
		{
			ID: "FR-5", Kind: KindFR,
			Title:          "Reliable Transaction Propagation",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"INTER-2", "CLIENT-1"},
		},
		{
			ID: "FR-6", Kind: KindFR,
			Title:          "Block and Transaction Notification Reliability",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"CLIENT-1", "CLIENT-3"},
		},
		{
			ID: "FR-7", Kind: KindFR,
			Title:          "Support for Unconfirmed Transaction Chains",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-FR7"},
		},
		{
			ID: "FR-8", Kind: KindFR,
			Title:          "Transaction Fee Estimation",
			CoverageStatus: CoverageExternal,
			CoveredBy:      []string{},
			Notes:          "Covered by Arcade / Arc. Fee estimation is provided by the external Arc/Arcade solution, not the Teranode estimatefee RPC; not validated by this harness.",
		},
		{
			ID: "FR-9", Kind: KindFR,
			Title:          "Double-Spend Detection and Notification",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-FR9"},
		},
		{
			ID: "FR-10", Kind: KindFR,
			Title:          "Historical Data Access",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-FR10"},
		},
		{
			ID: "FR-11", Kind: KindFR,
			Title:          "Mempool Query and Filtering",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-FR11"},
		},

		// ---- Non-Functional Requirements (NFR-1 .. NFR-13) ----
		{
			ID: "NFR-1", Kind: KindNFR,
			Title:          "Upstream Availability Guarantees",
			CoverageStatus: CoverageLongTermObservation,
			CoveredBy:      []string{},
			Notes:          "30-day window required; runner records observed uptime as supporting metric only.",
		},
		{
			ID: "NFR-2", Kind: KindNFR,
			Title:          "Fault Tolerance and Recovery",
			CoverageStatus: CoveragePrivilegedAccess,
			CoveredBy:      []string{},
			Notes:          "OPS-1 excluded — requires admin access.",
		},
		{
			ID: "NFR-3", Kind: KindNFR,
			Title:          "Throughput and Latency Performance",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"PERF-1"},
		},
		{
			ID: "NFR-4", Kind: KindNFR,
			Title:          "Translation and Gateway Overhead",
			CoverageStatus: CoveragePrivilegedAccess,
			CoveredBy:      []string{},
			Notes:          "PERF-3 excluded — requires bypass of Teranode gateway.",
		},
		{
			ID: "NFR-5", Kind: KindNFR,
			Title:          "IPv4/IPv6 and Real-World Internet Compat.",
			CoverageStatus: CoveragePartial,
			CoveredBy:      []string{"INTER-1", "CLIENT-1"},
			PartialNote:    "AUTOMATED for IPv4 (runner self-hosts on IPv4). IPv6-only path not exercised.",
		},
		{
			ID: "NFR-6", Kind: KindNFR,
			Title:          "Interoperability Across Implementations",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"INTER-1", "INTER-2"},
		},
		{
			ID: "NFR-7", Kind: KindNFR,
			Title:          "Deterministic Behavior Under Load",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-NFR7"},
		},
		{
			ID: "NFR-8", Kind: KindNFR,
			Title:          "API Stability and Versioning",
			CoverageStatus: CoverageContractual,
			CoveredBy:      []string{},
			Notes:          "BSVA documentation review required.",
		},
		{
			ID: "NFR-9", Kind: KindNFR,
			Title:          "API Pricing and Access Model",
			CoverageStatus: CoverageContractual,
			CoveredBy:      []string{},
			Notes:          "BSVA pricing documentation review required.",
		},
		{
			ID: "NFR-10", Kind: KindNFR,
			Title:          "Observability and Diagnostics",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"OPS-3"},
		},
		{
			ID: "NFR-11", Kind: KindNFR,
			Title:          "Security and Authentication",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-NFR11"},
		},
		{
			ID: "NFR-12", Kind: KindNFR,
			Title:          "Data Consistency Guarantees",
			CoverageStatus: CoveragePartial,
			CoveredBy:      []string{"PC-1"},
			PartialNote:    "PC-1 covers cross-node consistency. OPS-2 excluded for reorg/partition scenarios.",
		},
		{
			ID: "NFR-13", Kind: KindNFR,
			Title:          "Rate Limiting and Throttling",
			CoverageStatus: CoverageAutomated,
			CoveredBy:      []string{"NEW-NFR13"},
		},

		// ---- Test Environment items (TE-1 .. TE-3) ----
		{
			ID: "TE-1", Kind: KindTE,
			Title: "Testnet Deployment",
			Notes: "EXCLUDED_SETUP — pre-existing infrastructure expected; runner only validates connectivity.",
		},
		{
			ID: "TE-2", Kind: KindTE,
			Title: "Client Integration Sandbox",
			Notes: "EXCLUDED_SETUP — this project IS the integration sandbox.",
		},
		{
			ID: "TE-3", Kind: KindTE,
			Title: "Private Test Network",
			Notes: "EXCLUDED_SETUP — optional setup, not used by any in-scope test.",
		},

		// ---- Source-plan test cases ----
		{
			ID: "PC-1", Kind: KindTC, Title: "Parallel Node Comparison",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-1", "NFR-12"},
		},
		{
			ID: "PC-2", Kind: KindTC, Title: "Historical Script and Consensus Regression",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-1", "FR-3"},
		},
		{
			ID: "PC-3", Kind: KindTC, Title: "Message Format and Wire Protocol Verification",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-2"},
			Notes:         "Format scope only; raw P2P capture out of scope.",
		},
		{
			ID: "IBD-1", Kind: KindTC, Title: "Historical Validation Evidence Review",
			TestCaseStatus:  TCExcludedDocumentation,
			Severity:        SeverityCritical,
			ExclusionReason: "Source plan itself notes this is documentation review, not testing.",
			SatisfiesReqs:   []string{"FR-4"},
		},
		{
			ID: "IBD-2", Kind: KindTC, Title: "Historical UTXO Spend Verification",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-3"},
		},
		{
			ID: "PERF-1", Kind: KindTC, Title: "Throughput and Latency Baseline",
			TestCaseStatus: TCInScope, Severity: SeverityImportant,
			SatisfiesReqs: []string{"NFR-3"},
		},
		{
			ID: "PERF-2", Kind: KindTC, Title: "Microservices Horizontal Scaling",
			TestCaseStatus:  TCExcludedPrivileged,
			Severity:        SeverityImportant,
			ExclusionReason: "Requires admin access to scale Teranode replicas.",
		},
		{
			ID: "PERF-3", Kind: KindTC, Title: "Gateway and Translation Overhead",
			TestCaseStatus:  TCExcludedPrivileged,
			Severity:        SeverityImportant,
			ExclusionReason: "Requires bypass of Teranode P2P gateway.",
		},
		{
			ID: "INTER-1", Kind: KindTC, Title: "Mixed-Network Consensus",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"NFR-6"},
		},
		{
			ID: "INTER-2", Kind: KindTC, Title: "Cross-Implementation Transaction Propagation",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-5", "NFR-6"},
		},
		{
			ID: "CLIENT-1", Kind: KindTC, Title: "TNG P2P Client Functional Tests",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-5", "FR-6"},
		},
		{
			ID: "CLIENT-2", Kind: KindTC, Title: "Extended Transaction Format Support",
			TestCaseStatus: TCInScope, Severity: SeverityImportant,
			SatisfiesReqs: []string{"FR-2"},
			Notes:         "Skips at runtime if no extended format advertised.",
		},
		{
			ID: "CLIENT-3", Kind: KindTC, Title: "Notification Stream Reliability",
			TestCaseStatus: TCInScope, Severity: SeverityCritical,
			SatisfiesReqs: []string{"FR-6"},
		},
		{
			ID: "OPS-1", Kind: KindTC, Title: "Service Failure and Recovery",
			TestCaseStatus:  TCExcludedPrivileged,
			Severity:        SeverityImportant,
			ExclusionReason: "Requires killing internal microservices.",
		},
		{
			ID: "OPS-2", Kind: KindTC, Title: "Network Partition and Reorg Convergence",
			TestCaseStatus:  TCExcludedPrivileged,
			Severity:        SeverityImportant,
			ExclusionReason: "Requires controlling the entire test network.",
		},
		{
			ID: "OPS-3", Kind: KindTC, Title: "Observability and Monitoring",
			TestCaseStatus: TCInScope, Severity: SeverityImportant,
			SatisfiesReqs: []string{"NFR-10"},
			Notes:         "Probe scope only; integration with TNG monitoring out of scope.",
		},

		// ---- New automated test cases (NEW-*) ----
		{
			ID: "NEW-FR7", Kind: KindNEW, Title: "Unconfirmed Transaction Chain Acceptance",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"FR-7"},
		},
		{
			ID: "NEW-FR8", Kind: KindNEW, Title: "Fee Estimation Endpoint Validation",
			TestCaseStatus: TCResolvedExternal, Severity: SeverityAdvisory,
			SatisfiesReqs:   []string{"FR-8"},
			ExclusionReason: "Covered by Arcade / Arc — fee estimation is provided by the external Arc/Arcade solution, not the Teranode estimatefee RPC (which returns ErrRPCUnimplemented). Retired from the suite; no longer registered or executed.",
		},
		{
			ID: "NEW-FR9", Kind: KindNEW, Title: "Double-Spend Detection Behaviour",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"FR-9"},
		},
		{
			ID: "NEW-FR10", Kind: KindNEW, Title: "Historical Data Access Latency",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"FR-10"},
		},
		{
			ID: "NEW-FR11", Kind: KindNEW, Title: "Mempool Query Capabilities",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"FR-11"},
		},
		{
			ID: "NEW-NFR7", Kind: KindNEW, Title: "Deterministic Behaviour Under Repeated Operations",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"NFR-7"},
		},
		{
			ID: "NEW-NFR11", Kind: KindNEW, Title: "Transport Security and Authentication Probe",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"NFR-11"},
		},
		{
			ID: "NEW-NFR13", Kind: KindNEW, Title: "Rate Limit Discovery and Error Semantics",
			TestCaseStatus: TCInScope, Severity: SeverityAdvisory,
			SatisfiesReqs: []string{"NFR-13"},
		},

		// ---- Risks (R1 .. R7) ----
		{
			ID: "R1", Kind: KindR,
			Title:     "Transaction loss / inconsistent mempools (changed propagation, overlays)",
			CoveredBy: []string{"INTER-2", "CLIENT-1", "CLIENT-3", "NEW-FR7"},
		},
		{
			ID: "R2", Kind: KindR,
			Title:     "Protocol fragmentation / forks (message formats, translation, interop gaps)",
			CoveredBy: []string{"PC-1", "PC-3", "INTER-1", "INTER-2", "CLIENT-2"},
		},
		{
			ID: "R3", Kind: KindR,
			Title:     "Undetected consensus bugs (script interpreter parity, historical flags)",
			CoveredBy: []string{"PC-1", "PC-2", "IBD-2"},
		},
		{
			ID: "R4", Kind: KindR,
			Title:     "Undetected consensus bugs (incomplete historical validation)",
			CoveredBy: []string{"IBD-1", "IBD-2"},
			Notes:     "IBD-1 documentation review required for full mitigation.",
		},
		{
			ID: "R5", Kind: KindR,
			Title:     "Excessive operational cost / API fees",
			CoveredBy: []string{"PERF-1"},
			Notes:     "Cost side is CONTRACTUAL via NFR-9.",
		},
		{
			ID: "R6", Kind: KindR,
			Title:     "Underdocumented architecture (txn trees, microservices, overlays)",
			CoveredBy: []string{"OPS-3", "CLIENT-1", "CLIENT-3", "NEW-FR11"},
			Notes:     "PERF-2 / OPS-1 needed for full mitigation.",
		},
		{
			ID: "R7", Kind: KindR,
			Title:     "Real-world IPv4/IPv6 connectivity and partition behaviour",
			CoveredBy: []string{"INTER-1", "CLIENT-1"},
			Notes:     "OPS-2 needed for partition behaviour.",
		},
	}}
}
