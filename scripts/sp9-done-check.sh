#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP8 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh
./scripts/sp7-done-check.sh
./scripts/sp8-done-check.sh

echo "==> internal/observer + tests build pass"
go test -race ./internal/observer/... ./tests/...

echo "==> register.go has 19 tests"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP9_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --test-timeout 90m \
        --config config.docker.yaml \
        --only PC-1,INTER-1,PERF-1 || true
    test -s report.json
    for id in PC-1 INTER-1 PERF-1; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
    done
    make compose-down
fi
echo "==> SP9 done-check passed."
