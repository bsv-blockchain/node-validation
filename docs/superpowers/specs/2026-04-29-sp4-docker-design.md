# SP4-DOCKER — Test Environment (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP4-DOCKER / 11 — Multi-node Docker compose stack
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-29
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1 (skeleton), SP2 (discovery), SP3 (clients), SP4 (txgen)
**Parallel to:** SP4 (already complete)
**Status:** awaiting user review

---

## 1. Purpose

Stand up a single regtest network with 3 Teranodes and 3 SV nodes, sharing infrastructure (Aerospike, Postgres, Kafka), suitable for running the full acceptance suite end-to-end. The stack mirrors `../teranode/compose/` patterns, pinned to Teranode `v0.15.0-beta-2` and a wallet-enabled SV node faucet. Genesis and Chronicle consensus activations are forced from block 1 so script-rule tests don't have to mine past historical heights.

## 2. Scope

### In scope

- A `compose/` directory at the project root with:
  - `docker-compose.yml` — single file defining all containers and one bridge network.
  - `teranode/settings.docker.conf` — Teranode settings overrides (regtest, ports, genesis/chronicle activation = 1, shared-infra connection strings).
  - `svnode/bitcoin.conf` for each SV node (regtest, peer connections, wallet enabled on svnode-1 only).
  - `aerospike/aerospike.conf` — one server, three namespaces (`teranode1`, `teranode2`, `teranode3`).
  - `kafka/` — Kafka + Zookeeper with topic-prefix conventions per node.
  - `postgres/init.sql` — creates `teranode1`, `teranode2`, `teranode3` databases.
  - `bootstrap.sh` — waits for readiness, mines 110 initial blocks against svnode-1's wallet, funds the test wallet address, prints the bootstrap txid.
  - `teardown.sh` — `docker compose down -v` plus optional cleanup.
- A `config.docker.yaml` at project root pointing the test suite at the compose stack's exposed ports.
- New `Makefile` targets:
  - `make compose-up` — start stack, wait for health, run bootstrap.
  - `make compose-down` — stop and remove (including volumes).
  - `make compose-logs SERVICE=teranode-1` — tail one service.
  - `make compose-test` — runs `compose-up` (idempotent) then `make test-short` against `config.docker.yaml`.
  - `make compose-reset` — alias for `compose-down` (kept for symmetry).
- `scripts/sp4-docker-done-check.sh` — mechanical SP4-DOCKER definition-of-done.
- `docs/compose.md` — operator's guide: prerequisites, ports, debugging, reset procedure.

### Out of scope

- Production-grade tuning (memory limits, CPU pinning, persistent caches across reboots).
- Multi-host deployment / Kubernetes manifests.
- Automated image rebuilds when the upstream Teranode source changes.
- IPv6-only or NAT-traversal scenarios — single-host single-bridge.
- TLS / mTLS between containers — plain HTTP / TCP within the network.
- Provisioning a CI runner large enough to hold the stack — operator runs locally.

## 3. Architecture

