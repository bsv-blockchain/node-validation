// internal/svnode/clients.go
package svnode

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/node-validation/config"
)

type Clients struct {
	RPC *RPCClient
	ZMQ *ZMQClient
}

func NewClients(cfg config.SVNode, logger *slog.Logger) (*Clients, error) {
	rpc, err := NewRPCClient(cfg.RPCURL, cfg.RPCUser, cfg.RPCPass, logger)
	if err != nil {
		return nil, fmt.Errorf("svnode rpc: %w", err)
	}
	zmq, err := NewZMQClient(cfg.ZMQBlockURL, cfg.ZMQTxURL, logger)
	if err != nil {
		return nil, fmt.Errorf("svnode zmq: %w", err)
	}
	return &Clients{RPC: rpc, ZMQ: zmq}, nil
}
