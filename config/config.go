// config/config.go
//
// Package config loads and validates the runtime configuration for
// the Teranode acceptance-test suite.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Network identifies the Bitcoin SV network the suite targets.
type Network string

const (
	NetworkMainnet Network = "mainnet"
	NetworkTestnet Network = "testnet"
	NetworkRegtest Network = "regtest"
)

// Config is the fully-loaded, fully-validated configuration.
type Config struct {
	Network   Network   `yaml:"network"`
	Teranode  Teranode  `yaml:"teranode"`
	SVNode    SVNode    `yaml:"svnode"`
	Funding   Funding   `yaml:"funding"`
	Durations Durations `yaml:"durations"`
	Limits    Limits    `yaml:"limits"`

	// CLI-only fields.
	ConfigPath        string        `yaml:"-"`
	Only              []string      `yaml:"-"`
	Skip              []string      `yaml:"-"`
	ReportJSON        string        `yaml:"-"`
	ReportHTML        string        `yaml:"-"`
	Verbose           bool          `yaml:"-"`
	Short             bool          `yaml:"-"`
	AllowMainnetLoad  bool          `yaml:"-"`
	StrictConfig      bool          `yaml:"-"`
	ReviewerOverrides string        `yaml:"-"`
	TestTimeout       time.Duration `yaml:"-"`
}

// Teranode endpoints under test.
type Teranode struct {
	RPCURL          string `yaml:"rpc_url"`
	RPCUser         string `yaml:"rpc_user"`
	RPCPass         string `yaml:"rpc_pass"`
	RESTURL         string `yaml:"rest_url"`
	NotificationURL string `yaml:"notification_url"`
	P2PAddress      string `yaml:"p2p_address"`
	MetricsURL      string `yaml:"metrics_url"`
	HealthURL       string `yaml:"health_url"`
}

// SVNode reference endpoints.
type SVNode struct {
	RPCURL      string `yaml:"rpc_url"`
	RPCUser     string `yaml:"rpc_user"`
	RPCPass     string `yaml:"rpc_pass"`
	ZMQBlockURL string `yaml:"zmq_block_url"`
	ZMQTxURL    string `yaml:"zmq_tx_url"`
}

// Funding identifies a wallet that pays for test transactions.
type Funding struct {
	WIF            string `yaml:"wif"`
	MinBalanceSats uint64 `yaml:"min_balance_satoshis"`
}

// Durations groups long-running test durations.
type Durations struct {
	PC1Observation     time.Duration `yaml:"pc1_observation"`
	INTER1Observation  time.Duration `yaml:"inter1_observation"`
	PERF1PerRate       time.Duration `yaml:"perf1_per_rate"`
	DefaultPropagation time.Duration `yaml:"default_propagation"`
	CLIENT1Observation time.Duration `yaml:"client1_observation"`
	NewNFR7Iterations  int           `yaml:"new_nfr7_iterations"`
}

// Limits caps load-generating tests.
type Limits struct {
	PERF1MaxTPS         int      `yaml:"perf1_max_tps"`
	INTER2TxCount       int      `yaml:"inter2_tx_count"`
	CLIENT3TxCount      int      `yaml:"client3_tx_count"`
	FR7ChainDepth       int      `yaml:"fr7_chain_depth"`
	FR10LatencyTargetMs int      `yaml:"fr10_latency_target_ms"`
	FR8PriorityLevels   []string `yaml:"fr8_priority_levels"`
}

// loadYAMLFile parses a YAML file into a Config without applying defaults
// or running validation.
func loadYAMLFile(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}
