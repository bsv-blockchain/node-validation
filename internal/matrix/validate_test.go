// internal/matrix/validate_test.go
package matrix

import (
	"strings"
	"testing"
)

func TestValidate_canonicalManifestPasses(t *testing.T) {
	if err := manifest().Validate(); err != nil {
		t.Fatalf("canonical manifest must validate, got: %v", err)
	}
}

func TestValidate_totalCount(t *testing.T) {
	m := manifest()
	if got := len(m.Entries); got != 58 {
		t.Fatalf("expected 58 entries, got %d", got)
	}
}

func TestValidate_perKindCounts(t *testing.T) {
	m := manifest()
	want := map[Kind]int{
		KindFR: 11, KindNFR: 13, KindTE: 3, KindTC: 16, KindNEW: 8, KindR: 7,
	}
	got := map[Kind]int{}
	for _, e := range m.Entries {
		got[e.Kind]++
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("kind %s: want %d, got %d", k, n, got[k])
		}
	}
}

func TestValidate_duplicateID(t *testing.T) {
	m := manifest()
	m.Entries = append(m.Entries, m.Entries[0])
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate ID") {
		t.Fatalf("expected duplicate ID error, got %v", err)
	}
}

func TestValidate_unresolvedCoveredBy(t *testing.T) {
	m := manifest()
	m.Entries[0].CoveredBy = []string{"DOES-NOT-EXIST"}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "DOES-NOT-EXIST") {
		t.Fatalf("expected unresolved CoveredBy error, got %v", err)
	}
}

func TestValidate_unresolvedSatisfiesReqs(t *testing.T) {
	m := manifest()
	for i := range m.Entries {
		if m.Entries[i].ID == "PC-1" {
			m.Entries[i].SatisfiesReqs = []string{"FR-NOT-REAL"}
		}
	}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "FR-NOT-REAL") {
		t.Fatalf("expected unresolved SatisfiesReqs error, got %v", err)
	}
}

func TestValidate_severitySetOnTC(t *testing.T) {
	m := manifest()
	for i := range m.Entries {
		if m.Entries[i].ID == "PC-1" {
			m.Entries[i].Severity = ""
		}
	}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "severity required") {
		t.Fatalf("expected severity-required error, got %v", err)
	}
}

func TestValidate_coverageStatusSetOnFR(t *testing.T) {
	m := manifest()
	for i := range m.Entries {
		if m.Entries[i].ID == "FR-1" {
			m.Entries[i].CoverageStatus = ""
		}
	}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "coverage status required") {
		t.Fatalf("expected coverage-status-required error, got %v", err)
	}
}

func TestValidate_exclusionReasonSetOnExcluded(t *testing.T) {
	m := manifest()
	for i := range m.Entries {
		if m.Entries[i].ID == "PERF-2" {
			m.Entries[i].ExclusionReason = ""
		}
	}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "exclusion reason required") {
		t.Fatalf("expected exclusion-reason-required error, got %v", err)
	}
}
