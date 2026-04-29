// config/env.go
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// applyEnv overlays environment variables onto cfg. Empty values are
// ignored so a deployment that exports TNG_TERANODE_RPC_PASS="" does
// not blank a YAML-supplied password.
func applyEnv(cfg *Config, environ []string) error {
	get := func(name string) (string, bool) {
		val, ok := lookupEnv(environ, name)
		return val, ok && val != ""
	}

	if v, ok := get("TNG_NETWORK"); ok {
		cfg.Network = Network(v)
	}
	if v, ok := get("TNG_TERANODE_RPC_URL"); ok {
		cfg.Teranode.RPCURL = v
	}
	if v, ok := get("TNG_TERANODE_RPC_USER"); ok {
		cfg.Teranode.RPCUser = v
	}
	if v, ok := get("TNG_TERANODE_RPC_PASS"); ok {
		cfg.Teranode.RPCPass = v
	}
	if v, ok := get("TNG_TERANODE_REST_URL"); ok {
		cfg.Teranode.RESTURL = v
	}
	if v, ok := get("TNG_TERANODE_NOTIFICATION_URL"); ok {
		cfg.Teranode.NotificationURL = v
	}
	if v, ok := get("TNG_TERANODE_P2P_LEGACY_ADDRESS"); ok {
		cfg.Teranode.P2PLegacyAddress = v
	}
	if v, ok := get("TNG_TERANODE_P2P_ADDRESS"); ok {
		cfg.Teranode.P2PAddress = v
	}
	if v, ok := get("TNG_TERANODE_METRICS_URL"); ok {
		cfg.Teranode.MetricsURL = v
	}
	if v, ok := get("TNG_TERANODE_HEALTH_URL"); ok {
		cfg.Teranode.HealthURL = v
	}
	if v, ok := get("TNG_SVNODE_RPC_URL"); ok {
		cfg.SVNode.RPCURL = v
	}
	if v, ok := get("TNG_SVNODE_RPC_USER"); ok {
		cfg.SVNode.RPCUser = v
	}
	if v, ok := get("TNG_SVNODE_RPC_PASS"); ok {
		cfg.SVNode.RPCPass = v
	}
	if v, ok := get("TNG_SVNODE_ZMQ_BLOCK_URL"); ok {
		cfg.SVNode.ZMQBlockURL = v
	}
	if v, ok := get("TNG_SVNODE_ZMQ_TX_URL"); ok {
		cfg.SVNode.ZMQTxURL = v
	}
	if v, ok := get("TNG_FUNDING_WIF"); ok {
		cfg.Funding.WIF = v
	}
	if v, ok := get("TNG_FUNDING_MIN_BALANCE_SATOSHIS"); ok {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("TNG_FUNDING_MIN_BALANCE_SATOSHIS: %w", err)
		}
		cfg.Funding.MinBalanceSats = n
	}
	if v, ok := get("TNG_TEST_TIMEOUT"); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("TNG_TEST_TIMEOUT: %w", err)
		}
		cfg.TestTimeout = d
	}
	return nil
}

func lookupEnv(environ []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range environ {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	return "", false
}