```
                ┌─────────────────────── docker bridge: tng-net ───────────────────────┐
                │                                                                       │
                │   ┌──────────┐  ┌──────────┐  ┌──────────┐                            │
                │   │teranode-1│  │teranode-2│  │teranode-3│                            │
                │   └────┬─────┘  └────┬─────┘  └────┬─────┘                            │
                │        │             │             │                                  │
                │        ▼             ▼             ▼                                  │
                │   ┌─────────────────────────────────┐ ┌───────────────────────────┐   │
                │   │ aerospike (NS: teranode{1,2,3}) │ │ postgres (DB: teranode{1,2,3}) │
                │   └─────────────────────────────────┘ └───────────────────────────┘   │
                │                                                                       │
                │   ┌─────────────────┐                                                  │
                │   │ kafka+zookeeper │  topic prefix per node: teranode-1.*, ...        │
                │   └─────────────────┘                                                  │
                │                                                                       │
                │   ┌──────────┐  ┌──────────┐  ┌──────────┐                            │
                │   │ svnode-1 │  │ svnode-2 │  │ svnode-3 │   regtest, all mesh-peered │
                │   │ (wallet) │  │          │  │          │   to each other AND to     │
                │   └──────────┘  └──────────┘  └──────────┘   teranode-{1,2,3}.legacy  │
                │                                                                       │
                └───────────────────────────────────────────────────────────────────────┘

              host port mapping (for tests + manual probing):
                teranode-1 RPC: localhost:19292    teranode-2: 29292    teranode-3: 39292
                teranode-1 REST: localhost:18090   ...           28090   ...           38090
                teranode-1 WS:   localhost:18090/connection/websocket  (same port as REST)
                teranode-1 metrics: 19091          ...           29091   ...           39091
                teranode-1 health:  18000          ...           28000   ...           38000
                svnode-1 RPC: localhost:18332      svnode-2: 28332     svnode-3: 38332
                svnode-1 ZMQ block: localhost:18331  svnode-1 ZMQ tx: 18330
                aerospike: 13000   postgres: 15432   kafka: 19092
```

The "PORT_PREFIX" convention from `../teranode/compose/` (digit prefix per node) is adopted: node N's port is `(N0)PPPP` where `PPPP` is the canonical port. For containers internal to the network, services connect via Docker DNS (`teranode-1`, `aerospike`, etc.) using canonical ports.

### 3.1 Container manifest (12 services total)

| Service | Image | Internal ports | Host ports |
|---|---|---|---|
| `teranode-1` | `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2` | 9292 (rpc), 8090 (rest+ws), 9091 (metrics), 8000 (health), 18444 (legacy P2P), 9905 (libp2p) | 19292, 18090, 19091, 18000, 18444, 19905 |
| `teranode-2` | same | same | 29292, 28090, 29091, 28000, 28444, 29905 |
| `teranode-3` | same | same | 39292, 38090, 39091, 38000, 38444, 39905 |
| `svnode-1` (wallet) | `bitcoinsv/bitcoin-sv:1.1.0` (or pinned latest) | 18332 (rpc), 18444 (p2p), 28332 (zmq block), 28333 (zmq tx) | 18332, 14444, 28332, 28333 |
| `svnode-2` | same | same | 28332, 24444 |
| `svnode-3` | same | same | 38332, 34444 |
| `aerospike` | `aerospike/aerospike-server:6.4` | 3000 | 13000 |
| `postgres` | `postgres:15` | 5432 | 15432 |
| `zookeeper` | `confluentinc/cp-zookeeper:7.5.0` | 2181 | (internal only) |
| `kafka` | `confluentinc/cp-kafka:7.5.0` | 9092, 9093 | 19092 |

**Note on SV Node port conflict.** Both Teranodes (in regtest) and SV nodes use port `18444` for legacy P2P. To keep the host-side mapping unambiguous, host-port suffixes diverge: teranode-1 → `18444`, svnode-1 → `14444`, etc. Internal-network ports stay 18444 across all 6 nodes (Docker DNS routes by hostname).

### 3.2 Internal P2P mesh

Six full-mesh peer connections. SV node `bitcoin.conf` lists `addnode=teranode-1:18444 ... teranode-3:18444 ... svnode-2:18444 svnode-3:18444`. Teranode `settings.docker.conf` lists `legacy_config_AddPeers=svnode-1:18444,svnode-2:18444,svnode-3:18444,teranode-2:18444,teranode-3:18444` (and analogous for the other two Teranodes).

This forms a mesh where every blockchain message reaches every node within hops ≤ 2.

## 4. Teranode regtest configuration

The compose `teranode/settings.docker.conf` extends `settings.conf` from `../teranode/` with regtest-specific overrides:

