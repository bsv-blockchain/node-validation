package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
)

// Network is the compose bridge network name (project "node-validation").
const Network = "node-validation_tng-net"

// Node is the read surface every mesh node exposes for chaos assertions.
type Node interface {
	// Name is the short logical name (e.g. "teranode-1", "svnode-3").
	Name() string
	// Container is the docker container name for privileged operations.
	Container() string
	// BestBlockHash returns the node's current best-block hash.
	BestBlockHash(ctx context.Context) (string, error)
	// Height returns the node's current block height.
	Height(ctx context.Context) (int64, error)
}

// Miner is implemented by SV nodes that can mine blocks (regtest).
type Miner interface {
	Node
	// NewAddress returns a fresh address (wallet nodes only).
	NewAddress(ctx context.Context) (string, error)
	// Generate mines n blocks to addr and returns the block hashes.
	// Works on wallet-less nodes too: generatetoaddress takes an explicit
	// address and does not require a local wallet.
	Generate(ctx context.Context, n int, addr string) ([]string, error)
}

// svNode talks to a bitcoind SV node via `docker exec bitcoin-cli`, which is
// robust to host-port breakage caused by docker network disconnect/connect.
type svNode struct {
	d         *Docker
	name      string
	container string
}

func (n *svNode) Name() string      { return n.name }
func (n *svNode) Container() string { return n.container }

func (n *svNode) cli(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"bitcoin-cli", "-conf=/data/bitcoin.conf", "-datadir=/data"}, args...)
	return n.d.Exec(ctx, n.container, full...)
}

func (n *svNode) BestBlockHash(ctx context.Context) (string, error) {
	out, err := n.cli(ctx, "getbestblockhash")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (n *svNode) Height(ctx context.Context) (int64, error) {
	out, err := n.cli(ctx, "getblockcount")
	if err != nil {
		return 0, err
	}
	h, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s getblockcount parse %q: %w", n.name, out, err)
	}
	return h, nil
}

func (n *svNode) NewAddress(ctx context.Context) (string, error) {
	out, err := n.cli(ctx, "getnewaddress")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (n *svNode) Generate(ctx context.Context, count int, addr string) ([]string, error) {
	out, err := n.cli(ctx, "generatetoaddress", strconv.Itoa(count), addr)
	if err != nil {
		return nil, err
	}
	var hashes []string
	if err := json.Unmarshal([]byte(out), &hashes); err != nil {
		return nil, fmt.Errorf("%s generatetoaddress parse %q: %w", n.name, out, err)
	}
	return hashes, nil
}

// tnNode talks to a Teranode over its stable host RPC port using the existing
// black-box RPC client. Teranodes are never disconnected from the network by
// the chaos suite, so their host ports stay reachable.
type tnNode struct {
	d         *Docker
	name      string
	container string
	rpc       *teranode.RPCClient
}

func (n *tnNode) Name() string      { return n.name }
func (n *tnNode) Container() string { return n.container }

func (n *tnNode) BestBlockHash(ctx context.Context) (string, error) {
	return n.rpc.GetBestBlockHash(ctx)
}

func (n *tnNode) Height(ctx context.Context) (int64, error) {
	info, err := n.rpc.GetBlockchainInfo(ctx)
	if err != nil {
		return 0, err
	}
	return info.Blocks, nil
}

// sendFSMRun nudges the Teranode FSM out of IDLE into RUNNING. On a fresh
// start the legacy P2P server blocks until this transition (see
// compose/bootstrap.sh). Returns nil if the node is already running.
func (n *tnNode) sendFSMRun(ctx context.Context) error {
	_, err := n.d.Exec(ctx, n.container, "grpcurl", "-plaintext",
		"-d", `{"event": 1}`, "localhost:8087",
		"blockchain_api.BlockchainAPI.SendFSMEvent")
	// An error here usually just means the FSM is already RUNNING; treat as
	// best-effort and let convergence checks be the source of truth.
	return err
}

// Mesh is the running 6-node compose mesh under chaos.
type Mesh struct {
	D         *Docker
	Teranodes []*tnNode
	SVNodes   []*svNode
	logger    *slog.Logger
}

// teranodeHostPorts maps each Teranode to its published host RPC port.
var teranodeHostPorts = map[string]int{
	"teranode-1": 19292,
	"teranode-2": 29292,
	"teranode-3": 39292,
}

// NewMesh builds the mesh model. Teranode RPC clients are constructed against
// the published host ports using the credentials from cfg.Teranode. SV nodes
// are driven via docker exec, so they need no host config.
func NewMesh(d *Docker, cfg config.Config, logger *slog.Logger) (*Mesh, error) {
	if logger == nil {
		logger = slog.Default()
	}
	user, pass := cfg.Teranode.RPCUser, cfg.Teranode.RPCPass
	if user == "" {
		user = "bitcoin"
	}
	if pass == "" {
		pass = "bitcoin"
	}
	m := &Mesh{D: d, logger: logger}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("teranode-%d", i)
		port := teranodeHostPorts[name]
		rpc, err := teranode.NewRPCClient(fmt.Sprintf("http://localhost:%d", port), user, pass, logger)
		if err != nil {
			return nil, fmt.Errorf("build %s rpc client: %w", name, err)
		}
		m.Teranodes = append(m.Teranodes, &tnNode{
			d: d, name: name, container: fmt.Sprintf("node-validation-teranode-%d-1", i), rpc: rpc,
		})
		m.SVNodes = append(m.SVNodes, &svNode{
			d: d, name: fmt.Sprintf("svnode-%d", i), container: fmt.Sprintf("node-validation-svnode-%d-1", i),
		})
	}
	return m, nil
}

