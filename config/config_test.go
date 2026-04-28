// config/config_test.go
package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadYAML_minimal(t *testing.T) {
	cfg, err := loadYAMLFile("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("loadYAMLFile: %v", err)
	}
	if cfg.Network != NetworkTestnet {
		t.Errorf("Network: want testnet, got %q", cfg.Network)
	}
	if cfg.Teranode.RPCURL != "http://teranode.example:9292" {
		t.Errorf("Teranode.RPCURL: got %q", cfg.Teranode.RPCURL)
	}
	if cfg.Durations.PC1Observation != 168*time.Hour {
		t.Errorf("PC1Observation: got %v", cfg.Durations.PC1Observation)
	}
}

func TestLoadYAML_missingDefaultIsNotError(t *testing.T) {
	_, err := loadYAMLFile("/tmp/no-such-file-zzzzzz.yaml")
	if !os.IsNotExist(err) {
		t.Fatalf("want IsNotExist, got %v", err)
	}
}
