package svnode

import (
	"testing"

	"github.com/bsv-blockchain/node-validation/config"
)

func TestNewClients_allEmpty(t *testing.T) {
	c, err := NewClients(config.SVNode{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.RPC != nil || c.ZMQ != nil {
		t.Errorf("want all nil, got %+v", c)
	}
}

func TestNewClients_RPCOnly(t *testing.T) {
	c, err := NewClients(config.SVNode{RPCURL: "http://svnode:18332"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.RPC == nil {
		t.Error("RPC should be set")
	}
	if c.ZMQ != nil {
		t.Error("ZMQ should be nil")
	}
}