// Miner returns svnode-1, the only node with a wallet (funding + mining).
func (m *Mesh) Miner() *svNode { return m.SVNodes[0] }

// AllNodes returns teranodes followed by sv nodes.
func (m *Mesh) AllNodes() []Node {
	out := make([]Node, 0, len(m.Teranodes)+len(m.SVNodes))
	for _, n := range m.Teranodes {
		out = append(out, n)
	}
	for _, n := range m.SVNodes {
		out = append(out, n)
	}
	return out
}

// TipInfo is one node's observed tip.
type TipInfo struct {
	Hash   string `json:"hash"`
	Height int64  `json:"height"`
	Err    string `json:"err,omitempty"`
}

// Tips queries the given nodes once and returns name -> TipInfo.
func (m *Mesh) Tips(ctx context.Context, nodes []Node) map[string]TipInfo {
	out := make(map[string]TipInfo, len(nodes))
	for _, n := range nodes {
		ti := TipInfo{}
		h, err := n.BestBlockHash(ctx)
		if err != nil {
			ti.Err = err.Error()
			out[n.Name()] = ti
			continue
		}
		ti.Hash = h
		if hgt, herr := n.Height(ctx); herr == nil {
			ti.Height = hgt
		}
		out[n.Name()] = ti
	}
	return out
}

// WaitConverged polls nodes until they all report the same best-block hash,
// or the timeout elapses. Returns the converged hash/height and the final
// per-node snapshot. converged=false means the deadline passed while split.
func (m *Mesh) WaitConverged(ctx context.Context, nodes []Node, timeout time.Duration) (hash string, height int64, snap map[string]TipInfo, converged bool) {
	deadline := time.Now().Add(timeout)
	for {
		snap = m.Tips(ctx, nodes)
		hashes := map[string]bool{}
		var anyErr bool
		var h string
		var hh int64
		for _, ti := range snap {
			if ti.Err != "" {
				anyErr = true
				continue
			}
			hashes[ti.Hash] = true
			h = ti.Hash
			hh = ti.Height
		}
		if !anyErr && len(hashes) == 1 {
			return h, hh, snap, true
		}
		if time.Now().After(deadline) {
			return "", 0, snap, false
		}
		select {
		case <-ctx.Done():
			return "", 0, snap, false
		case <-time.After(3 * time.Second):
		}
	}
}

