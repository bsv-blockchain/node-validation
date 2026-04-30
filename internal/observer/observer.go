package observer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// TipReader is the minimal interface needed for chain-tip polling.
// Both *teranode.RPCClient and *svnode.RPCClient satisfy it.
type TipReader interface {
	GetBestBlockHash(ctx context.Context) (string, error)
	GetBlockchainInfo(ctx context.Context) (json.RawMessage, error)
}

// TipSnapshot is one observation of one node's best tip.
type TipSnapshot struct {
	Time   time.Time
	Source string
	Hash   string
	Height int64
}

// ReorgEvent is observed when a node's best tip moves to a different chain
// (best-hash changes without height monotonically increasing — i.e. height
// drops or stays equal with a different hash).
type ReorgEvent struct {
	Time   time.Time
	Source string
	From   TipSnapshot
	To     TipSnapshot
}

// Observer polls a set of TipReaders at a fixed interval and emits
// snapshots to a buffered channel.
type Observer struct {
	rpcs     map[string]TipReader
	interval time.Duration
	logger   *slog.Logger
}

// NewObserver constructs an Observer.
func NewObserver(rpcs map[string]TipReader, interval time.Duration, logger *slog.Logger) *Observer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Observer{rpcs: rpcs, interval: interval, logger: logger}
}

// Run polls until the deadline; returns all snapshots collected.
func (o *Observer) Run(ctx context.Context, until time.Time) []TipSnapshot {
	var (
		mu        sync.Mutex
		snapshots []TipSnapshot
	)
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	for time.Now().Before(until) {
		select {
		case <-ctx.Done():
			return snapshots
		case <-ticker.C:
			now := time.Now()
			for label, rpc := range o.rpcs {
				h, err := rpc.GetBestBlockHash(ctx)
				if err != nil {
					o.logger.Debug("observer: getbestblockhash error", "src", label, "err", err)
					continue
				}
				var info struct {
					Blocks int64 `json:"blocks"`
				}
				raw, err := rpc.GetBlockchainInfo(ctx)
				height := int64(-1)
				if err == nil {
					_ = json.Unmarshal(raw, &info)
					height = info.Blocks
				}
				mu.Lock()
				snapshots = append(snapshots, TipSnapshot{
					Time: now, Source: label, Hash: h, Height: height,
				})
				mu.Unlock()
			}
		}
	}
	return snapshots
}

// DivergenceCount returns the number of timestamps where ≥2 sources
// reported different best-block hashes simultaneously.
func DivergenceCount(snapshots []TipSnapshot) int {
	// Group by ~50ms time window (since polls happen at the same moment, snapshots
	// from one poll round share the same Time within microseconds).
	rounds := map[time.Time]map[string]string{}
	for _, s := range snapshots {
		// Round Time to interval-bucket (1s precision suffices).
		bucket := s.Time.Round(time.Second)
		if rounds[bucket] == nil {
			rounds[bucket] = map[string]string{}
		}
		rounds[bucket][s.Source] = s.Hash
	}
	count := 0
	for _, hashes := range rounds {
		seen := map[string]bool{}
		for _, h := range hashes {
			seen[h] = true
		}
		if len(seen) > 1 {
			count++
		}
	}
	return count
}

// ReorgsObserved scans per-source snapshots for any best-hash change at the
// same or lower height (= chain switched, not advanced).
func ReorgsObserved(snapshots []TipSnapshot) []ReorgEvent {
	bySource := map[string][]TipSnapshot{}
	for _, s := range snapshots {
		bySource[s.Source] = append(bySource[s.Source], s)
	}
	var events []ReorgEvent
	for src, ss := range bySource {
		for i := 1; i < len(ss); i++ {
			prev, cur := ss[i-1], ss[i]
			if prev.Hash == cur.Hash {
				continue
			}
			// Reorg signal: new hash with height ≤ previous height.
			if cur.Height <= prev.Height && prev.Height > 0 {
				events = append(events, ReorgEvent{
					Time: cur.Time, Source: src, From: prev, To: cur,
				})
			}
		}
		_ = src // used as map key above
	}
	return events
}

// ConvergedAt returns the earliest time after `from` at which all sources
// reported the same hash. Returns zero time if never converged within the
// snapshots.
func ConvergedAt(snapshots []TipSnapshot, from time.Time, expectedHash string) time.Time {
	rounds := map[time.Time]map[string]string{}
	for _, s := range snapshots {
		if s.Time.Before(from) {
			continue
		}
		bucket := s.Time.Round(time.Second)
		if rounds[bucket] == nil {
			rounds[bucket] = map[string]string{}
		}
		rounds[bucket][s.Source] = s.Hash
	}
	// Sort buckets ascending.
	var keys []time.Time
	for k := range rounds {
		keys = append(keys, k)
	}
	sortTimes(keys)
	for _, k := range keys {
		hashes := rounds[k]
		allMatch := true
		for _, h := range hashes {
			if h != expectedHash {
				allMatch = false
				break
			}
		}
		if allMatch && len(hashes) >= 2 {
			return k
		}
	}
	return time.Time{}
}

func sortTimes(t []time.Time) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && t[j].Before(t[j-1]); j-- {
			t[j], t[j-1] = t[j-1], t[j]
		}
	}
}
