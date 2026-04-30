#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP6 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh

echo "==> tests/ + internal/txgen/ build and unit tests pass"
go test -race ./tests/... ./internal/txgen/...

echo "==> register.go registers tests"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP7_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only NEW-FR7,NEW-NFR7,INTER-2 || true
    test -s report.json
    for id in NEW-FR7 NEW-NFR7 INTER-2; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi
echo "==> SP7 done-check passed."
