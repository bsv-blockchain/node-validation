// config/validate.go
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

// loadGeneratingTests is the set of in-scope tests that produce
// significant transaction load. Used by the mainnet safety gate.
var loadGeneratingTests = map[string]bool{
	"PERF-1": true, "INTER-2": true, "CLIENT-3": true,
	"NEW-FR7": true, "NEW-NFR7": true, "NEW-NFR13": true,
}

// Validate enforces structural and semantic rules on a fully-merged Config.
// All errors are aggregated into a single multi-line message.
func Validate(c *Config) error {
	var errs []string

	switch c.Network {
	case NetworkMainnet, NetworkTestnet, NetworkRegtest:
	default:
		errs = append(errs, fmt.Sprintf("network: must be mainnet|testnet|regtest, got %q", c.Network))
	}

	checkURL := func(name, raw string, schemes ...string) {
		if raw == "" {
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			return
		}
		if u.Scheme == "" {
			errs = append(errs, fmt.Sprintf("%s: missing scheme in %q", name, raw))
			return
		}
		ok := false
		for _, s := range schemes {
			if u.Scheme == s {
				ok = true
				break
			}
		}
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: scheme %q not in %v", name, u.Scheme, schemes))
		}
	}
	checkURL("teranode.rpc_url", c.Teranode.RPCURL, "http", "https")
	checkURL("teranode.rest_url", c.Teranode.RESTURL, "http", "https")
	checkURL("teranode.notification_url", c.Teranode.NotificationURL, "ws", "wss", "http", "https")
	checkURL("teranode.metrics_url", c.Teranode.MetricsURL, "http", "https")
	checkURL("teranode.health_url", c.Teranode.HealthURL, "http", "https")
	checkURL("svnode.rpc_url", c.SVNode.RPCURL, "http", "https")
	checkURL("svnode.zmq_block_url", c.SVNode.ZMQBlockURL, "tcp")
	checkURL("svnode.zmq_tx_url", c.SVNode.ZMQTxURL, "tcp")

	if c.Funding.WIF != "" {
		wifPattern := regexp.MustCompile(`^[59KLc][1-9A-HJ-NP-Za-km-z]{50,51}$`)
		if !wifPattern.MatchString(c.Funding.WIF) {
			errs = append(errs, "funding.wif: not a valid base58 WIF (mainnet/testnet WIFs start with 5/K/L/c)")
		}
	}

	if c.StrictConfig && c.Teranode.RPCURL == "" {
		errs = append(errs, "teranode.rpc_url required under --strict-config")
	}

	if len(c.Only) > 0 && len(c.Skip) > 0 {
		errs = append(errs, "--only and --skip are mutually exclusive")
	}

	m := matrix.Load()
	known := map[string]bool{}
	for _, e := range m.Entries {
		known[e.ID] = true
	}
	for _, id := range c.Only {
		if !known[id] {
			errs = append(errs, fmt.Sprintf("--only: unknown test ID %q (did you mean PC-1 instead of PC1?)", id))
		}
	}
	for _, id := range c.Skip {
		if !known[id] {
			errs = append(errs, fmt.Sprintf("--skip: unknown test ID %q", id))
		}
	}

	if c.Network == NetworkMainnet && !c.AllowMainnetLoad {
		requested := c.Only
		if len(requested) == 0 {
			requested = m.InScopeTestIDs()
		}
		for _, id := range requested {
			if loadGeneratingTests[id] {
				errs = append(errs, fmt.Sprintf("network=mainnet requires --allow-mainnet-load when test %s is in the requested set", id))
				break
			}
		}
	}

	if c.Durations.PC1Observation <= 0 {
		errs = append(errs, "durations.pc1_observation must be > 0")
	}
	if c.Durations.INTER1Observation <= 0 {
		errs = append(errs, "durations.inter1_observation must be > 0")
	}
	if c.Durations.PERF1PerRate <= 0 {
		errs = append(errs, "durations.perf1_per_rate must be > 0")
	}
	if c.Durations.DefaultPropagation <= 0 {
		errs = append(errs, "durations.default_propagation must be > 0")
	}
	if c.Durations.CLIENT1Observation <= 0 {
		errs = append(errs, "durations.client1_observation must be > 0")
	}
	if c.Durations.NewNFR7Iterations < 1 {
		errs = append(errs, "durations.new_nfr7_iterations must be >= 1")
	}
	if c.Limits.PERF1MaxTPS <= 0 {
		errs = append(errs, "limits.perf1_max_tps must be > 0")
	}
	if c.Limits.INTER2TxCount <= 0 {
		errs = append(errs, "limits.inter2_tx_count must be > 0")
	}
	if c.Limits.CLIENT3TxCount <= 0 {
		errs = append(errs, "limits.client3_tx_count must be > 0")
	}
	if c.Limits.FR7ChainDepth <= 0 {
		errs = append(errs, "limits.fr7_chain_depth must be > 0")
	}
	if c.Limits.FR10LatencyTargetMs <= 0 {
		errs = append(errs, "limits.fr10_latency_target_ms must be > 0")
	}
	if len(c.Limits.FR8PriorityLevels) == 0 {
		errs = append(errs, "limits.fr8_priority_levels must list at least one level")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config invalid:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
