// config/config.go
//
// Package config loads and validates the runtime configuration for
// the Teranode acceptance-test suite.
package config

import (
	"fmt"
	"os"
	"strings"
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
	RPCURL           string `yaml:"rpc_url"`
	RPCUser          string `yaml:"rpc_user"`
	RPCPass          string `yaml:"rpc_pass"`
	RESTURL          string `yaml:"rest_url"`
	NotificationURL  string `yaml:"notification_url"`
	P2PLegacyAddress string `yaml:"p2p_legacy_address"` // Bitcoin wire P2P (8333 mainnet, 18333 testnet)
	P2PAddress       string `yaml:"p2p_address"`        // libp2p TCP listener (9905)
	MetricsURL       string `yaml:"metrics_url"`
	HealthURL        string `yaml:"health_url"`
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

// Load is the public entry point. Precedence (highest first):
//  1. CLI flags
//  2. Environment variables (TNG_*)
//  3. YAML file (default ./config.yaml)
//  4. Built-in defaults
//
// If --short is set, applyShort runs after the precedence chain.
// Validate runs last and aggregates all errors.
func Load(args []string, environ []string) (Config, error) {
	var cfg Config
	applyDefaults(&cfg)

	// Find --config early so we know which YAML to load. Std flag does not
	// offer this primitive, so we scan args ourselves.
	configPath := defaultConfigPath(args)

	// 1. YAML.
	if configPath != "" {
		yamlCfg, err := loadYAMLFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) || configPath != "config.yaml" {
				return Config{}, fmt.Errorf("loading config: %w", err)
			}
			// Default file missing is fine.
		} else {
			mergeYAML(&cfg, yamlCfg)
		}
	}

	// 2. Env.
	if err := applyEnv(&cfg, environ); err != nil {
		return Config{}, err
	}

	// 3. Flags.
	if err := applyFlags(&cfg, args); err != nil {
		return Config{}, err
	}

	// 4. --short.
	if cfg.Short {
		applyShort(&cfg)
	}

	if err := Validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaultConfigPath(args []string) string {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(args[i], "--config="):
			return strings.TrimPrefix(args[i], "--config=")
		}
	}
	return "config.yaml"
}

// mergeYAML overwrites cfg fields with non-zero values from yamlCfg.
func mergeYAML(dst *Config, src Config) {
	if src.Network != "" {
		dst.Network = src.Network
	}
	if src.Teranode.RPCURL != "" {
		dst.Teranode.RPCURL = src.Teranode.RPCURL
	}
	if src.Teranode.RPCUser != "" {
		dst.Teranode.RPCUser = src.Teranode.RPCUser
	}
	if src.Teranode.RPCPass != "" {
		dst.Teranode.RPCPass = src.Teranode.RPCPass
	}
	if src.Teranode.RESTURL != "" {
		dst.Teranode.RESTURL = src.Teranode.RESTURL
	}
	if src.Teranode.NotificationURL != "" {
		dst.Teranode.NotificationURL = src.Teranode.NotificationURL
	}
	if src.Teranode.P2PLegacyAddress != "" {
		dst.Teranode.P2PLegacyAddress = src.Teranode.P2PLegacyAddress
	}
	if src.Teranode.P2PAddress != "" {
		dst.Teranode.P2PAddress = src.Teranode.P2PAddress
	}
	if src.Teranode.MetricsURL != "" {
		dst.Teranode.MetricsURL = src.Teranode.MetricsURL
	}
	if src.Teranode.HealthURL != "" {
		dst.Teranode.HealthURL = src.Teranode.HealthURL
	}
	if src.SVNode.RPCURL != "" {
		dst.SVNode.RPCURL = src.SVNode.RPCURL
	}
	if src.SVNode.RPCUser != "" {
		dst.SVNode.RPCUser = src.SVNode.RPCUser
	}
	if src.SVNode.RPCPass != "" {
		dst.SVNode.RPCPass = src.SVNode.RPCPass
	}
	if src.SVNode.ZMQBlockURL != "" {
		dst.SVNode.ZMQBlockURL = src.SVNode.ZMQBlockURL
	}
	if src.SVNode.ZMQTxURL != "" {
		dst.SVNode.ZMQTxURL = src.SVNode.ZMQTxURL
	}
	if src.Funding.WIF != "" {
		dst.Funding.WIF = src.Funding.WIF
	}
	if src.Funding.MinBalanceSats != 0 {
		dst.Funding.MinBalanceSats = src.Funding.MinBalanceSats
	}
	if src.Durations.PC1Observation != 0 {
		dst.Durations.PC1Observation = src.Durations.PC1Observation
	}
	if src.Durations.INTER1Observation != 0 {
		dst.Durations.INTER1Observation = src.Durations.INTER1Observation
	}
	if src.Durations.PERF1PerRate != 0 {
		dst.Durations.PERF1PerRate = src.Durations.PERF1PerRate
	}
	if src.Durations.DefaultPropagation != 0 {
		dst.Durations.DefaultPropagation = src.Durations.DefaultPropagation
	}
	if src.Durations.CLIENT1Observation != 0 {
		dst.Durations.CLIENT1Observation = src.Durations.CLIENT1Observation
	}
	if src.Durations.NewNFR7Iterations != 0 {
		dst.Durations.NewNFR7Iterations = src.Durations.NewNFR7Iterations
	}
	if src.Limits.PERF1MaxTPS != 0 {
		dst.Limits.PERF1MaxTPS = src.Limits.PERF1MaxTPS
	}
	if src.Limits.INTER2TxCount != 0 {
		dst.Limits.INTER2TxCount = src.Limits.INTER2TxCount
	}
	if src.Limits.CLIENT3TxCount != 0 {
		dst.Limits.CLIENT3TxCount = src.Limits.CLIENT3TxCount
	}
	if src.Limits.FR7ChainDepth != 0 {
		dst.Limits.FR7ChainDepth = src.Limits.FR7ChainDepth
	}
	if src.Limits.FR10LatencyTargetMs != 0 {
		dst.Limits.FR10LatencyTargetMs = src.Limits.FR10LatencyTargetMs
	}
	if len(src.Limits.FR8PriorityLevels) > 0 {
		dst.Limits.FR8PriorityLevels = src.Limits.FR8PriorityLevels
	}
}
