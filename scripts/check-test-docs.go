// Command check-test-docs asserts every tests/<id>.go file's top-of-file
// comment block contains "Objective:", "Method:", and "Acceptance criteria:"
// markers. Run via:
//
//	go run ./scripts/check-test-docs.go --tests-dir tests/
//
// Skips fixtures.go, helper.go, doc.go, and any *_test.go files. Reports
// violations to stderr; exits 1 if any are found.

//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var skipFiles = map[string]bool{
	"fixtures.go": true, "helper.go": true, "doc.go": true,
	"tests_test.go": true, "fixtures_test.go": true,
}

var requiredMarkers = []string{"Objective:", "Method:", "Acceptance criteria:"}

// auditDir walks dir and returns a list of violation strings (one per missing
// marker per file). An empty slice means the audit passed.
func auditDir(dir string) []string {
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
		if skipFiles[name] {
			continue
		}
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: read: %v", path, err))
			continue
		}
		// Heuristic: extract the leading comment block (lines starting with "//").
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
		for _, m := range requiredMarkers {
			if !strings.Contains(blob, m) {
				violations = append(violations, fmt.Sprintf("%s: missing %q in top comment block", path, m))
			}
		}
	}
	return violations
}

func main() {
	dir := flag.String("tests-dir", "tests", "directory containing tests/*.go files")
	flag.Parse()

	violations := auditDir(*dir)
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Doc-comment audit failed:")
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, "  - "+v)
		}
		os.Exit(1)
	}
	fmt.Printf("check-test-docs: OK (audited %s)\n", *dir)
}
