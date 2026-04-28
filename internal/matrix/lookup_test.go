// internal/matrix/lookup_test.go
package matrix

import "testing"

func TestByID(t *testing.T) {
	m := Load()
	if e, ok := m.ByID("PC-1"); !ok || e.Title != "Parallel Node Comparison" {
		t.Errorf("ByID(PC-1) failed: ok=%v entry=%+v", ok, e)
	}
	if _, ok := m.ByID("DOES-NOT-EXIST"); ok {
		t.Errorf("ByID of unknown should not be ok")
	}
}

func TestByKind(t *testing.T) {
	m := Load()
	cases := []struct {
		k    Kind
		want int
	}{{KindFR, 11}, {KindNFR, 13}, {KindTE, 3}, {KindTC, 16}, {KindNEW, 8}, {KindR, 7}}
	for _, c := range cases {
		if got := len(m.ByKind(c.k)); got != c.want {
			t.Errorf("ByKind(%s): want %d, got %d", c.k, c.want, got)
		}
	}
}

func TestRequirements(t *testing.T) {
	if got := len(Load().Requirements()); got != 24 {
		t.Errorf("Requirements: want 24, got %d", got)
	}
}

func TestTestCases(t *testing.T) {
	if got := len(Load().TestCases()); got != 24 {
		t.Errorf("TestCases: want 24, got %d", got)
	}
}

func TestInScopeTestIDs(t *testing.T) {
	ids := Load().InScopeTestIDs()
	// 11 in-scope source TCs + 8 NEW = 19.
	if len(ids) != 19 {
		t.Errorf("InScopeTestIDs: want 19, got %d (%v)", len(ids), ids)
	}
	for _, want := range []string{"PC-1", "PC-2", "PC-3", "IBD-2", "PERF-1", "INTER-1", "INTER-2",
		"CLIENT-1", "CLIENT-2", "CLIENT-3", "OPS-3",
		"NEW-FR7", "NEW-FR8", "NEW-FR9", "NEW-FR10", "NEW-FR11",
		"NEW-NFR7", "NEW-NFR11", "NEW-NFR13"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("InScopeTestIDs missing %s", want)
		}
	}
}

func TestRisks(t *testing.T) {
	if got := len(Load().Risks()); got != 7 {
		t.Errorf("Risks: want 7, got %d", got)
	}
}