```conf
SETTINGS_CONTEXT = docker

# --- network ---
network = regtest

# --- consensus activations forced from block 1 ---
genesis_activation_height = 1
chronicle_activation_height = 1

# --- shared infra ---
postgres_url = postgresql://teranode:teranode@postgres:5432/teranode${PORT_PREFIX}
aerospike_host = aerospike
aerospike_port = 3000
aerospike_namespace = teranode${PORT_PREFIX}
kafka_brokers = kafka:9092
kafka_topic_prefix = teranode-${PORT_PREFIX}

# --- enabled services ---
startBlockchain = true
startBlockAssembly = true
startBlockValidation = true
startSubtreeValidation = true
startPropagation = true
startValidator = true
startP2P = true
startLegacy = true
startAsset = true
startRPC = true
startBlockPersister = true

# --- RPC auth ---
rpc_user = bitcoin
rpc_pass = bitcoin

# --- legacy peers (3 Teranodes + 3 SV nodes minus self) ---
# Filled in per-node via PORT_PREFIX-conditional sections.
```

The `PORT_PREFIX` env var (1, 2, or 3) is set per-container in `docker-compose.yml`; gocore's context system applies the matching `.docker` section. Teranode-1 gets `PORT_PREFIX=1`, teranode-2 gets `2`, etc.

### 4.1 Genesis + Chronicle from block 1

Setting `genesis_activation_height = 1` and `chronicle_activation_height = 1` activates Genesis (post-2020 BSV consensus) and Chronicle (later BSV upgrade) from the first block. This means:
- All historical script flags (CLEANSTACK, MINIMALDATA, etc.) are active immediately.
- No "transition" period where old rules apply.
- Tests that exercise post-Genesis script semantics work without mining past activation heights.

This is the right default for an acceptance test environment that wants to exercise current consensus.

## 5. SV Node configuration

`compose/svnode/bitcoin.conf` (shared base, with per-node tweaks):

```conf
regtest=1
server=1
listen=1
txindex=1
debug=net
debug=mempool

# RPC (open to host for tests)
rpcuser=bitcoin
rpcpassword=bitcoin
rpcallowip=0.0.0.0/0
rpcbind=0.0.0.0

# ZMQ (svnode-1 only — used by SP3's ZMQ subscriber)
zmqpubhashblock=tcp://0.0.0.0:28332
zmqpubrawtx=tcp://0.0.0.0:28333

# Mesh peers (rest filled per node)
addnode=teranode-1:18444
addnode=teranode-2:18444
addnode=teranode-3:18444
```

svnode-1 additionally has:

```conf
# Wallet — required by SP4 txgen Bootstrap (sendtoaddress)
disablewallet=0
```

svnode-2, svnode-3 add:

```conf
disablewallet=1
```

## 6. Shared infrastructure

### 6.1 Aerospike

`compose/aerospike/aerospike.conf` — one server, three namespaces (`teranode1`, `teranode2`, `teranode3`), each backed by in-memory storage with disk overflow:

```
namespace teranode1 {
    replication-factor 1
    memory-size 512M
    storage-engine memory
}
namespace teranode2 { ... }
namespace teranode3 { ... }
```

Each Teranode connects with `aerospike_namespace = teranodeN` matching its `PORT_PREFIX`.

### 6.2 Postgres

Initialised by `compose/postgres/init.sql`:

```sql
CREATE DATABASE teranode1;
CREATE DATABASE teranode2;
CREATE DATABASE teranode3;
CREATE USER teranode WITH PASSWORD 'teranode';
GRANT ALL PRIVILEGES ON DATABASE teranode1 TO teranode;
GRANT ALL PRIVILEGES ON DATABASE teranode2 TO teranode;
GRANT ALL PRIVILEGES ON DATABASE teranode3 TO teranode;
```

Mounted as `/docker-entrypoint-initdb.d/init.sql`.

### 6.3 Kafka

Single broker plus single Zookeeper. Topics auto-create with prefix `teranode-N.*` per Teranode. Auto-creation simplifies the compose; production would predeclare topics.

`KAFKA_AUTO_CREATE_TOPICS_ENABLE=true` in env.

