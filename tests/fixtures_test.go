package tests

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFixtures_validFile(t *testing.T) {
	d := t.TempDir()
	yamlData := []byte(`
- id: f1
  category: cat-a
  description: "A fixture"
  hex_tx: "01000000"
  expected_valid: false
  expected_category: "MALFORMED"
  provenance: "test"
`)
	p := filepath.Join(d, "x.yaml")
	if err := os.WriteFile(p, yamlData, 0o644); err != nil {
		t.Fatal(err)
	}
	fxs, err := LoadFixtures(p)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fxs) != 1 || fxs[0].ID != "f1" {
		t.Errorf("got %+v", fxs)
	}
}

func TestLoadFixtures_missingFile(t *testing.T) {
	if _, err := LoadFixtures("/tmp/no-such-fixture-zzz.yaml"); err == nil {
		t.Error("want error")
	}
}

func TestLoadFixtures_allFields(t *testing.T) {
	d := t.TempDir()
	yamlData := []byte(`
- id: fx-001
  category: complex-p2sh
  description: "Test fixture"
  hex_tx: "deadbeef"
  expected_valid: false
  expected_category: "UTXO_MISSING"
  provenance: "synthetic"
  notes: "some note"
`)
	p := filepath.Join(d, "full.yaml")
	if err := os.WriteFile(p, yamlData, 0o644); err != nil {
		t.Fatal(err)
	}
	fxs, err := LoadFixtures(p)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fxs) != 1 {
		t.Fatalf("got %d fixtures, want 1", len(fxs))
	}
	f := fxs[0]
	if f.ID != "fx-001" {
		t.Errorf("ID: got %q, want %q", f.ID, "fx-001")
	}
	if f.Category != "complex-p2sh" {
		t.Errorf("Category: got %q", f.Category)
	}
	if f.ExpectedCategory != "UTXO_MISSING" {
		t.Errorf("ExpectedCategory: got %q", f.ExpectedCategory)
	}
	if f.Notes != "some note" {
		t.Errorf("Notes: got %q", f.Notes)
	}
}

func TestLoadFixtures_multipleEntries(t *testing.T) {
	d := t.TempDir()
	yamlData := []byte(`
- id: a
  category: complex-p2sh
  description: "A"
  hex_tx: "01"
  expected_valid: false
  expected_category: "UTXO_MISSING"
  provenance: "test"
- id: b
  category: restricted-opcodes
  description: "B"
  hex_tx: "02"
  expected_valid: false
  expected_category: "MALFORMED"
  provenance: "test"
`)
	p := filepath.Join(d, "multi.yaml")
	if err := os.WriteFile(p, yamlData, 0o644); err != nil {
		t.Fatal(err)
	}
	fxs, err := LoadFixtures(p)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fxs) != 2 {
		t.Fatalf("got %d fixtures, want 2", len(fxs))
	}
	if fxs[0].ID != "a" || fxs[1].ID != "b" {
		t.Errorf("IDs: %v %v", fxs[0].ID, fxs[1].ID)
	}
}
