// internal/matrix/validate.go
package matrix

import (
	"fmt"
	"regexp"
	"strings"
)

// Load returns the canonical 58-row manifest. It panics if the
// embedded data fails Manifest.Validate, since that is a programmer
// error caught by package-level tests.
func Load() Manifest {
	m := manifest()
	if err := m.Validate(); err != nil {
		panic("matrix: invalid embedded manifest: " + err.Error())
	}
	return m
}

// Validate enforces the structural invariants of the manifest. It returns
// a single error whose message contains all problems found, separated by
// "; ". The expected counts (11 FR, 13 NFR, 3 TE, 16 TC, 8 NEW, 7 R, total 58)
// are part of the contract.
func (m Manifest) Validate() error {
	var errs []string

	// 1. Total count.
	if got := len(m.Entries); got != 58 {
		errs = append(errs, fmt.Sprintf("expected 58 entries, got %d", got))
	}

	// 2. Per-kind counts.
	wantCounts := map[Kind]int{
		KindFR: 11, KindNFR: 13, KindTE: 3, KindTC: 16, KindNEW: 8, KindR: 7,
	}
	gotCounts := map[Kind]int{}
	ids := map[string]int{}
	for _, e := range m.Entries {
		gotCounts[e.Kind]++
		ids[e.ID]++
	}
	for k, want := range wantCounts {
		if gotCounts[k] != want {
			errs = append(errs, fmt.Sprintf("kind %s: want %d entries, got %d", k, want, gotCounts[k]))
		}
	}

	// 3. Duplicate IDs.
	for id, n := range ids {
		if n > 1 {
			errs = append(errs, fmt.Sprintf("duplicate ID %q (%d times)", id, n))
		}
	}

	// 4. ID format per kind.
	idPatterns := map[Kind]*regexp.Regexp{
		KindFR:  regexp.MustCompile(`^FR-([1-9]|10|11)$`),
		KindNFR: regexp.MustCompile(`^NFR-([1-9]|10|11|12|13)$`),
		KindTE:  regexp.MustCompile(`^TE-[1-3]$`),
		KindR:   regexp.MustCompile(`^R[1-7]$`),
	}
	knownTC := map[string]bool{
		"PC-1": true, "PC-2": true, "PC-3": true,
		"IBD-1": true, "IBD-2": true,
		"PERF-1": true, "PERF-2": true, "PERF-3": true,
		"INTER-1": true, "INTER-2": true,
		"CLIENT-1": true, "CLIENT-2": true, "CLIENT-3": true,
		"OPS-1": true, "OPS-2": true, "OPS-3": true,
	}
	knownNEW := map[string]bool{
		"NEW-FR7": true, "NEW-FR8": true, "NEW-FR9": true,
		"NEW-FR10": true, "NEW-FR11": true,
		"NEW-NFR7": true, "NEW-NFR11": true, "NEW-NFR13": true,
	}
	for _, e := range m.Entries {
		switch e.Kind {
		case KindTC:
			if !knownTC[e.ID] {
				errs = append(errs, fmt.Sprintf("unknown TC ID %q", e.ID))
			}
		case KindNEW:
			if !knownNEW[e.ID] {
				errs = append(errs, fmt.Sprintf("unknown NEW ID %q", e.ID))
			}
		default:
			pat := idPatterns[e.Kind]
			if pat != nil && !pat.MatchString(e.ID) {
				errs = append(errs, fmt.Sprintf("bad ID format for kind %s: %q", e.Kind, e.ID))
			}
		}
	}

	// 5. Cross-references resolve.
	known := map[string]bool{}
	for _, e := range m.Entries {
		known[e.ID] = true
	}
	for _, e := range m.Entries {
		for _, ref := range e.CoveredBy {
			if !known[ref] {
				errs = append(errs, fmt.Sprintf("%s.CoveredBy references unknown ID %q", e.ID, ref))
			}
		}
		for _, ref := range e.SatisfiesReqs {
			if !known[ref] {
				errs = append(errs, fmt.Sprintf("%s.SatisfiesReqs references unknown ID %q", e.ID, ref))
			}
		}
	}

	// 6, 7, 8, 9. Field presence by kind.
	for _, e := range m.Entries {
		isTC := e.Kind == KindTC || e.Kind == KindNEW
		isReq := e.Kind == KindFR || e.Kind == KindNFR

		if isTC && e.Severity == "" {
			errs = append(errs, fmt.Sprintf("%s: severity required for TC/NEW", e.ID))
		}
		if !isTC && e.Severity != "" {
			errs = append(errs, fmt.Sprintf("%s: severity must be empty for kind %s", e.ID, e.Kind))
		}
		if isReq && e.CoverageStatus == "" {
			errs = append(errs, fmt.Sprintf("%s: coverage status required for FR/NFR", e.ID))
		}
		if !isReq && e.CoverageStatus != "" {
			errs = append(errs, fmt.Sprintf("%s: coverage status must be empty for kind %s", e.ID, e.Kind))
		}
		if isTC && e.TestCaseStatus == "" {
			errs = append(errs, fmt.Sprintf("%s: test-case status required for TC/NEW", e.ID))
		}
		if !isTC && e.TestCaseStatus != "" {
			errs = append(errs, fmt.Sprintf("%s: test-case status must be empty for kind %s", e.ID, e.Kind))
		}
		switch e.TestCaseStatus {
		case TCExcludedSetup, TCExcludedDocumentation, TCExcludedPrivileged, TCResolvedExternal:
			if e.ExclusionReason == "" {
				errs = append(errs, fmt.Sprintf("%s: exclusion reason required for status %s", e.ID, e.TestCaseStatus))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("manifest invalid: %s", strings.Join(errs, "; "))
	}
	return nil
}
