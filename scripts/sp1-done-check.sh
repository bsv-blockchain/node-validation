#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "==> make build lint test verify"
make build lint test verify

echo "==> running test-short (expect exit 3 INCOMPLETE)"
make build
set +e
./bin/teranode-acceptance --short --config config.yaml
rc=$?
set -e
if [ "$rc" -ne 3 ]; then
    echo "FAIL: test-short exited $rc, expected 3 (INCOMPLETE)"
    exit 1
fi

echo "==> verifying report.json structure"
test -s report.json
test -s report.html

require_count() {
    local key="$1" want="$2"
    local got
    got=$(jq ".$key | length" report.json)
    if [ "$got" -ne "$want" ]; then
        echo "FAIL: $key has $got entries, want $want"
        exit 1
    fi
}

require_count requirements 24
require_count test_environment 3
require_count test_cases 24
require_count risks 7

decision=$(jq -r '.verdict.decision' report.json)
if [ "$decision" != "INCOMPLETE" ]; then
    echo "FAIL: verdict is $decision, want INCOMPLETE"
    exit 1
fi

echo "==> SP1 done-check passed."
