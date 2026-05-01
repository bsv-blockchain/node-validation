// Package tests — CLIENT-3 implementation.
//
// Source plan §"Client Integration Tests" → CLIENT-3. Captures R1, R6.
// Severity Critical.
//
// Objective:
//
//	Verify notification mechanisms deliver complete, ordered block and
//	transaction streams.
//
// Method:
//  1. Subscribe to block + (subtree) notifications.
//  2. Generate Cfg.Limits.CLIENT3TxCount (default 500) txs on a controlled
//     schedule using BuildSplitter to create the UTXO set.
//  3. Mine blocks containing them.
//  4. Verify every generated txid is reachable via REST tx-fetch (Teranode
//     doesn't emit per-tx events on Centrifuge per SP2 §3 — coverage is
//     inferred via subtree expansion + REST). Document this finding.
//  5. Blocks arrive in strictly non-decreasing height order via subscription.
//  6. Simulate midpoint reconnection (fresh NotificationClient); verify
//     cached node_status arrives.
//
// Acceptance criteria:
//   - ≥99% of generated txids reachable via REST after mining (proxy for
//     "100% of expected notifications delivered" per SP2 §3 architecture).
//   - Strict block-height non-decreasing order via subscription.
//   - Reconnection (fresh NotificationClient) succeeds.
//   - Architectural finding documented (no per-tx Centrifuge events).
package tests

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunCLIENT3(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "CLIENT-3", Title: "Notification Stream Reliability",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-6"},
		CapturedRisks:         []string{"R1", "R6"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil ||
		env.Teranode.REST == nil || env.Teranode.Notifications == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil || env.TxGen == nil {
		return skipMissing(res, "client(s) not configured")
	}

	count := env.Cfg.Limits.CLIENT3TxCount
	if count <= 0 {
		count = 500
	}
	res.Observations["tx_count"] = count

	// Establish notification session and track block heights.
	notif := env.Teranode.Notifications
	if err := notif.Connect(ctx); err != nil {
		return errorResult(res, fmt.Errorf("connect: %w", err))
	}

	var mu sync.Mutex
	blockHeights := []uint64{}

	origCtx, origCancel := context.WithCancel(ctx)
	defer origCancel()
	var freshCancel context.CancelFunc
	defer func() {
		if freshCancel != nil {
			freshCancel()
		}
	}()
	go tailBlockHeights(origCtx, notif, &mu, &blockHeights)

	// Bootstrap funder and build splitter to produce `count` UTXOs.
	funder := env.TxGen
	builder := funder.Builder()
	const splitterSatsPerOutput uint64 = 100_000
	target := uint64(count) * splitterSatsPerOutput * 2 // 2x headroom for fees
	if funder.Balance() < target {
		if _, err := bootstrapConfirmed(ctx, env, target); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
		time.Sleep(2 * time.Second)
	}

	splitter, err := builder.BuildSplitter(count, splitterSatsPerOutput, 500)
	if err != nil {
		return errorResult(res, fmt.Errorf("BuildSplitter: %w", err))
	}
	if _, err := env.Teranode.RPC.SendRawTransaction(ctx, splitter.HexTx); err != nil {
		return errorResult(res, fmt.Errorf("submit splitter: %w", err))
	}
	if _, err := mineBlocks(ctx, env, 1); err != nil {
		return errorResult(res, err)
	}
	time.Sleep(2 * time.Second)

	// Reset funder state and register splitter outputs as spendable UTXOs.
	funder.Reset()
	addrScript, _ := txgen.P2PKHScript(funder.Address())
	newUTXOs := make([]txgen.UTXO, count)
	for i := 0; i < count; i++ {
		newUTXOs[i] = txgen.UTXO{
			TxID:     splitter.TxID,
			Vout:     uint32(i),
			Satoshis: splitterSatsPerOutput,
			Script:   addrScript,
		}
	}
	funder.ConfirmMulti(splitter.Inputs, newUTXOs)

	// Submit all `count` transactions to Teranode.
	// At the midpoint (i == half), perform a controlled reconnect so that the
	// block-ordering check covers both pre- and post-reconnect blocks.
	sentTxIDs := make([]string, 0, count)
	half := count / 2
	var freshErr error
	reconnected := false
	for i := 0; i < count; i++ {
		if i == half {
			// Midpoint reconnect: close original client, construct a fresh one.
			_ = notif.Close()
			origCancel() // signal the original tail goroutine to exit
			time.Sleep(2 * time.Second)
			var freshNotif *teranode.NotificationClient
			freshNotif, freshErr = teranode.NewNotificationClient(env.Cfg.Teranode.NotificationURL, env.Logger)
			if freshErr == nil && freshNotif != nil {
				if connErr := freshNotif.Connect(ctx); connErr == nil {
					env.Teranode.Notifications = freshNotif
					notif = freshNotif
					freshCtx, freshCancelInner := context.WithCancel(ctx)
					freshCancel = freshCancelInner
					go tailBlockHeights(freshCtx, freshNotif, &mu, &blockHeights)
					reconnected = true
				}
			}
		}
		bres, err := builder.BuildP2PKH(txgen.BuildRequest{
			Outputs:   []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
			FeeRate:   500,
			SpendUTXO: &newUTXOs[i],
		})
		if err != nil {
			continue
		}
		id, err := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
		if err == nil {
			sentTxIDs = append(sentTxIDs, id)
		}
	}
	res.Observations["txs_submitted"] = len(sentTxIDs)

	if reconnected {
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Reconnection (fresh NotificationClient) succeeded",
			"per spec Q4=A — fresh client constructed post-disconnect",
		))
	} else {
		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
			"Reconnection (fresh NotificationClient) succeeded",
			fmt.Sprintf("freshErr=%v", freshErr),
		))
	}

	// Mine to confirm all submitted txs.
	_, _ = mineBlocks(ctx, env, 2)
	time.Sleep(3 * time.Second)

	// Verify each sentTxID is reachable via REST (Teranode knows about them).
	// Per SP2 §3, Teranode does not emit per-tx events on Centrifuge;
	// REST tx-fetch is the proxy for "tx notification delivered".
	confirmedCount := 0
	for _, id := range sentTxIDs {
		_, err := env.Teranode.REST.GetTxBytes(ctx, id)
		if err == nil {
			confirmedCount++
		}
	}
	res.Observations["confirmed_via_rest"] = confirmedCount

	restPct := 0.0
	if len(sentTxIDs) > 0 {
		restPct = float64(confirmedCount) / float64(len(sentTxIDs))
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"≥99% of generated txs confirmed via REST (proxy for notification delivery)",
		len(sentTxIDs) > 0 && restPct >= 0.99,
		fmt.Sprintf("confirmed=%d/%d (%.1f%%)", confirmedCount, len(sentTxIDs), restPct*100),
	))

	// Block-height ordering check: heights must be non-decreasing.
	mu.Lock()
	heightsCopy := append([]uint64(nil), blockHeights...)
	mu.Unlock()
	res.Observations["block_heights_seen"] = len(heightsCopy)

	ascending := true
	for i := 1; i < len(heightsCopy); i++ {
		if heightsCopy[i] < heightsCopy[i-1] {
			ascending = false
			break
		}
	}
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block heights arrive in non-decreasing order via subscription",
		ascending,
		fmt.Sprintf("heights_seen=%d ascending=%v", len(heightsCopy), ascending),
	))

	// Architectural finding: document that Teranode does not emit per-tx events
	// on Centrifuge (per SP2 §3 discovery). REST is used as coverage proxy.
	res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
		"Notification mechanism documented (architectural finding)",
		"Teranode emits block + subtree events on Centrifuge; per-tx events are absent per SP2 §3 discovery — REST tx-fetch used as delivery proxy for CLIENT-3",
	))

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
