package teranode

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/node-validation/config"
)

// Clients is the bundle of every Teranode sub-client. Each field is
// independently nil-safe: a missing URL in cfg yields a nil sub-client.
type Clients struct {
	RPC           *RPCClient
	REST          *RESTClient
	Notifications *NotificationClient
	P2PProbe      *P2PProbe
	P2PWS         *P2PWSClient
	Metrics       *MetricsScraper
	Health        *HealthProbe
}

// NewClients constructs all sub-clients from cfg. Missing fields produce
// nil sub-clients, not errors.
func NewClients(cfg config.Teranode, logger *slog.Logger) (*Clients, error) {
	rpc, err := NewRPCClient(cfg.RPCURL, cfg.RPCUser, cfg.RPCPass, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode rpc: %w", err)
	}
	rest, err := NewRESTClient(cfg.RESTURL, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode rest: %w", err)
	}
	notif, err := NewNotificationClient(cfg.NotificationURL, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode notifications: %w", err)
	}
	met, err := NewMetricsScraper(cfg.MetricsURL, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode metrics: %w", err)
	}
	health, err := NewHealthProbe(cfg.HealthURL, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode health: %w", err)
	}
	var p2p *P2PProbe
	if cfg.P2PLegacyAddress != "" || cfg.P2PAddress != "" {
		p2p = NewP2PProbe(cfg.P2PLegacyAddress, cfg.P2PAddress, logger)
	}
	p2pws, err := NewP2PWSClient(cfg.P2PWSURL, logger)
	if err != nil {
		return nil, fmt.Errorf("teranode p2p-ws: %w", err)
	}
	return &Clients{
		RPC:           rpc,
		REST:          rest,
		Notifications: notif,
		P2PProbe:      p2p,
		P2PWS:         p2pws,
		Metrics:       met,
		Health:        health,
	}, nil
}
