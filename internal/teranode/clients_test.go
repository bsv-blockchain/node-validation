package teranode

import (
	"testing"

	"github.com/bsv-blockchain/node-validation/config"
)

func TestNewClients_allEmpty(t *testing.T) {
	c, err := NewClients(config.Teranode{}, nil)
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if c.RPC != nil || c.REST != nil || c.Notifications != nil || c.P2PProbe != nil || c.Metrics != nil || c.Health != nil {
		t.Errorf("expected all nil sub-clients, got %+v", c)
	}
}

func TestNewClients_P2PAddressesWired(t *testing.T) {
	c, err := NewClients(config.Teranode{
		P2PLegacyAddress: "host:18333",
		P2PAddress:       "host:9905",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.P2PProbe == nil {
		t.Fatal("P2PProbe should be set")
	}
	if c.P2PProbe.LegacyAddr() != "host:18333" {
		t.Errorf("legacy addr: %q", c.P2PProbe.LegacyAddr())
	}
	if c.P2PProbe.Libp2pAddr() != "host:9905" {
		t.Errorf("libp2p addr: %q", c.P2PProbe.Libp2pAddr())
	}
}

func TestNewClients_partialConfig(t *testing.T) {
	c, err := NewClients(config.Teranode{
		RPCURL:     "http://teranode:9292",
		MetricsURL: "http://teranode:9091/metrics",
	}, nil)
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if c.RPC == nil {
		t.Error("RPC should be present")
	}
	if c.Metrics == nil {
		t.Error("Metrics should be present")
	}
	if c.REST != nil {
		t.Error("REST should be nil")
	}
}
