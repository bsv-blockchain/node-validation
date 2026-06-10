// Package chaos implements the privileged, NON-GATING "chaos" test suite
// that exercises operational failure modes against the local docker-compose
// mesh (3× Teranode + 3× SV Node).
//
// It is deliberately kept entirely separate from the acceptance suite in
// internal/testrunner + tests/:
//
//   - It is NOT registered via cmd/teranode-acceptance/register.go.
//   - Its results never feed testrunner.ComputeVerdict.
//   - It ships its own command (cmd/teranode-chaos) and its own report.
//
// The manifest rows OPS-1 and OPS-2 stay EXCLUDED_PRIVILEGED for the
// acceptance suite; this package is an additional, parallel artifact.
//
// # Privileged operations
//
// Unlike the black-box acceptance suite, the chaos suite shells out to the
// `docker` CLI to manipulate the running mesh: disconnect/connect containers
// from the bridge network, kill/start containers, exec into them, and
// inspect their state. It reuses the existing black-box client helpers
// (internal/teranode RPC) for assertions where it can.
//
// # Topology facts (compose project "node-validation")
//
//	bridge network : node-validation_tng-net
//	teranodes      : node-validation-teranode-{1,2,3}-1  (host RPC 19292/29292/39292)
//	sv nodes       : node-validation-svnode-{1,2,3}-1    (host RPC 18332/28332/38332)
//	miner+wallet   : svnode-1 (only node with disablewallet=0)
//
// # Why SV nodes are queried via `docker exec bitcoin-cli`
//
// `docker network disconnect`/`connect` breaks a container's published host
// port mapping until it is restarted, so a partitioned SV node cannot be
// reached on its host RPC port. Querying it from inside the container with
// bitcoin-cli is immune to both the host-port breakage and network
// membership, which keeps assertions reliable across a partition. Teranodes
// are never disconnected (see chaos design notes / OPS-2), so they are
// queried over their stable host RPC ports via the existing RPC client.
package chaos
