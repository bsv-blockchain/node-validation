// internal/testrunner/reporter_json_test.go
package testrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/overrides"
)

func TestWriteJSON_completeReport(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.Network = "testnet"
	model, err := BuildReportModel(env, nil, overrides.File{}, time.Now().UTC(), time.Now().UTC(), "v0.1.0")
	if err != nil {
		t.Fatalf("BuildReportModel: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := WriteJSON(path, model); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("parse: %v\n--- json ---\n%s", err, b)
	}
	for _, key := range []string{"run", "verdict", "requirements", "test_environment", "test_cases", "risks", "summary"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	if reqs, _ := parsed["requirements"].([]any); len(reqs) != 24 {
		t.Errorf("requirements: want 24, got %d", len(reqs))
	}
	if tcs, _ := parsed["test_cases"].([]any); len(tcs) != 24 {
		t.Errorf("test_cases: want 24, got %d", len(tcs))
	}
	if risks, _ := parsed["risks"].([]any); len(risks) != 7 {
		t.Errorf("risks: want 7, got %d", len(risks))
	}
}
