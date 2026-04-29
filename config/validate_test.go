// config/validate_test.go
package config

import (
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

func validBase() Config {
	return Config{
		Network: NetworkTestnet,
		Teranode: Teranode{
			RPCURL:          "http://teranode.example:9292",
			RESTURL:         "http://teranode.example:8090",
			NotificationURL: "ws://teranode.example:8090/notifications",
			P2PAddress:      "teranode.example:9905",
			MetricsURL:      "http://teranode.example:9000/metrics",
			HealthURL:       "http://teranode.example:9000/health",
		},
		SVNode: SVNode{
			RPCURL:      "http://svnode.example:18332",
			ZMQBlockURL: "tcp://svnode.example:28332",
			ZMQTxURL:    "tcp://svnode.example:28333",
		},
		Funding: Funding{MinBalanceSats: 100000000},
		Durations: Durations{
			PC1Observation: time.Hour, INTER1Observation: time.Hour,
			PERF1PerRate: time.Minute, DefaultPropagation: time.Second,
			CLIENT1Observation: time.Minute, NewNFR7Iterations: 10,
		},
		Limits: Limits{
			PERF1MaxTPS: 1, INTER2TxCount: 1, CLIENT3TxCount: 1,
			FR7ChainDepth: 1, FR10LatencyTargetMs: 1,
			FR8PriorityLevels:  []string{"standard"},
			NFR13MaxProbeRate:  100,
			NFR13ProbeDuration: time.Second,
		},
		ReportJSON: "report.json", ReportHTML: "report.html",
		TestTimeout: time.Minute,
	}
}

func TestValidate_happyPath(t *testing.T) {
	c := validBase()
	if err := Validate(&c); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidate_badNetwork(t *testing.T) {
	c := validBase()
	c.Network = "wonderland"
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Errorf("want network error, got %v", err)
	}
}

func TestValidate_mainnetLoadGate(t *testing.T) {
	c := validBase()
	c.Network = NetworkMainnet
	c.Only = []string{"PERF-1"} // load-generating
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "allow-mainnet-load") {
		t.Errorf("want mainnet-load error, got %v", err)
	}

	c.AllowMainnetLoad = true
	if err := Validate(&c); err != nil {
		t.Errorf("with --allow-mainnet-load, expected pass: %v", err)
	}
}

func TestValidate_unknownOnlyID(t *testing.T) {
	c := validBase()
	c.Only = []string{"PC1"} // typo: should be PC-1
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "PC1") {
		t.Errorf("want unknown-ID error, got %v", err)
	}
}

func TestValidate_onlyAndSkipMutuallyExclusive(t *testing.T) {
	c := validBase()
	c.Only = []string{"PC-1"}
	c.Skip = []string{"PC-2"}
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutual-exclusion error, got %v", err)
	}
}

func TestValidate_badURLScheme(t *testing.T) {
	c := validBase()
	c.Teranode.RPCURL = "://broken"
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "rpc_url") {
		t.Errorf("want URL error, got %v", err)
	}
}

func TestValidate_zeroDurations(t *testing.T) {
	c := validBase()
	c.Durations.PC1Observation = 0
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "pc1_observation") {
		t.Errorf("want zero-duration error, got %v", err)
	}
}

func TestValidate_unknownSkipID(t *testing.T) {
	c := validBase()
	c.Skip = []string{"BOGUS-99"}
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "BOGUS-99") {
		t.Errorf("want unknown skip error, got %v", err)
	}
}

func TestValidate_badURLMissingScheme(t *testing.T) {
	c := validBase()
	c.Teranode.RPCURL = "no-scheme-here"
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Errorf("want missing-scheme error, got %v", err)
	}
}

func TestValidate_badURLWrongScheme(t *testing.T) {
	c := validBase()
	c.Teranode.RPCURL = "ftp://teranode.example:9292"
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Errorf("want wrong-scheme error, got %v", err)
	}
}

func TestValidate_strictConfigRequiresRPCURL(t *testing.T) {
	c := validBase()
	c.StrictConfig = true
	c.Teranode.RPCURL = ""
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "strict-config") {
		t.Errorf("want strict-config error, got %v", err)
	}
}

func TestValidate_mainnetLoadGateAllInScope(t *testing.T) {
	c := validBase()
	c.Network = NetworkMainnet
	// No --only set: all in-scope tests checked; PERF-1 is load-generating.
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "allow-mainnet-load") {
		t.Errorf("want mainnet-load error (all-in-scope), got %v", err)
	}
}

func TestValidate_zeroDurationsMultiple(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantKey string
	}{
		{"inter1_observation", func(c *Config) { c.Durations.INTER1Observation = 0 }, "inter1_observation"},
		{"perf1_per_rate", func(c *Config) { c.Durations.PERF1PerRate = 0 }, "perf1_per_rate"},
		{"default_propagation", func(c *Config) { c.Durations.DefaultPropagation = 0 }, "default_propagation"},
		{"client1_observation", func(c *Config) { c.Durations.CLIENT1Observation = 0 }, "client1_observation"},
		{"new_nfr7_iterations", func(c *Config) { c.Durations.NewNFR7Iterations = 0 }, "new_nfr7_iterations"},
		{"perf1_max_tps", func(c *Config) { c.Limits.PERF1MaxTPS = 0 }, "perf1_max_tps"},
		{"inter2_tx_count", func(c *Config) { c.Limits.INTER2TxCount = 0 }, "inter2_tx_count"},
		{"client3_tx_count", func(c *Config) { c.Limits.CLIENT3TxCount = 0 }, "client3_tx_count"},
		{"fr7_chain_depth", func(c *Config) { c.Limits.FR7ChainDepth = 0 }, "fr7_chain_depth"},
		{"fr10_latency_target_ms", func(c *Config) { c.Limits.FR10LatencyTargetMs = 0 }, "fr10_latency_target_ms"},
		{"fr8_priority_levels", func(c *Config) { c.Limits.FR8PriorityLevels = nil }, "fr8_priority_levels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validBase()
			tt.mutate(&c)
			err := Validate(&c)
			if err == nil || !strings.Contains(err.Error(), tt.wantKey) {
				t.Errorf("want error containing %q, got %v", tt.wantKey, err)
			}
		})
	}
}

func TestValidate_badWIFFormat(t *testing.T) {
	c := validBase()
	c.Funding.WIF = "not-a-wif"
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "wif") {
		t.Errorf("want WIF format error, got %v", err)
	}
}

func TestValidate_validWIFAccepted(t *testing.T) {
	c := validBase()
	// Sample compressed mainnet WIF (do not use this for real funds; it's a known test vector).
	c.Funding.WIF = "L1aW4aubDFB7yfras2S1mN3bqg9nwySY8nkoLmJebSLD5BWv3ENZ"
	if err := Validate(&c); err != nil {
		t.Errorf("valid WIF rejected: %v", err)
	}
}

func TestValidate_NFR13Defaults(t *testing.T) {
	c := validBase()
	if err := Validate(&c); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_NFR13NegativeProbeRate(t *testing.T) {
	c := validBase()
	c.Limits.NFR13MaxProbeRate = -1
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "nfr13_max_probe_rate") {
		t.Errorf("want NFR13MaxProbeRate error, got %v", err)
	}
}

func TestValidate_NFR13NegativeProbeDuration(t *testing.T) {
	c := validBase()
	c.Limits.NFR13ProbeDuration = -1
	err := Validate(&c)
	if err == nil || !strings.Contains(err.Error(), "nfr13_probe_duration") {
		t.Errorf("want NFR13ProbeDuration error, got %v", err)
	}
}

var _ = matrix.Load // keep matrix import live for test files