## 7. Bootstrap flow (`compose/bootstrap.sh`)

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> Waiting for SV node 1 RPC"
until curl -fsS -u bitcoin:bitcoin --data '{"method":"getblockchaininfo","params":[],"id":1}' \
       http://localhost:18332/ | grep -q '"result"'; do
    sleep 2
done

echo "==> Waiting for Teranode 1 health"
until curl -fsS http://localhost:18000/health/readiness | grep -q '"status"'; do
    sleep 2
done

echo "==> Generating mining address"
ADDR=$(curl -fsS -u bitcoin:bitcoin -d '{"method":"getnewaddress","id":1}' http://localhost:18332/ \
       | jq -r '.result')
echo "    miner address: $ADDR"

echo "==> Mining 110 blocks (past coinbase maturity 100)"
curl -fsS -u bitcoin:bitcoin \
     -d "{\"method\":\"generatetoaddress\",\"params\":[110, \"$ADDR\"],\"id\":1}" \
     http://localhost:18332/ > /dev/null

echo "==> Verifying chain tip propagated to Teranode-1"
TIP_TERANODE=$(curl -fsS -u bitcoin:bitcoin \
               -d '{"method":"getbestblockhash","id":1}' \
               http://localhost:19292/ | jq -r '.result')
TIP_SVNODE=$(curl -fsS -u bitcoin:bitcoin \
             -d '{"method":"getbestblockhash","id":1}' \
             http://localhost:18332/ | jq -r '.result')
if [ "$TIP_TERANODE" != "$TIP_SVNODE" ]; then
    echo "    WARN: Teranode-1 tip ($TIP_TERANODE) does not match SV node-1 tip ($TIP_SVNODE)"
    echo "    Sleeping 30s for propagation..."
    sleep 30
fi

echo "==> Funding test address (10 BSV)"
TEST_ADDR=$(grep -A1 'funding:' ../config.docker.yaml | grep address | awk '{print $2}' | tr -d '"')
if [ -n "$TEST_ADDR" ]; then
    TXID=$(curl -fsS -u bitcoin:bitcoin \
           -d "{\"method\":\"sendtoaddress\",\"params\":[\"$TEST_ADDR\", 10.0],\"id\":1}" \
           http://localhost:18332/ | jq -r '.result')
    echo "    funded $TEST_ADDR via tx $TXID"
fi

echo "==> Bootstrap complete."
```

Bootstrap runs on the host (not inside a container); it shells out to `curl` against the host-mapped ports. Idempotent — re-running after compose-up is safe (mining 110 more blocks is fine in regtest).

## 8. `config.docker.yaml`

```yaml
network: regtest

teranode:
  rpc_url: "http://localhost:19292"
  rpc_user: "bitcoin"
  rpc_pass: "bitcoin"
  rest_url: "http://localhost:18090/api/v1"
  notification_url: "ws://localhost:18090/connection/websocket"
  p2p_legacy_address: "localhost:18444"
  p2p_address: "localhost:19905"
  metrics_url: "http://localhost:19091/metrics"
  health_url: "http://localhost:18000"

svnode:
  rpc_url: "http://localhost:18332"
  rpc_user: "bitcoin"
  rpc_pass: "bitcoin"
  zmq_block_url: "tcp://localhost:28332"
  zmq_tx_url: "tcp://localhost:28333"

funding:
  # WIF for the test wallet — funded by bootstrap.sh.
  wif: "cVjzvdHGfQDtBT2YjMxnAmfgYqf6XwHsLY1xBUtJqDk9pKr8gNRk"  # regtest, deterministic
  min_balance_satoshis: 100000000

durations:
  pc1_observation: 30m
  inter1_observation: 1h
  perf1_per_rate: 30s
  default_propagation: 10s
  client1_observation: 5m
  new_nfr7_iterations: 50

limits:
  perf1_max_tps: 100
  inter2_tx_count: 100
  client3_tx_count: 50
  fr7_chain_depth: 25
  fr10_latency_target_ms: 100
  fr8_priority_levels:
    - economy
    - standard
    - priority
```

The default suite points at `teranode-1` and `svnode-1` (host ports). Tests that need mixed-network behaviour (INTER-1, INTER-2) will know to also poll `localhost:29292` etc. via additional config sections — deferred to SP5 if/when needed.

## 9. Makefile additions

```makefile
.PHONY: compose-up compose-down compose-logs compose-test compose-reset

COMPOSE := docker compose -f compose/docker-compose.yml

compose-up:
	$(COMPOSE) up -d
	@echo "Waiting 10s for services to settle..."
	@sleep 10
	@./compose/bootstrap.sh

compose-down:
	$(COMPOSE) down -v

compose-logs:
	$(COMPOSE) logs -f $${SERVICE:-teranode-1}

compose-test: compose-up
	./bin/teranode-acceptance --short --config config.docker.yaml

compose-reset: compose-down
```

`compose-up` brings the stack up and runs bootstrap. `compose-down -v` removes volumes. `compose-test` is the user's typical entry point for an end-to-end smoke run.

## 10. Verification & testing strategy

SP4-DOCKER doesn't add Go code; verification is operational.

### 10.1 SP4-DOCKER done-check (`scripts/sp4-docker-done-check.sh`)

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> compose files exist"
test -f compose/docker-compose.yml
test -f compose/teranode/settings.docker.conf
test -f compose/svnode/bitcoin.conf
test -f compose/aerospike/aerospike.conf
test -f compose/postgres/init.sql
test -x compose/bootstrap.sh
test -f config.docker.yaml

echo "==> docker compose config validates"
docker compose -f compose/docker-compose.yml config --quiet

echo "==> stack starts"
make compose-up

echo "==> verify chain tip is non-zero on all 6 nodes"
for port in 18332 28332 38332 19292 29292 39292; do
    HASH=$(curl -fsS -u bitcoin:bitcoin \
           -d '{"method":"getbestblockhash","id":1}' \
           http://localhost:$port/ | jq -r '.result')
    [ -n "$HASH" ] && [ "$HASH" != "null" ] || { echo "FAIL: node on port $port has no tip"; exit 1; }
    echo "    port $port tip: $HASH"
done

echo "==> verify all tips agree"
TIPS=$(for port in 18332 28332 38332 19292 29292 39292; do
    curl -fsS -u bitcoin:bitcoin -d '{"method":"getbestblockhash","id":1}' \
        http://localhost:$port/ | jq -r '.result'
done | sort -u | wc -l)
[ "$TIPS" -eq 1 ] || { echo "FAIL: nodes disagree on chain tip"; exit 1; }

echo "==> run a smoke acceptance run"
make compose-test || true   # exit code 3 (INCOMPLETE) is expected since SP5 hasn't shipped tests

echo "==> tear down"
make compose-down

echo "==> SP4-DOCKER done-check passed."
```

### 10.2 Health gate

`compose-up` waits 10 seconds, then runs `bootstrap.sh` which polls health endpoints. If any node fails to come up within ~60 seconds, bootstrap exits non-zero and `make compose-up` fails. Operator can `make compose-logs SERVICE=teranode-1` to debug.

## 11. Definition of done

- `compose/docker-compose.yml` brings up all 12 services on a single bridge.
- `make compose-up` exits 0; bootstrap mines 110 blocks; the wallet is funded.
- All 6 nodes report the same chain tip after bootstrap.
- `make compose-test` runs the suite against `config.docker.yaml` and produces a report (exit code 3 INCOMPLETE expected pre-SP5; later sub-projects flip it to 0/2 as tests land).
- `make compose-down` removes all containers and volumes.
- `scripts/sp4-docker-done-check.sh` exits 0.
- `docs/compose.md` documents prerequisites (Docker Engine ≥24, Compose V2, ~8 GB RAM, ~30 GB disk) and the host port table.
- Code review (`superpowers:code-reviewer`) approves.

## 12. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2` may have been built from a commit different to the SP2 discovery SHA (`11f5fa6a8...`) — endpoints could differ | SP4-DOCKER's done-check runs `getbestblockhash` against all nodes as a smoke gate; deeper drift surfaces in SP5+ tests. Document the version comparison in `docs/compose.md`. |
| B | SV node `addnode=teranode-N:18444` only works if Teranode's legacy listener actually speaks BSV-wire on that port in regtest | Verified by SP2 discovery (`go-chaincfg/params.go:451` regtest `DefaultPort: "18444"`); fallback: bootstrap script can dial `nc` to confirm the port answers before mining |
| C | Genesis + Chronicle activation from block 1 may reject the regtest genesis block itself | Both flags activate at the *target* block; the regtest genesis is at height 0, so block 1 and onwards play under the new rules. If this is wrong in practice, Teranode logs surface the rejection on startup |
| D | Multiple containers binding the same internal port (legacy 18444) within one Docker network | Each container has its own loopback; Docker DNS resolves by service name, not port. Confirmed by `../teranode/compose/` which does the same |
| E | Aerospike single-instance with 3 namespaces consumes ~1.5 GB memory by default | Memory size set to 512 MB per namespace = 1.5 GB total; documented in `docs/compose.md` |
| F | Kafka topic auto-creation may race with first publishes | `KAFKA_AUTO_CREATE_TOPICS_ENABLE=true` plus a 10s settle delay in `compose-up`; if races persist, add `kafka-init.sh` to predeclare topics |
| G | Pre-existing port collisions on the host (5432, 9092, 18332 are common) | Use `127.0.0.1:NNNN:NNNN` mappings throughout so the binding is explicit; document in `compose.md`; suggest `lsof -i` if startup fails |
| H | `bootstrap.sh` requires `jq` and `curl` on the host | Documented prerequisite; check at top of script and exit cleanly if missing |
| I | The fixture WIF in `config.docker.yaml` is a known-public test key | Documented in line and in `compose.md`; never use for mainnet |

## 13. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Single network, 3 Teranodes + 3 SV nodes | per user |
| 2 | Teranode image `ghcr.io/bsv-blockchain/teranode:v0.15.0-beta-2` | per user (Q1) |
| 3 | Shared infra — one Aerospike (3 NS), one Postgres (3 DB), one Kafka | per user (Q2) with proper data separation |
| 4 | svnode-1 wallet enabled; svnode-2/3 wallet disabled | per user (Q3) |
| 5 | Bootstrap mines 110 blocks via `generatetoaddress` against svnode-1; tests mine on demand thereafter | per user (Q4) |
| 6 | Genesis + Chronicle activation forced to block 1 | per user (Q4) |
| 7 | Ephemeral volumes; `compose-down -v` wipes everything | per user (Q5) |
| 8 | Port-prefix convention from `../teranode/compose/`: 1xxxx, 2xxxx, 3xxxx | drafter — mirrors upstream |
| 9 | Bootstrap script runs on host, shells out via curl + jq | drafter — simpler than putting bootstrap in a sidecar container |
| 10 | `config.docker.yaml` points at teranode-1 + svnode-1 by default; mixed-network endpoints come from per-test config in SP5 | drafter |
| 11 | Compose stack targets one host (no swarm / k8s) | drafter, out of scope |
| 12 | No CI integration in SP4-DOCKER; operator runs locally | drafter, deferred to SP10 |

## 14. Out-of-scope reminders

This sub-project produces an environment, not test logic. SP5+ tests then point their `--config config.docker.yaml` at the running stack. The acceptance suite itself does not orchestrate compose lifecycle — that's the operator's job via `make compose-up/down`.

The Teranode v0.15.0-beta-2 image's exact build SHA may differ from the SP2-discovery commit (`11f5fa6a8...`). If specific endpoint behaviour drifts (e.g. an RPC method renamed between versions), tests will surface it in SP5+. Document the version mismatch in `docs/compose.md`'s "version note" section.
