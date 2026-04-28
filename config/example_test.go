// config/example_test.go
package config

import "testing"

func TestExampleYAMLLoads(t *testing.T) {
	cfg, err := loadYAMLFile("../config.example.yaml")
	if err != nil {
		t.Fatalf("config.example.yaml failed to parse: %v", err)
	}
	if cfg.Network != NetworkTestnet {
		t.Errorf("example network: got %q", cfg.Network)
	}
	if cfg.Teranode.RPCURL == "" {
		t.Error("example must set teranode.rpc_url")
	}
}
