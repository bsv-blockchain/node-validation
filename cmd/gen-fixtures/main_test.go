package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGeneratePC2Fixtures_minimum30(t *testing.T) {
	fxs := generatePC2Fixtures()
	if len(fxs) < 30 {
		t.Errorf("PC-2 fixtures: %d, want ≥30", len(fxs))
	}
}

func TestGeneratePC2Fixtures_categoriesCovered(t *testing.T) {
	fxs := generatePC2Fixtures()
	cats := map[string]int{}
	for _, f := range fxs {
		cats[f.Category]++
	}
	for _, want := range []string{"complex-p2sh", "restricted-opcodes", "cleanstack", "minimaldata", "malleability"} {
		if cats[want] < 6 {
			t.Errorf("category %q: %d fixtures, want ≥6", want, cats[want])
		}
	}
}

func TestGenerateIBD2Fixtures_minimum10(t *testing.T) {
	fxs := generateIBD2Fixtures()
	if len(fxs) < 10 {
		t.Errorf("IBD-2 fixtures: %d, want ≥10", len(fxs))
	}
}

func TestGenerateIBD2Fixtures_noEmptyIDs(t *testing.T) {
	fxs := generateIBD2Fixtures()
	for i, f := range fxs {
		if f.ID == "" {
			t.Errorf("IBD-2 fixture[%d] has empty ID", i)
		}
		if f.HexTx == "" {
			t.Errorf("IBD-2 fixture[%d] (%s) has empty HexTx", i, f.ID)
		}
		if f.Category == "" {
			t.Errorf("IBD-2 fixture[%d] (%s) has empty Category", i, f.ID)
		}
	}
}

func TestGeneratePC2Fixtures_noEmptyIDs(t *testing.T) {
	fxs := generatePC2Fixtures()
	seen := map[string]bool{}
	for i, f := range fxs {
		if f.ID == "" {
			t.Errorf("PC-2 fixture[%d] has empty ID", i)
		}
		if f.HexTx == "" {
			t.Errorf("PC-2 fixture[%d] (%s) has empty HexTx", i, f.ID)
		}
		if seen[f.ID] {
			t.Errorf("PC-2 fixture[%d] has duplicate ID: %s", i, f.ID)
		}
		seen[f.ID] = true
	}
}

func TestGenerator_isDeterministic(t *testing.T) {
	d := t.TempDir()
	pc2a := generatePC2Fixtures()
	pc2b := generatePC2Fixtures()
	if len(pc2a) != len(pc2b) {
		t.Fatal("PC-2 length differs between runs")
	}
	for i := range pc2a {
		if pc2a[i] != pc2b[i] {
			t.Errorf("PC-2 fixture %d differs: %+v vs %+v", i, pc2a[i], pc2b[i])
		}
	}

	if err := writeYAML(filepath.Join(d, "a.yaml"), pc2a); err != nil {
		t.Fatal(err)
	}
	if err := writeYAML(filepath.Join(d, "b.yaml"), pc2b); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(d, "a.yaml"))
	b, _ := os.ReadFile(filepath.Join(d, "b.yaml"))
	if string(a) != string(b) {
		t.Error("YAML output non-deterministic")
	}

	// Round-trip parse.
	var roundTrip []fixture
	if err := yaml.Unmarshal(a, &roundTrip); err != nil {
		t.Errorf("YAML round-trip parse failed: %v", err)
	}
	if len(roundTrip) != len(pc2a) {
		t.Errorf("round-trip: got %d fixtures, want %d", len(roundTrip), len(pc2a))
	}
}

func TestGenerator_ibd2Deterministic(t *testing.T) {
	d := t.TempDir()
	ibd2a := generateIBD2Fixtures()
	ibd2b := generateIBD2Fixtures()
	if len(ibd2a) != len(ibd2b) {
		t.Fatal("IBD-2 length differs between runs")
	}
	for i := range ibd2a {
		if ibd2a[i] != ibd2b[i] {
			t.Errorf("IBD-2 fixture %d differs: %+v vs %+v", i, ibd2a[i], ibd2b[i])
		}
	}

	if err := writeYAML(filepath.Join(d, "ibd2a.yaml"), ibd2a); err != nil {
		t.Fatal(err)
	}
	if err := writeYAML(filepath.Join(d, "ibd2b.yaml"), ibd2b); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(d, "ibd2a.yaml"))
	b, _ := os.ReadFile(filepath.Join(d, "ibd2b.yaml"))
	if string(a) != string(b) {
		t.Error("IBD-2 YAML output non-deterministic")
	}
}
