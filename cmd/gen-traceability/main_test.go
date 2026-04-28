package main

import (
	"strings"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

func TestRenderMatrix_includesAllSections(t *testing.T) {
	body := renderMatrix(matrix.Load())
	for _, want := range []string{
		"## Functional Requirements",
		"## Non-Functional Requirements",
		"## Test Environment",
		"## Source-plan test cases",
		"## New test cases",
		"## Risks",
		"FR-1", "FR-11", "NFR-13", "TE-3",
		"PC-1", "OPS-3", "NEW-FR7", "NEW-NFR13",
		"R1", "R7",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("matrix render missing %q", want)
		}
	}
}

func TestReplaceBlock_happyPath(t *testing.T) {
	src := "before\n<!-- TRACEABILITY:START -->\nold\n<!-- TRACEABILITY:END -->\nafter"
	out, err := replaceBlock(src, "<!-- TRACEABILITY:START -->", "<!-- TRACEABILITY:END -->", "NEW BODY")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "NEW BODY") || strings.Contains(out, "old") {
		t.Errorf("unexpected output: %s", out)
	}
}
