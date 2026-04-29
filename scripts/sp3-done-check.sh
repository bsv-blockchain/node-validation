#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1 still green"
./scripts/sp1-done-check.sh

echo "==> SP2 still green"
./scripts/sp2-done-check.sh

echo "==> SP3 package coverage"
go test -race -coverprofile=cov.out \
    ./internal/teranode/... \
    ./internal/svnode/... \
    ./internal/compare/... \
    ./internal/jsonrpc/...

totals=$(go tool cover -func=cov.out | tail -1 | awk '{ print $3 }')
totalNum=${totals%\%}
threshold=80

if command -v bc >/dev/null 2>&1; then
    below=$(echo "$totalNum < $threshold" | bc -l)
else
    below=$(awk -v t="$threshold" -v v="$totalNum" 'BEGIN { print (v < t) ? 1 : 0 }')
fi

if [ "$below" -eq 1 ]; then
    echo "FAIL: total coverage $totals < ${threshold}%"
    rm -f cov.out
    exit 1
fi

rm -f cov.out

echo "SP3 done-check passed."
