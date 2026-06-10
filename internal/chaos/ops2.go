package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RunOPS2 — Network Partition and Reorg Convergence.
//
// Mechanism (genuine network partition via `docker network disconnect`):
//
//	The partition boundary is placed at an SV node (svnode-3). Teranodes
//	share their datastores (Kafka/Aerospike/Postgres) over tng-net and become
//	non-functional if detached from it, so they cannot be cut from the network
//	without crashing — empirically verified. SV nodes are self-contained
//	bitcoind processes that survive a network disconnect cleanly, so the
//	partition is realized there. (A Teranode-vs-Teranode partition would
//	require deploying the shared infra on a separate Docker network; deferred.)
//
//	1. Baseline: all 6 nodes converged at height H, tip T0.
//	2. Partition: detach svnode-3 (minority side B) from tng-net.
//	3. Diverge: mine 1 block on svnode-3 (side B) and 3 blocks on svnode-1
//	   (side A / majority), building competing chains from the same fork point
//	   with side A strictly longer.
//	4. Verify divergence: side A (5 nodes incl. all Teranodes) is at H+3 on
//	   chain A; svnode-3 is at H+1 on chain B; the two tips differ.
//	5. Heal: reconnect svnode-3 and restart it to force a fresh P2P handshake.
//	6. Converge: all 6 nodes reorg onto the longer chain A (height H+3); the
//	   minority's competing block is orphaned. No permanent split.
//
// Implementation note: res is a NAMED return value so the deferred
// res.derive() in cleanup sets the final Status on the value the caller
// observes. See the matching note on RunOPS1 for why a plain local would
// report a spurious ERROR.
func RunOPS2(ctx context.Context, m *Mesh, logger *slog.Logger) (res Result) {
	res = Result{ID: "OPS-2", Title: "Network Partition and Reorg Convergence", StartedAt: time.Now().UTC()}

	miner := m.Miner()    // svnode-1 (side A, has wallet)
	sideB := m.SVNodes[2] // svnode-3 (minority side B)
	sideBContainer := sideB.container

	// Side A = everything except the partitioned svnode-3.
	var sideA []Node
	for _, n := range m.AllNodes() {
		if n.Name() != sideB.Name() {
			sideA = append(sideA, n)
		}
	}

	// Self-healing cleanup: always reconnect + restart svnode-3 and reconverge.
	defer func() {
		logger.Info("OPS-2: cleanup — restoring mesh")
		if err := m.Restore(ctx, 4*time.Minute); err != nil {
			logger.Error("OPS-2: cleanup restore failed", "err", err)
			res.observe("cleanup_restore_error", err.Error())
		} else {
			logger.Info("OPS-2: cleanup — mesh restored and converged")
		}
		res.derive()
		res.Duration = time.Since(res.StartedAt)
	}()

	// 1. Baseline convergence across all 6 nodes.
	baseHash, baseHeight, baseSnap, ok := m.WaitConverged(ctx, m.AllNodes(), 2*time.Minute)
	res.observe("baseline_tips", baseSnap)
	if !ok {
		res.Status = StatusError
		res.Err = "mesh not converged at baseline; cannot run partition test"
		return res
	}
	res.observe("baseline_hash", baseHash)
	res.observe("baseline_height", baseHeight)
	logger.Info("OPS-2: baseline", "hash", baseHash, "height", baseHeight)

	// Mining address (from svnode-1's wallet; reused on wallet-less svnode-3).
	addr, err := miner.NewAddress(ctx)
	if err != nil {
		res.Status = StatusError
		res.Err = fmt.Sprintf("getnewaddress on svnode-1: %v", err)
		return res
	}

	// 2. Partition: detach svnode-3.
	logger.Info("OPS-2: partitioning — disconnecting svnode-3 from tng-net")
	if err := m.D.NetworkDisconnect(ctx, Network, sideBContainer); err != nil {
		res.Status = StatusError
		res.Err = fmt.Sprintf("network disconnect svnode-3: %v", err)
		return res
	}
	detached, _ := m.D.NetworkContains(ctx, Network, sideBContainer)
	res.check("Partition established: svnode-3 detached from tng-net", !detached,
		fmt.Sprintf("svnode-3 attached_to_tng-net=%v (want false)", detached))

	// 3a. Side B mines 1 block on its isolated chain.
	bHashes, err := sideB.Generate(ctx, 1, addr)
	if err != nil || len(bHashes) != 1 {
		res.fail("Side B (svnode-3) mined an independent block", fmt.Sprintf("err=%v hashes=%v", err, bHashes))
		return res
	}
	bTip := bHashes[0]
	res.observe("sideB_block", bTip)

	// 3b. Side A mines 3 blocks → strictly longer competing chain.
	aHashes, err := miner.Generate(ctx, 3, addr)
	if err != nil || len(aHashes) != 3 {
		res.fail("Side A (svnode-1) mined a longer competing chain", fmt.Sprintf("err=%v hashes=%v", err, aHashes))
		return res
	}
	aTip := aHashes[2]
	res.observe("sideA_tip", aTip)

	// 4. Side A converges on chain A at H+3 (all 5 nodes incl. Teranodes).
	aHash, aHeight, aSnap, aOK := m.WaitConverged(ctx, sideA, 2*time.Minute)
	res.observe("partition_sideA_tips", aSnap)
	res.check("Side A (5 nodes incl. all Teranodes) converged on the longer chain",
		aOK && aHash == aTip && aHeight == baseHeight+3,
		fmt.Sprintf("converged=%v hash=%s height=%d want_hash=%s want_height=%d", aOK, aHash, aHeight, aTip, baseHeight+3))

	// 4b. Verify svnode-3 genuinely diverged (isolated): still on chain B at H+1.
	bSnapHash, _ := sideB.BestBlockHash(ctx)
	bSnapHeight, _ := sideB.Height(ctx)
	res.observe("partition_sideB_tip", TipInfo{Hash: bSnapHash, Height: bSnapHeight})
	res.check("Side B (svnode-3) diverged: isolated on its own shorter chain",
		bSnapHash == bTip && bSnapHeight == baseHeight+1 && bSnapHash != aTip,
		fmt.Sprintf("sideB tip=%s height=%d (want %s @ %d); differs from sideA tip=%v",
			bSnapHash, bSnapHeight, bTip, baseHeight+1, bSnapHash != aTip))
	res.check("Competing chains: side A strictly longer than side B",
		aHeight > bSnapHeight,
		fmt.Sprintf("sideA_height=%d > sideB_height=%d", aHeight, bSnapHeight))

	// 5. Heal: reconnect svnode-3 (restoring its compose DNS aliases so it stays
	// resolvable by name — see Docker.NetworkConnect) and restart it to force a
	// fresh P2P handshake.
	logger.Info("OPS-2: healing — reconnecting + restarting svnode-3")
	if err := m.D.NetworkConnect(ctx, Network, sideBContainer, sideB.Name(), sideBContainer); err != nil {
		res.fail("Partition healed: svnode-3 reconnected", err.Error())
		return res
	}
	if err := m.D.Restart(ctx, sideBContainer); err != nil {
		res.fail("Partition healed: svnode-3 restarted", err.Error())
		return res
	}
	reattached, _ := m.D.NetworkContains(ctx, Network, sideBContainer)
	res.check("Partition healed: svnode-3 reconnected to tng-net", reattached,
		fmt.Sprintf("svnode-3 attached_to_tng-net=%v (want true)", reattached))

	// 6. All 6 reconverge onto the longer chain A; svnode-3 reorgs.
	finalHash, finalHeight, finalSnap, finalOK := m.WaitConverged(ctx, m.AllNodes(), 4*time.Minute)
	res.observe("final_tips", finalSnap)
	res.observe("final_hash", finalHash)
	res.observe("final_height", finalHeight)
	res.check("All 6 nodes reconverged on a single tip after heal", finalOK,
		fmt.Sprintf("converged=%v hash=%s height=%d", finalOK, finalHash, finalHeight))
	res.check("Mesh converged on the longer chain (chain A) via reorg",
		finalOK && finalHash == aTip && finalHeight == baseHeight+3,
		fmt.Sprintf("final=%s @ %d want=%s @ %d", finalHash, finalHeight, aTip, baseHeight+3))
	res.check("Minority chain reorged away (svnode-3 dropped its competing block)",
		finalOK && finalHash != bTip,
		fmt.Sprintf("final_tip=%s sideB_block=%s (must differ)", finalHash, bTip))

	return res
}
