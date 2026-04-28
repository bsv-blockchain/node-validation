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
			FR8PriorityLevels: []string{"standard"},
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

var _ = matrix.Load // keep matrix import live for test files
