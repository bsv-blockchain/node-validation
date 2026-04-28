// internal/testrunner/reporter_text_test.go
package testrunner

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func TestWriteText_emptyResults(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.Network = "testnet"
	model, err := BuildReportModel(env, nil, overrides.File{},
		time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 9, 30, 0, 0, time.UTC), "v0.1.0")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteText(&buf, model); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, expect := range []string{
		"Teranode Acceptance Tests",
		"Verdict: INCOMPLETE",
		"Requirements:        24",
		"Test cases:          24",
		"Risks:               7",
		"v0.1.0",
	} {
		if !strings.Contains(out, expect) {
			t.Errorf("text output missing %q\n--- output ---\n%s", expect, out)
		}
	}
}
