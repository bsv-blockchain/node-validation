#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

fail() { echo "FAIL: $*"; exit 1; }

echo "==> 1. Checking all 11 surface section headings in discovery.md"
SURFACES=(
  "JSON-RPC service"
  "REST / Asset HTTP API"
  "Notifications"
  "P2P listener"
  "Metrics endpoint"
  "Health endpoint"
  "Extended transaction format"
  "testmempoolaccept"
  "Fee estimation"
  "Mempool query"
  "Double-spend"
)
for surface in "${SURFACES[@]}"; do
  grep -qi "^## .*${surface}" docs/discovery.md \
    || fail "Missing section heading for surface: ${surface}"
done
echo "    all 11 surface headings found"

echo "==> 2. Checking frontmatter records upstream_commit"
grep -q '^upstream_commit: "11f5fa6a81c36490e2796561f76a39294fc422b5"' docs/discovery.md \
  || fail "discovery.md frontmatter missing or wrong upstream_commit"
echo "    upstream_commit present"

echo "==> 3. Running check-refs"
go run ./scripts/check-refs.go \
  --discovery docs/discovery.md \
  --yaml docs/discovery.yaml \
  --upstream /Users/oskarsson/gitcheckout/teranode
echo "    check-refs passed"

echo "==> 4. SP1 still green (make build lint test)"
make build lint test
echo "    make build lint test passed"

echo ""
echo "SP2 done-check passed."
