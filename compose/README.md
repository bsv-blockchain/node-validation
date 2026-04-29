# Compose stack

Operator-facing docs live in `docs/compose.md`.

This directory contains all configuration files used by `docker-compose.yml`.
Files committed:
- `teranode/settings.docker.conf` — Teranode regtest overrides
- `svnode/bitcoin.conf` (+ per-node tweaks via `bitcoin.conf.svnode-{1,2,3}`)
- `aerospike/aerospike.conf` — single server, three namespaces
- `postgres/init.sql` — three databases
- `bootstrap.sh` / `teardown.sh`
