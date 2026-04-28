// internal/matrix/lookup.go
package matrix

// ByID returns the entry with the given ID.
func (m Manifest) ByID(id string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// ByKind returns all entries of the given Kind, preserving order.
func (m Manifest) ByKind(k Kind) []Entry {
	var out []Entry
	for _, e := range m.Entries {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

// Requirements returns FR + NFR entries.
func (m Manifest) Requirements() []Entry {
	out := make([]Entry, 0, 24)
	for _, e := range m.Entries {
		if e.Kind == KindFR || e.Kind == KindNFR {
			out = append(out, e)
		}
	}
	return out
}

// TestCases returns TC + NEW entries.
func (m Manifest) TestCases() []Entry {
	out := make([]Entry, 0, 24)
	for _, e := range m.Entries {
		if e.Kind == KindTC || e.Kind == KindNEW {
			out = append(out, e)
		}
	}
	return out
}

// InScopeTestIDs returns IDs of TC entries with status IN_SCOPE plus all NEW entries.
func (m Manifest) InScopeTestIDs() []string {
	var out []string
	for _, e := range m.Entries {
		if (e.Kind == KindTC && e.TestCaseStatus == TCInScope) || e.Kind == KindNEW {
			out = append(out, e.ID)
		}
	}
	return out
}

// Risks returns R entries.
func (m Manifest) Risks() []Entry {
	return m.ByKind(KindR)
}
