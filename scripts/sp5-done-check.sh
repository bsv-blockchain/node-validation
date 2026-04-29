#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP4 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh

echo "==> tests/ package builds and unit tests pass"
go test -race ./tests/...

echo "==> register.go registers tests (any TestRegisterTests_* must pass)"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP5_LIVE:-0}" = "1" ]; then
    echo "==> live: bringing up compose stack"
    make compose-up
    echo "==> running the 4 SP5 tests against live stack"
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only NEW-NFR11,NEW-NFR13,OPS-3,PC-3 || true
    test -s report.json
    for id in NEW-NFR11 NEW-NFR13 OPS-3 PC-3; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status (expected non-NOT_RUN)"
            exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi

echo "==> SP5 done-check passed."
