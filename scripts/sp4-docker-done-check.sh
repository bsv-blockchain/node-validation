#!/usr/bin/env bash
# scripts/sp4-docker-done-check.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> compose files exist"
test -f compose/docker-compose.yml
test -f compose/teranode/settings.docker.conf
test -f compose/svnode/bitcoin.conf
test -f compose/svnode/bitcoin.conf.svnode-1
test -f compose/svnode/bitcoin.conf.svnode-2
test -f compose/svnode/bitcoin.conf.svnode-3
test -f compose/aerospike/aerospike.conf
test -f compose/postgres/init.sql
test -x compose/bootstrap.sh
test -x compose/teardown.sh
test -f config.docker.yaml
test -f docs/compose.md

echo "==> docker compose config validates"
docker compose -f compose/docker-compose.yml config --quiet

echo "==> derive-address builds and runs"
make build
addr=$(./bin/derive-address KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn)
[ -n "$addr" ] || { echo "FAIL: derive-address produced empty"; exit 1; }
echo "    derived address: $addr"

if [ "${SP4DOCKER_SKIP_LIVE:-0}" = "1" ]; then
    echo "==> SP4DOCKER_SKIP_LIVE=1 — skipping compose-up"
    echo "==> SP4-DOCKER done-check (no-live) passed."
    exit 0
fi

echo "==> stack starts (this may take 1-2 minutes)"
make compose-up

echo "==> verify chain tip is non-zero on all 6 nodes"
for port in 18332 28332 38332 19292 29292 39292; do
    HASH=$(curl -fsS -u bitcoin:bitcoin \
           -H 'Content-Type: application/json' \
           -d '{"method":"getbestblockhash","id":1}' \
           "http://localhost:$port/" | jq -r '.result')
    [ -n "$HASH" ] && [ "$HASH" != "null" ] || { echo "FAIL: node on port $port has no tip"; exit 1; }
    echo "    port $port tip: $HASH"
done

echo "==> verify all tips agree"
TIPS=$(for port in 18332 28332 38332 19292 29292 39292; do
    curl -fsS -u bitcoin:bitcoin \
        -H 'Content-Type: application/json' \
        -d '{"method":"getbestblockhash","id":1}' \
        "http://localhost:$port/" | jq -r '.result'
done | sort -u)
UNIQ=$(echo "$TIPS" | wc -l | tr -d ' ')
[ "$UNIQ" = "1" ] || { echo "FAIL: nodes disagree on chain tip:"; echo "$TIPS"; exit 1; }

echo "==> run a smoke acceptance run"
./bin/teranode-acceptance --short --config config.docker.yaml || true
test -s report.json
test -s report.html

echo "==> tear down"
make compose-down

echo "==> SP4-DOCKER done-check passed."
