// cmd/teranode-acceptance/main_test.go
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the CLI into a temp dir and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "teranode-acceptance")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return bin
}

func TestCLI_filterTypoExitsWith4(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	cmd := exec.Command(bin,
		"--config", "testdata/integration.yaml",
		"--only", "DOES-NOT-EXIST",
		"--report-json", filepath.Join(dir, "r.json"),
		"--report-html", filepath.Join(dir, "r.html"),
	)
	out, err := cmd.CombinedOutput()
	exitCode := -1
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
	}
	if exitCode != 4 {
		t.Fatalf("want exit 4, got %d\noutput:\n%s", exitCode, out)
	}
	if !strings.Contains(string(out), "DOES-NOT-EXIST") {
		t.Errorf("expected error to mention typo, got: %s", out)
	}
}

func TestCLI_zeroTestsHappyPathExitsWith3(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "r.json")
	htmlPath := filepath.Join(dir, "r.html")
	cmd := exec.Command(bin,
		"--config", "testdata/integration.yaml",
		"--report-json", jsonPath,
		"--report-html", htmlPath,
	)
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() != 3 {
		t.Fatalf("want exit 3, got %d\noutput:\n%s", cmd.ProcessState.ExitCode(), out)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("json report missing: %v", err)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("html report missing: %v", err)
	}
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var parsed struct {
		Verdict struct {
			Decision string `json:"decision"`
			ExitCode int    `json:"exit_code"`
		} `json:"verdict"`
		Requirements    []any `json:"requirements"`
		TestEnvironment []any `json:"test_environment"`
		TestCases       []any `json:"test_cases"`
		Risks           []any `json:"risks"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if parsed.Verdict.Decision != "INCOMPLETE" || parsed.Verdict.ExitCode != 3 {
		t.Errorf("verdict: %+v", parsed.Verdict)
	}
	if len(parsed.Requirements) != 24 {
		t.Errorf("requirements: %d", len(parsed.Requirements))
	}
	if len(parsed.TestEnvironment) != 3 {
		t.Errorf("test_environment: %d", len(parsed.TestEnvironment))
	}
	if len(parsed.TestCases) != 24 {
		t.Errorf("test_cases: %d", len(parsed.TestCases))
	}
	if len(parsed.Risks) != 7 {
		t.Errorf("risks: %d", len(parsed.Risks))
	}
}

func TestCLI_overridesAlonelyDoNotProduceGo(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	overridesPath := filepath.Join(dir, "ovr.yaml")
	if err := os.WriteFile(overridesPath, []byte(`
reviewer: "Test Reviewer"
reviewed_at: "2026-04-29T12:00:00Z"
overrides:
  IBD-1:
    decision: PASS
    artefacts: ["audit.pdf"]
    note: "ok"
  FR-4:
    decision: PASS
    artefacts: ["audit.pdf"]
    note: "ok"
  NFR-1:
    decision: PASS
    artefacts: ["uptime.csv"]
    note: "ok"
  NFR-8:
    decision: PASS
    artefacts: ["docs"]
    note: "ok"
  NFR-9:
    decision: PASS
    artefacts: ["pricing"]
    note: "ok"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin,
		"--config", "testdata/integration.yaml",
		"--reviewer-overrides", overridesPath,
		"--report-json", filepath.Join(dir, "r.json"),
		"--report-html", filepath.Join(dir, "r.html"),
	)
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() != 3 {
		t.Fatalf("with overrides but zero registered tests, want exit 3 (NOT_RUN dominates), got %d\noutput:\n%s",
			cmd.ProcessState.ExitCode(), out)
	}
}
