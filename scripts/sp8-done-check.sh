#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1-SP7 done-checks"
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh
./scripts/sp7-done-check.sh

echo "==> Fixture invariants"
test -s tests/testdata/historical_scripts.yaml
test -s tests/testdata/historical_utxos.yaml
pc2_count=$(grep -c '^- id:' tests/testdata/historical_scripts.yaml)
ibd2_count=$(grep -c '^- id:' tests/testdata/historical_utxos.yaml)
[ "$pc2_count" -ge 30 ] || { echo "FAIL: PC-2 fixtures=$pc2_count, want >=30"; exit 1; }
[ "$ibd2_count" -ge 10 ] || { echo "FAIL: IBD-2 fixtures=$ibd2_count, want >=10"; exit 1; }

echo "==> Fixture generator deterministic (no diff after re-run)"
./bin/gen-fixtures --out tests/testdata/
git diff --exit-code tests/testdata/historical_scripts.yaml tests/testdata/historical_utxos.yaml

echo "==> tests/ + cmd/ build and unit tests pass"
go test -race ./tests/... ./cmd/gen-fixtures/...

echo "==> register.go registers 16 tests"
go test -race ./cmd/teranode-acceptance/... -run '^TestRegisterTests_'

if [ "${SP8_LIVE:-0}" = "1" ]; then
    make compose-up
    ./bin/teranode-acceptance --short --config config.docker.yaml \
        --only CLIENT-1,CLIENT-3,PC-2,IBD-2 || true
    test -s report.json
    for id in CLIENT-1 CLIENT-3 PC-2 IBD-2; do
        status=$(jq -r ".test_cases[] | select(.id == \"$id\") | .result.status" report.json)
        if [ "$status" = "NOT_RUN" ] || [ -z "$status" ] || [ "$status" = "null" ]; then
            echo "FAIL: $id status=$status"; exit 1
        fi
        echo "    $id: $status"
    done
    make compose-down
fi
echo "==> SP8 done-check passed."
