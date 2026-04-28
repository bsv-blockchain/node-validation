package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRefs_validReference(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(src, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("see `fake.go:2`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMarkdownRefs(mdPath, dir); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestCheckRefs_outOfBoundsLine(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(src, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(mdPath, []byte("see `fake.go:99`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMarkdownRefs(mdPath, dir); err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestCheckYAML_minimumSurfaceCount(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "x.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
upstream_commit: "abcdef1"
discovered_at: "2026-04-29T00:00:00Z"
surfaces: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkYAML(yamlPath); err == nil {
		t.Error("expected 'surfaces must have 11' error")
	}
}
