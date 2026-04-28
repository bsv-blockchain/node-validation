// config/env_test.go
package config

import (
	"strings"
	"testing"
)

func TestApplyEnv_badMinBalanceSatoshis(t *testing.T) {
	cfg := Config{}
	err := applyEnv(&cfg, []string{"TNG_FUNDING_MIN_BALANCE_SATOSHIS=not-a-number"})
	if err == nil || !strings.Contains(err.Error(), "TNG_FUNDING_MIN_BALANCE_SATOSHIS") {
		t.Errorf("want parse error, got %v", err)
	}
}

func TestApplyEnv_badTestTimeout(t *testing.T) {
	cfg := Config{}
	err := applyEnv(&cfg, []string{"TNG_TEST_TIMEOUT=bad-duration"})
	if err == nil || !strings.Contains(err.Error(), "TNG_TEST_TIMEOUT") {
		t.Errorf("want parse error, got %v", err)
	}
}

func TestApplyEnv_validMinBalanceSatoshis(t *testing.T) {
	cfg := Config{}
	err := applyEnv(&cfg, []string{"TNG_FUNDING_MIN_BALANCE_SATOSHIS=500000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Funding.MinBalanceSats != 500000 {
		t.Errorf("want 500000, got %d", cfg.Funding.MinBalanceSats)
	}
}

func TestApplyEnv_validTestTimeout(t *testing.T) {
	cfg := Config{}
	err := applyEnv(&cfg, []string{"TNG_TEST_TIMEOUT=5m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TestTimeout.String() != "5m0s" {
		t.Errorf("want 5m0s, got %s", cfg.TestTimeout)
	}
}

func TestApplyEnv_emptyValueIgnored(t *testing.T) {
	cfg := Config{Network: NetworkTestnet}
	err := applyEnv(&cfg, []string{"TNG_NETWORK="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Network != NetworkTestnet {
		t.Errorf("empty env should not override; got %q", cfg.Network)
	}
}

func TestApplyEnv_allStringFields(t *testing.T) {
	cfg := Config{}
	env := []string{
		"TNG_NETWORK=testnet",
		"TNG_TERANODE_RPC_URL=http://rpc.example:9292",
		"TNG_TERANODE_RPC_USER=user1",
		"TNG_TERANODE_RPC_PASS=pass1",
		"TNG_TERANODE_REST_URL=http://rest.example:8090",
		"TNG_TERANODE_NOTIFICATION_URL=ws://notif.example:8090",
		"TNG_TERANODE_P2P_ADDRESS=p2p.example:9905",
		"TNG_TERANODE_METRICS_URL=http://metrics.example:9000",
		"TNG_TERANODE_HEALTH_URL=http://health.example:9000",
		"TNG_SVNODE_RPC_URL=http://svnode.example:18332",
		"TNG_SVNODE_RPC_USER=svuser",
		"TNG_SVNODE_RPC_PASS=svpass",
		"TNG_SVNODE_ZMQ_BLOCK_URL=tcp://svnode.example:28332",
		"TNG_SVNODE_ZMQ_TX_URL=tcp://svnode.example:28333",
		"TNG_FUNDING_WIF=L1aW4aubDFB7yfras2S1mN3bqg9nwySY8nkoLmJebSLD5BWv3ENZ",
	}
	if err := applyEnv(&cfg, env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Network != NetworkTestnet {
		t.Errorf("TNG_NETWORK: got %q", cfg.Network)
	}
	if cfg.Funding.WIF != "L1aW4aubDFB7yfras2S1mN3bqg9nwySY8nkoLmJebSLD5BWv3ENZ" {
		t.Errorf("TNG_FUNDING_WIF: got %q", cfg.Funding.WIF)
	}
	if cfg.SVNode.RPCUser != "svuser" {
		t.Errorf("TNG_SVNODE_RPC_USER: got %q", cfg.SVNode.RPCUser)
	}
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"PC-1,PC-2", []string{"PC-1", "PC-2"}},
		{" PC-1 , PC-2 ", []string{"PC-1", "PC-2"}},
		{"PC-1,,PC-2", []string{"PC-1", "PC-2"}},
		{"single", []string{"single"}},
		{"", []string{}},
	}
	for _, tc := range cases {
		got := splitCSV(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitCSV(%q): got %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitCSV(%q)[%d]: got %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}
