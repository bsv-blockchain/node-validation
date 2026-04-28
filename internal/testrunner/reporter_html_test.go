// internal/testrunner/reporter_html_test.go
package testrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func TestWriteHTML_structuralPresence(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.Network = "testnet"
	model, err := BuildReportModel(env, nil, overrides.File{}, time.Now().UTC(), time.Now().UTC(), "v0.1.0")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	path := filepath.Join(t.TempDir(), "report.html")
	if err := WriteHTML(path, model); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	html := string(b)
	for _, want := range []string{
		"Teranode Acceptance Tests",
		"<h2>Functional Requirements</h2>",
		"<h2>Non-Functional Requirements</h2>",
		"<h2>Test Environment</h2>",
		"<h2>Test Cases</h2>",
		"<h2>Risks</h2>",
		"banner incomplete",
		"FR-1", "FR-11", "NFR-13", "TE-3",
		"PC-1", "OPS-3", "NEW-FR7", "NEW-NFR13",
		"R1", "R7",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}
