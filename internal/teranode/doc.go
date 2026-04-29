// Package teranode contains typed clients for every external interface
// Teranode exposes (RPC, REST, Centrifuge notifications, P2P probe,
// Prometheus metrics, health). Each sub-client is independently usable
// and nil-safe — when the corresponding URL is empty in config, the
// sub-client is nil and consumers in tests/ skip cleanly.
//
// The clients are constructed by NewClients(cfg.Teranode, logger) at
// startup; see internal/teranode/clients.go.
package teranode
