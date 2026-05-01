#!/usr/bin/env bash
# compose/bootstrap.sh
# Mines the initial regtest chain and funds the test wallet.
set -euo pipefail

cd "$(dirname "$0")/.."   # project root

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "FAIL: missing prerequisite: $1"
        exit 1
    fi
}
require curl
require jq

rpc_call() {
    local port="$1" method="$2" params="$3"
    curl -fsS -u bitcoin:bitcoin \
        -H 'Content-Type: application/json' \
        -d "{\"method\":\"$method\",\"params\":$params,\"id\":1}" \
        "http://localhost:$port/"
}

wait_for() {
    local label="$1" port="$2" method="$3" timeout="${4:-90}"
    echo "==> waiting for $label (port $port, method $method)"
    for i in $(seq 1 "$timeout"); do
        if rpc_call "$port" "$method" "[]" 2>/dev/null | grep -q '"result"'; then
            return 0
        fi
        sleep 1
    done
    echo "FAIL: $label did not become ready within ${timeout}s"
    return 1
}

wait_for "svnode-1" 18332 "getblockchaininfo"
wait_for "teranode-1 RPC" 19292 "getblockchaininfo" 120
wait_for "teranode-2 RPC" 29292 "getblockchaininfo" 30
wait_for "teranode-3 RPC" 39292 "getblockchaininfo" 30

# Kick the Teranode FSM out of IDLE state on each node.
#
# Problem: legacy.(*Server).Start() blocks on WaitUntilFSMTransitionFromIdleState()
# which waits for a message on the blocks-final Kafka topic.  On a fresh chain that
# topic is empty (60 s retention) so the legacy P2P layer never starts, no blocks
# can arrive, and the FSM never leaves IDLE — a deadlock.
#
# Fix: send FSMEventType=RUN (value 1) to the blockchain gRPC API on each node
# immediately after the RPC port is up.  This advances the FSM to RUNNING so
# legacy.Server.Start() unblocks and P2P connections can be established.
echo "==> unblocking Teranode FSM on all three nodes (RUN event)"
for ctr in node-validation-teranode-1-1 node-validation-teranode-2-1 node-validation-teranode-3-1; do
    # If FSM is already in RUNNING/CATCHINGBLOCKS, SendFSMEvent returns an error.
    # In that case query the current state instead.
    out=$(docker exec "$ctr" grpcurl -plaintext \
        -d '{"event": 1}' localhost:8087 \
        blockchain_api.BlockchainAPI.SendFSMEvent 2>&1) || true
    state=$(echo "$out" | grep -o '"state":"[^"]*"' || \
            docker exec "$ctr" grpcurl -plaintext localhost:8087 \
                blockchain_api.BlockchainAPI.GetFSMCurrentState 2>/dev/null | grep -o '"state":"[^"]*"' || \
            echo '"state":"unknown"')
    echo "    $ctr: $state"
done

echo "==> generating mining address (svnode-1 wallet)"
ADDR=$(rpc_call 18332 "getnewaddress" '[]' | jq -r '.result')
[ -n "$ADDR" ] || { echo "FAIL: no address from svnode-1"; exit 1; }
echo "    miner address: $ADDR"

echo "==> mining 110 blocks"
rpc_call 18332 "generatetoaddress" "[110, \"$ADDR\"]" >/dev/null
COUNT=$(rpc_call 18332 "getblockcount" '[]' | jq -r '.result')
echo "    svnode-1 height: $COUNT"

echo "==> waiting up to 60s for the mesh to converge on the same tip"
for i in $(seq 1 30); do
    TIPS=$(for p in 18332 28332 38332 19292 29292 39292; do
        rpc_call "$p" "getbestblockhash" '[]' 2>/dev/null | jq -r '.result' || echo "ERR"
    done | sort -u)
    UNIQ=$(echo "$TIPS" | wc -l | tr -d ' ')
    if [ "$UNIQ" = "1" ] && ! echo "$TIPS" | grep -q ERR; then
        echo "    converged tip: $TIPS"
        break
    fi
    sleep 2
done
if [ "$UNIQ" != "1" ] || echo "$TIPS" | grep -q ERR; then
    echo "WARN: mesh did not converge within 60s. Tips:"
    echo "$TIPS"
    echo "    Continuing — propagation may complete after bootstrap."
fi

echo "==> deriving funding address from config.docker.yaml WIF"
if [ ! -x bin/derive-address ]; then
    echo "FAIL: bin/derive-address not built. Run 'make build' first."
    exit 1
fi
WIF=$(awk '/^[[:space:]]*wif:/ {
    gsub(/^[[:space:]]*wif:[[:space:]]*"?|"?[[:space:]]*$/, "")
    print
    exit
}' config.docker.yaml)
[ -n "$WIF" ] || { echo "FAIL: WIF not found in config.docker.yaml"; exit 1; }
TEST_ADDR=$(./bin/derive-address "$WIF")
[ -n "$TEST_ADDR" ] || { echo "FAIL: derive-address produced empty result"; exit 1; }
echo "    test wallet address: $TEST_ADDR"

echo "==> funding test wallet (10 BSV)"
TXID=$(rpc_call 18332 "sendtoaddress" "[\"$TEST_ADDR\", 10.0]" | jq -r '.result')
echo "    funding txid: $TXID"

echo "==> mining 1 confirmation block"
rpc_call 18332 "generatetoaddress" "[1, \"$ADDR\"]" >/dev/null

echo "==> Bootstrap complete."
echo "    svnode-1 RPC:    http://localhost:18332"
echo "    teranode-1 RPC:  http://localhost:19292"
echo "    teranode-1 REST: http://localhost:18090/api/v1"
echo "    test address:    $TEST_ADDR"
echo "    funding txid:    $TXID"
