package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RunOPS1 — Service Failure and Recovery (NODE-CRASH recovery).
//
// Scope note: this compose topology runs Teranode as a single all-in-one
// process (settings.docker.conf), so there are no separable microservices to
// fault-inject individually. This test therefore covers whole-node crash and
// recovery — `docker kill` then `docker start` of a Teranode — NOT
// per-microservice fault isolation (which needs the upstream split topology
// and is deferred upstream).
//
// Mechanism:
//
//  1. Baseline: all 6 nodes converged at height H, tip T0.
//  2. Crash: `docker kill` teranode-1.
//  3. Assert the node is down (RPC unreachable).
//  4. Assert the rest of the mesh keeps operating: mine 2 blocks on svnode-1;
//     the 5 surviving nodes advance to H+2 and stay converged.
//  5. Recover: `docker start` teranode-1, wait for RPC, nudge its FSM to RUN.
//  6. Assert teranode-1 recovers and re-converges to the mesh tip (H+2).
//
// Implementation note: res is a NAMED return value. The deferred cleanup calls
// res.derive() to set the final Status from the accumulated checks; that only
// affects the value the caller observes if res is the named return (a plain
// local would already have been copied into the return slot by `return res`
// before the defer runs, leaving Status="" → a spurious ERROR verdict).
func RunOPS1(ctx context.Context, m *Mesh, logger *slog.Logger) (res Result) {
	res = Result{ID: "OPS-1", Title: "Service Failure and Recovery", StartedAt: time.Now().UTC()}

	miner := m.Miner()       // svnode-1
	victim := m.Teranodes[0] // teranode-1

	// Survivors = everything except teranode-1.
	var survivors []Node
	for _, n := range m.AllNodes() {
		if n.Name() != victim.Name() {
			survivors = append(survivors, n)
		}
	}

	// Self-healing cleanup: always bring teranode-1 back and reconverge.
	// A killed all-in-one Teranode is slow to fully restart, re-handshake P2P
	// and re-sync, so give the restore generous headroom.
	defer func() {
		logger.Info("OPS-1: cleanup — restoring mesh")
		if err := m.Restore(ctx, 8*time.Minute); err != nil {
			logger.Error("OPS-1: cleanup restore failed", "err", err)
			res.observe("cleanup_restore_error", err.Error())
		} else {
			logger.Info("OPS-1: cleanup — mesh restored and converged")
		}
		res.derive()
		res.Duration = time.Since(res.StartedAt)
	}()

	// 1. Baseline convergence.
	baseHash, baseHeight, baseSnap, ok := m.WaitConverged(ctx, m.AllNodes(), 2*time.Minute)
	res.observe("baseline_tips", baseSnap)
	if !ok {
		res.Status = StatusError
		res.Err = "mesh not converged at baseline; cannot run failure test"
		return res
	}
	res.observe("baseline_hash", baseHash)
	res.observe("baseline_height", baseHeight)
	logger.Info("OPS-1: baseline", "hash", baseHash, "height", baseHeight)

	// 2. Crash teranode-1.
	logger.Info("OPS-1: crashing teranode-1 (docker kill)")
	if err := m.D.Kill(ctx, victim.container); err != nil {
		res.Status = StatusError
		res.Err = fmt.Sprintf("docker kill teranode-1: %v", err)
		return res
	}

	// 3. Confirm it is down.
	down := false
	for i := 0; i < 5; i++ {
		if _, err := victim.BestBlockHash(ctx); err != nil {
			down = true
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	running, _ := m.D.Running(ctx, victim.container)
	res.check("Crashed node is down (teranode-1 not running, RPC unreachable)",
		down && !running, fmt.Sprintf("rpc_down=%v container_running=%v", down, running))

	// 4. Mesh keeps operating: mine 2 blocks; survivors advance + converge.
	addr, err := miner.NewAddress(ctx)
	if err != nil {
		res.Status = StatusError
		res.Err = fmt.Sprintf("getnewaddress on svnode-1: %v", err)
		return res
	}
	mined, err := miner.Generate(ctx, 2, addr)
	if err != nil || len(mined) != 2 {
		res.fail("Surviving mesh mined new blocks while teranode-1 was down",
			fmt.Sprintf("err=%v hashes=%v", err, mined))
		return res
	}
	wantTip := mined[1]
	survHash, survHeight, survSnap, survOK := m.WaitConverged(ctx, survivors, 2*time.Minute)
	res.observe("survivors_tips", survSnap)
	res.check("Surviving mesh kept operating: 5 nodes advanced and converged while node down",
		survOK && survHash == wantTip && survHeight == baseHeight+2,
		fmt.Sprintf("converged=%v hash=%s height=%d want_hash=%s want_height=%d",
			survOK, survHash, survHeight, wantTip, baseHeight+2))

	// 5. Recover teranode-1. RecoverTeranode does the reliable procedure:
	// docker start (the common path: the all-in-one restores its FSM from
	// postgres to RUNNING and serves RPC within seconds), with a docker-restart
	// escalation if a cold start wedges on DHT bootstrap, then an FSM nudge.
	// The generous budget is retained as headroom for the rare wedge.
	logger.Info("OPS-1: recovering teranode-1 (docker start, restart-on-wedge fallback)")
	if err := m.RecoverTeranode(ctx, victim, 6*time.Minute); err != nil {
		res.fail("Recovered node RPC came back up", err.Error())
		return res
	}
	res.pass("Crashed node restarted and RPC recovered", "teranode-1 RPC responsive after recovery")

	// 6. Recovered node re-converges to the mesh tip.
	if err := m.WaitNodeTip(ctx, victim, wantTip, 6*time.Minute); err != nil {
		res.fail("Recovered node re-converged to the mesh tip", err.Error())
		return res
	}
	recoveredHeight, _ := victim.Height(ctx)
	res.observe("recovered_tip", wantTip)
	res.observe("recovered_height", recoveredHeight)
	res.check("Recovered node re-converged to the mesh tip (re-synced missed blocks)",
		recoveredHeight == baseHeight+2,
		fmt.Sprintf("teranode-1 height=%d want=%d tip=%s", recoveredHeight, baseHeight+2, wantTip))

	// Final full-mesh convergence.
	finalHash, finalHeight, finalSnap, finalOK := m.WaitConverged(ctx, m.AllNodes(), 4*time.Minute)
	res.observe("final_tips", finalSnap)
	res.check("Full mesh (6 nodes) converged after recovery", finalOK && finalHash == wantTip,
		fmt.Sprintf("converged=%v hash=%s height=%d", finalOK, finalHash, finalHeight))

	return res
}
