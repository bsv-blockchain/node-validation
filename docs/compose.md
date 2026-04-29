# Compose stack — operator's guide

The `compose/` directory provides a regtest network with 3 Teranodes and
3 SV nodes for end-to-end acceptance testing.

## Prerequisites

- Docker Engine ≥ 24
- Docker Compose v2 (the `docker compose` plugin)
- bash, curl, jq on the host
- ~8 GB RAM free (Aerospike + Postgres + Kafka + 3× Teranode + 3× SV node)
- ~30 GB disk for images + ephemeral state

## Quickstart

```bash
make build               # builds bin/teranode-acceptance + bin/derive-address
make compose-up          # boots the stack and runs bootstrap
make compose-test        # runs the acceptance suite against the stack
make compose-down        # tears down (removes containers and volumes)
```

## Host port table

| Service | Host port | Container port | Purpose |
|---|---|---|---|
| teranode-1 RPC | 19292 | 9292 | JSON-RPC |
| teranode-1 REST | 18090 | 8090 | Asset API + WS |
| teranode-1 metrics | 19091 | 9091 | Prometheus |
| teranode-1 health | 18000 | 8000 | health/readiness |
| teranode-1 legacy P2P | 18444 | 18444 | BSV-wire P2P |
| teranode-1 libp2p | 19905 | 9905 | libp2p TCP |
| teranode-2 (same suite) | 29292/28090/29091/28000/28444/29905 | | |
| teranode-3 (same suite) | 39292/38090/39091/38000/38444/39905 | | |
| svnode-1 RPC | 18332 | 18332 | bitcoind RPC (wallet) |
| svnode-1 P2P | 14444 | 18444 | BSV-wire P2P |
| svnode-1 ZMQ block | 18331 | 28332 | ZMQ hashblock |
| svnode-1 ZMQ tx | 18330 | 28333 | ZMQ rawtx |
| svnode-2 RPC / P2P | 28332 / 24444 | 18332 / 18444 | (no wallet) |
| svnode-3 RPC / P2P | 38332 / 34444 | 18332 / 18444 | (no wallet) |
| aerospike | 13000 | 3000 | client port |
| postgres | 15432 | 5432 | psql |
| kafka | 19092 | 19092 | external listener |

## Debugging

```bash
make compose-logs SERVICE=teranode-1
make compose-logs SERVICE=svnode-1
make compose-logs SERVICE=aerospike
```

Connect to a node directly:

```bash
curl -u bitcoin:bitcoin -d '{"method":"getblockchaininfo","id":1}' http://localhost:18332/
curl http://localhost:18000/health/readiness
```

## Reset

`make compose-down` removes containers AND volumes. `make compose-up` then
starts fresh. There is no incremental restart — state is ephemeral by
design.

## Version note

The Teranode image is pinned to `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2`.
SP2 discovery (see `docs/discovery.md`) was performed against commit
`11f5fa6a81c36490e2796561f76a39294fc422b5` from a feature branch. If the
v0.15.0-beta-2 image was built from a different commit, some endpoint
behaviour may differ. The done-check (`scripts/sp4-docker-done-check.sh`)
smoke-tests basic RPC reachability; deeper drift surfaces in SP5+ tests.

## Bootstrap notes

`compose/bootstrap.sh`:

- mines 110 blocks via svnode-1's wallet (past coinbase maturity)
- waits up to 60s for the mesh to converge on one tip
- derives the test wallet address from the WIF in `config.docker.yaml`
- funds it with 10 BSV via `sendtoaddress`
- mines 1 confirmation block

The script is idempotent — re-running it after `compose-up` is safe;
it just mines more blocks.

## Genesis + Chronicle activation

The Teranode regtest config forces both consensus rule activations to
block 1 (`genesis_activation_height = 1`, `chronicle_activation_height = 1`).
This means tests exercising post-Genesis script semantics work without
mining past historical heights.

If Teranode rejects regtest's genesis block under these rules, startup
will fail loudly with a chain-validation error. Easy to spot in
`make compose-logs SERVICE=teranode-1`.
