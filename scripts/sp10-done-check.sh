#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> All prior done-checks present + executable + exit 0"
for sp in 1 2 3 4 4-docker 5 6 7 8 9; do
    script="./scripts/sp${sp}-done-check.sh"
    [ -x "$script" ] || { echo "FAIL: $script missing or not executable"; exit 1; }
done
./scripts/sp1-done-check.sh
./scripts/sp2-done-check.sh
./scripts/sp3-done-check.sh
./scripts/sp4-done-check.sh
SP4DOCKER_SKIP_LIVE=1 ./scripts/sp4-docker-done-check.sh
./scripts/sp5-done-check.sh
./scripts/sp6-done-check.sh
./scripts/sp7-done-check.sh
./scripts/sp8-done-check.sh
./scripts/sp9-done-check.sh

echo "==> Test doc-comment audit"
go run ./scripts/check-test-docs.go --tests-dir tests/

echo "==> Coverage thresholds (>=70%)"
for pkg in config internal/matrix internal/overrides internal/jsonrpc \
           internal/teranode internal/svnode internal/compare \
           internal/txgen internal/testrunner internal/observer; do
    pct=$(go test -race -cover ./$pkg/... 2>&1 | grep -oE 'coverage: [0-9.]+%' | grep -oE '[0-9.]+' | head -1)
    if [ -z "$pct" ]; then continue; fi
    awk -v p="$pct" -v t=70 'BEGIN { if (p+0 < t+0) exit 1 }' \
        || { echo "FAIL: $pkg coverage $pct% < 70%"; exit 1; }
    printf "    %-30s %s%%\n" "$pkg" "$pct"
done

echo "==> Build-doc §13 mechanical checks"
make verify
if grep -q "github.com/bsv-blockchain/teranode " go.sum 2>/dev/null; then
    echo "FAIL: bsv-blockchain/teranode is a dependency"; exit 1
fi

echo "==> Documentation present"
test -s README.md
test -s docs/operator-guide.md
test -s docs/verdict-interpretation.md
test -s docs/operator-guide-overrides-example.yaml

if [ "${SP10_LIVE:-0}" = "1" ]; then
    echo "==> Live: full --short run"
    make compose-up
    overrides_arg=""
    if [ -n "${SP10_OVERRIDES:-}" ]; then
        overrides_arg="--reviewer-overrides ${SP10_OVERRIDES}"
        echo "    using overrides: ${SP10_OVERRIDES}"
    else
        echo "    no overrides supplied (set SP10_OVERRIDES=path); verdict will likely be INCOMPLETE"
    fi
    ./bin/teranode-acceptance --short --test-timeout 90m \
        --config config.docker.yaml \
        $overrides_arg || true
    test -s report.json
    test -s report.html
    decision=$(jq -r '.verdict.decision' report.json)
    echo "==> Live verdict: $decision"
    make compose-down
fi
echo "==> SP10 done-check passed."