// WaitNodeTip polls a single node until its best-block hash equals want.
func (m *Mesh) WaitNodeTip(ctx context.Context, n Node, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		got, err := n.BestBlockHash(ctx)
		if err == nil && got == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s tip never reached %s within %v (last err=%v)", n.Name(), want, timeout, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// WaitNodeRPC polls a Teranode's RPC until it answers or the timeout elapses.
func (m *Mesh) WaitNodeRPC(ctx context.Context, n Node, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := n.BestBlockHash(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s RPC not responsive within %v", n.Name(), timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// RecoverTeranode brings a crashed/stopped Teranode back to an RPC-ready,
// FSM-RUNNING state and is the reliable single-node crash-recovery procedure
// used by OPS-1 and the shared Restore safety net.
//
// Empirically, a killed all-in-one Teranode is restored by a plain
// `docker start`: it restores its FSM from postgres (to RUNNING, because
// fsm_state_restore=true) and serves JSON-RPC again within a few seconds, then
// re-syncs any blocks it missed from its still-running peers. RPC is NOT gated
// on the FSM transition (the RPC server binds while the FSM is still being
// restored), so no FSM nudge is needed for RPC to come back — the nudge below
// is belt-and-suspenders for the IDLE-restore case.
//
// Rarely, a cold start wedges (e.g. a libp2p DHT bootstrap that loops on
// "failed to dial: context canceled" and never settles, leaving the RPC port
// unbound past a minute). The observed cure is a clean `docker restart`. This
// helper therefore: starts the container if stopped, waits a bounded window for
// RPC, and if RPC has not appeared escalates to a single `docker restart`
// before waiting out the remaining budget. Once RPC is up it nudges the FSM to
// RUNNING (a no-op when state restore already advanced it).
func (m *Mesh) RecoverTeranode(ctx context.Context, tn *tnNode, timeout time.Duration) error {
	startedAt := time.Now()

	// kill/stop never detaches the container, but a prior force-recreate or
	// partition test might have; make sure it can reach its peers + infra.
	_ = m.EnsureConnected(ctx, tn)

	if running, _ := m.D.Running(ctx, tn.container); !running {
		if err := m.D.Start(ctx, tn.container); err != nil {
			return fmt.Errorf("docker start %s: %w", tn.name, err)
		}
	}

	// Bounded wait on the plain start (the overwhelmingly common path: RPC is
	// back in a few seconds).
	firstWait := 90 * time.Second
	if firstWait > timeout {
		firstWait = timeout
	}
	if err := m.WaitNodeRPC(ctx, tn, firstWait); err == nil {
		_ = tn.sendFSMRun(ctx)
		return nil
	}

	// Escalate once: a clean restart reliably unwedges a stuck cold start.
	m.logger.Warn("recover: RPC not up after start, escalating to docker restart", "node", tn.name)
	if err := m.D.Restart(ctx, tn.container); err != nil {
		return fmt.Errorf("docker restart %s: %w", tn.name, err)
	}
	remaining := timeout - time.Since(startedAt)
	if remaining < 30*time.Second {
		remaining = 30 * time.Second
	}
	if err := m.WaitNodeRPC(ctx, tn, remaining); err != nil {
		return err
	}
	_ = tn.sendFSMRun(ctx)
	return nil
}

// EnsureConnected re-attaches a node to the network if it is detached,
// restoring its compose DNS aliases (service short name + container name) so it
// stays resolvable by name. See Docker.NetworkConnect for why aliases matter.
func (m *Mesh) EnsureConnected(ctx context.Context, n Node) error {
	in, err := m.D.NetworkContains(ctx, Network, n.Container())
	if err != nil {
		return err
	}
	if in {
		return nil
	}
	return m.D.NetworkConnect(ctx, Network, n.Container(), n.Name(), n.Container())
}

// Restore is the self-healing safety net every chaos test defers. It brings
// the mesh back to a healthy, connected, converged state regardless of where
// a test left off:
//
//  1. Start any stopped container (kill/stop recovery).
//  2. Reconnect any detached SV node, then restart it — a reconnected
//     bitcoind needs a fresh start to re-handshake its P2P peers reliably
//     (empirically: addnode does not always re-establish after reconnect).
//  3. Reconnect any detached Teranode and restart it; a restarted Teranode
//     needs an FSM RUN nudge to unblock its legacy P2P server.
//  4. Wait for all 6 nodes to converge on one tip.
//
// Errors are returned for logging; Restore is best-effort and never panics.
func (m *Mesh) Restore(ctx context.Context, timeout time.Duration) error {
	// 1. Start stopped containers.
	for _, n := range append(svContainers(m), tnContainers(m)...) {
		running, err := m.D.Running(ctx, n)
		if err != nil {
			m.logger.Warn("restore: inspect failed", "container", n, "err", err)
			continue
		}
		if !running {
			m.logger.Info("restore: starting stopped container", "container", n)
			if err := m.D.Start(ctx, n); err != nil {
				m.logger.Warn("restore: start failed", "container", n, "err", err)
			}
		}
	}

	// 2. Reconnect + restart detached SV nodes.
	for _, sv := range m.SVNodes {
		in, err := m.D.NetworkContains(ctx, Network, sv.container)
		if err != nil {
			m.logger.Warn("restore: net inspect failed", "container", sv.container, "err", err)
			continue
		}
		if !in {
			m.logger.Info("restore: reconnecting + restarting sv node", "container", sv.container)
			if err := m.D.NetworkConnect(ctx, Network, sv.container, sv.name, sv.container); err != nil {
				m.logger.Warn("restore: connect failed", "container", sv.container, "err", err)
			}
			if err := m.D.Restart(ctx, sv.container); err != nil {
				m.logger.Warn("restore: restart failed", "container", sv.container, "err", err)
			}
		}
	}

	// 3. Reconnect detached Teranodes (then they get the FSM nudge below).
	for _, tn := range m.Teranodes {
		in, err := m.D.NetworkContains(ctx, Network, tn.container)
		if err == nil && !in {
			m.logger.Info("restore: reconnecting + restarting teranode", "container", tn.container)
			_ = m.D.NetworkConnect(ctx, Network, tn.container, tn.name, tn.container)
			_ = m.D.Restart(ctx, tn.container)
		}
	}

	// 4. Recover every Teranode to RPC-ready + FSM RUNNING. RecoverTeranode is
	// the reliable single-node procedure: plain start (the common path settles
	// in seconds) with a docker-restart escalation if a cold start wedges, then
	// an FSM nudge. The generous per-node budget is retained for headroom.
	for _, tn := range m.Teranodes {
		if err := m.RecoverTeranode(ctx, tn, 6*time.Minute); err != nil {
			m.logger.Warn("restore: teranode recovery failed", "node", tn.name, "err", err)
		}
	}

	_, _, _, converged := m.WaitConverged(ctx, m.AllNodes(), timeout)
	if !converged {
		return fmt.Errorf("restore: mesh did not reconverge within %v", timeout)
	}
	return nil
}

func svContainers(m *Mesh) []string {
	out := make([]string, 0, len(m.SVNodes))
	for _, n := range m.SVNodes {
		out = append(out, n.container)
	}
	return out
}

func tnContainers(m *Mesh) []string {
	out := make([]string, 0, len(m.Teranodes))
	for _, n := range m.Teranodes {
		out = append(out, n.container)
	}
	return out
}
