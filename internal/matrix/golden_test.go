// internal/matrix/golden_test.go
package matrix

import (
	_ "embed"
	"testing"

	"gopkg.in/yaml.v3"
)

//go:embed testdata/golden.yaml
var goldenYAML []byte

type goldenRow struct {
	ID    string `yaml:"id"`
	Kind  string `yaml:"kind"`
	Title string `yaml:"title"`
}

func TestGoldenSnapshot(t *testing.T) {
	var rows []goldenRow
	if err := yaml.Unmarshal(goldenYAML, &rows); err != nil {
		t.Fatalf("parsing golden.yaml: %v", err)
	}
	m := Load()
	if len(rows) != len(m.Entries) {
		t.Fatalf("golden has %d rows, manifest has %d entries", len(rows), len(m.Entries))
	}
	for i, want := range rows {
		got := m.Entries[i]
		if want.ID != got.ID || want.Kind != string(got.Kind) || want.Title != got.Title {
			t.Errorf("row %d:\n  golden:  {%s, %s, %q}\n  current: {%s, %s, %q}",
				i, want.ID, want.Kind, want.Title, got.ID, got.Kind, got.Title)
		}
	}
}
