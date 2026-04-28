// config/load_test.go
package config

import (
	"testing"
	"time"
)

func TestLoad_appliesDefaults(t *testing.T) {
	cfg, err := Load([]string{"--config", "testdata/minimal.yaml", "--allow-mainnet-load"}, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TestTimeout != 30*time.Minute {
		t.Errorf("default TestTimeout: got %v", cfg.TestTimeout)
	}
	if cfg.ReportJSON != "report.json" {
		t.Errorf("default ReportJSON: got %q", cfg.ReportJSON)
	}
}

func TestLoad_envOverridesYAML(t *testing.T) {
	env := []string{"TNG_TERANODE_RPC_URL=http://override.example:1111"}
	cfg, err := Load([]string{"--config", "testdata/minimal.yaml"}, env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Teranode.RPCURL != "http://override.example:1111" {
		t.Errorf("env override failed: got %q", cfg.Teranode.RPCURL)
	}
}

func TestLoad_flagOverridesEnv(t *testing.T) {
	env := []string{"TNG_TERANODE_RPC_URL=http://from-env.example:1111"}
	cfg, err := Load([]string{
		"--config", "testdata/minimal.yaml",
		"--report-json", "/tmp/custom.json",
	}, env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ReportJSON != "/tmp/custom.json" {
		t.Errorf("flag override failed: got %q", cfg.ReportJSON)
	}
}

func TestLoad_shortSubstitution(t *testing.T) {
	cfg, err := Load([]string{"--config", "testdata/minimal.yaml", "--short"}, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Durations.PC1Observation != 30*time.Minute {
		t.Errorf("--short PC1Observation: got %v", cfg.Durations.PC1Observation)
	}
	if cfg.Durations.INTER1Observation != time.Hour {
		t.Errorf("--short INTER1Observation: got %v", cfg.Durations.INTER1Observation)
	}
	// DefaultPropagation is not in the --short list and must remain.
	if cfg.Durations.DefaultPropagation != 10*time.Second {
		t.Errorf("--short corrupted DefaultPropagation: got %v", cfg.Durations.DefaultPropagation)
	}
}
