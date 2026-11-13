package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/stretchr/testify/require"
)

func TestNEWFR10LatencyEvidence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hashes     bool
		txids      bool
		restStatus int
		wantPass   [3]bool
	}{
		{"successful probes", true, true, http.StatusOK, [3]bool{true, true, true}},
		{"no transaction sample", true, false, http.StatusOK, [3]bool{false, true, true}},
		{"no block sample", false, false, http.StatusOK, [3]bool{false, false, true}},
		{"fast failed requests", true, true, http.StatusNotFound, [3]bool{false, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(tc.restStatus)
					return
				}
				var req struct {
					Method string `json:"method"`
					ID     int64  `json:"id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				var result any
				var rpcErr any
				switch req.Method {
				case "getblockchaininfo":
					result = map[string]any{"blocks": 5}
				case "getblockhash":
					if tc.hashes {
						result = "blockhash"
					} else {
						rpcErr = map[string]any{"code": -5, "message": "missing block"}
					}
				case "getblock":
					txids := []string{}
					if tc.txids {
						txids = []string{"coinbase"}
					}
					result = map[string]any{"tx": txids}
				default:
					t.Errorf("unexpected RPC method: %s", req.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": result, "error": rpcErr})
			}))
			defer srv.Close()
			rpc, err := teranode.NewRPCClient(srv.URL, "", "", nil)
			require.NoError(t, err)
			rest, err := teranode.NewRESTClient(srv.URL, nil)
			require.NoError(t, err)
			env := &testrunner.Env{Now: time.Now, Teranode: &teranode.Clients{RPC: rpc, REST: rest}}
			env.Cfg.Limits.FR10LatencyTargetMs = 10000
			res := RunNEWFR10(context.Background(), env)
			require.GreaterOrEqual(t, len(res.AcceptanceChecks), 3)
			for i, want := range tc.wantPass {
				require.Equal(t, want, res.AcceptanceChecks[i].Pass, res.AcceptanceChecks[i].Detail)
			}
			if tc.wantPass == [3]bool{true, true, true} {
				require.Equal(t, testrunner.StatusPass, res.Status)
			} else {
				require.Equal(t, testrunner.StatusFail, res.Status)
			}
		})
	}
}

func TestHistoricalLatencyCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, check := historicalLatencyCheck(ctx, "tx", []string{"one", "two"}, time.Second, func(string) error {
		calls++
		cancel()
		return nil
	})
	require.Equal(t, 1, calls)
	require.False(t, check.Pass, check.Detail)
}

func TestHistoricalLatencyCheckAboveTarget(t *testing.T) {
	_, check := historicalLatencyCheck(context.Background(), "tx", []string{"one"}, -time.Nanosecond, func(string) error { return nil })
	require.False(t, check.Pass, check.Detail)
}
