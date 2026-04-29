#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> SP1 still green"; ./scripts/sp1-done-check.sh
echo "==> SP2 still green"; ./scripts/sp2-done-check.sh
echo "==> SP3 still green"; ./scripts/sp3-done-check.sh

echo "==> SP4 package coverage"
go test -race -coverprofile=cov.out ./internal/txgen/...
total=$(go tool cover -func=cov.out | tail -1 | awk '{ print $3 }' | tr -d %)
threshold=80
awk -v t="$total" -v th="$threshold" 'BEGIN { if (t+0 < th+0) { exit 1 } }'
rm -f cov.out

echo "==> SP4 done-check passed."
