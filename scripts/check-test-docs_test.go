package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// auditDir is the testable core of check-test-docs.go. It walks dir and
// returns a violation string for each file missing a required marker.
func auditDir(dir string) []string {
	skip := map[string]bool{
		"fixtures.go": true, "helper.go": true, "doc.go": true,
		"tests_test.go": true, "fixtures_test.go": true,
	}
	markers := []string{"Objective:", "Method:", "Acceptance criteria:"}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("read dir %s: %v", dir, err)}
	}
	var violations []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if skip[name] {
			continue
		}
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: read: %v", path, err))
			continue
		}
		lines := strings.Split(string(body), "\n")
		var commentBlock strings.Builder
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "//") {
				commentBlock.WriteString(strings.TrimPrefix(trimmed, "//"))
				commentBlock.WriteString("\n")
				continue
			}
			if trimmed == "" {
				continue
			}
			break
		}
		blob := commentBlock.String()
		for _, m := range markers {
			if !strings.Contains(blob, m) {
				violations = append(violations, fmt.Sprintf("%s: missing %q in top comment block", path, m))
			}
		}
	}
	return violations
}

func TestAudit_validFile(t *testing.T) {
	dir := t.TempDir()
	src := `// Package x
//
// Objective: do a thing
// Method: try it
// Acceptance criteria: it works
package x
`
	if err := os.WriteFile(filepath.Join(dir, "valid.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := auditDir(dir)
	if len(violations) != 0 {
		t.Errorf("unexpected: %v", violations)
	}
}

func TestAudit_missingMarker(t *testing.T) {
	dir := t.TempDir()
	src := `// Package x
//
// Objective: a thing
// Method: try it
package x
`
	if err := os.WriteFile(filepath.Join(dir, "incomplete.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations := auditDir(dir)
	if len(violations) != 1 {
		t.Errorf("got %d violations, want 1: %v", len(violations), violations)
	}
}
