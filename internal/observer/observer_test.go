package observer

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"
)

// stubTipReader is a minimal TipReader that always returns fixed values.
type stubTipReader struct {
	hash   string
	height int64
}

func (s *stubTipReader) GetBestBlockHash(_ context.Context) (string, error) {
	return s.hash, nil
}

func (s *stubTipReader) GetBlockchainInfo(_ context.Context) (json.RawMessage, error) {
	raw, _ := json.Marshal(map[string]int64{"blocks": s.height})
	return raw, nil
}

func TestNewObserver_nilLogger(t *testing.T) {
	rpcs := map[string]TipReader{"node": &stubTipReader{hash: "abc", height: 1}}
	obs := NewObserver(rpcs, time.Millisecond, nil)
	if obs == nil {
		t.Fatal("NewObserver returned nil")
	}
}

func TestObserver_Run_collectsSnapshots(t *testing.T) {
	stub := &stubTipReader{hash: "deadbeef", height: 10}
	rpcs := map[string]TipReader{"node1": stub}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	obs := NewObserver(rpcs, 10*time.Millisecond, logger)

	ctx := context.Background()
	until := time.Now().Add(35 * time.Millisecond) // allow 2-3 ticks
	snaps := obs.Run(ctx, until)
	if len(snaps) == 0 {
		t.Fatal("expected at least one snapshot, got 0")
	}
	for _, s := range snaps {
		if s.Hash != "deadbeef" {
			t.Errorf("unexpected hash %q", s.Hash)
		}
		if s.Source != "node1" {
			t.Errorf("unexpected source %q", s.Source)
		}
		if s.Height != 10 {
			t.Errorf("unexpected height %d", s.Height)
		}
	}
}

func TestObserver_Run_cancelledContext(t *testing.T) {
	stub := &stubTipReader{hash: "aabbcc", height: 5}
	rpcs := map[string]TipReader{"node1": stub}
	obs := NewObserver(rpcs, 5*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	until := time.Now().Add(100 * time.Millisecond)
	snaps := obs.Run(ctx, until)
	// Context already cancelled; should return quickly with 0 or few snapshots.
	_ = snaps // just verify it doesn't hang
}

func TestDivergenceCount_allAgree(t *testing.T) {
	now := time.Now()
	ss := []TipSnapshot{
		{Time: now, Source: "a", Hash: "x", Height: 1},
		{Time: now, Source: "b", Hash: "x", Height: 1},
	}
	if got := DivergenceCount(ss); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestDivergenceCount_disagree(t *testing.T) {
	now := time.Now()
	ss := []TipSnapshot{
		{Time: now, Source: "a", Hash: "x", Height: 1},
		{Time: now, Source: "b", Hash: "y", Height: 1},
	}
	if got := DivergenceCount(ss); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestReorgsObserved_simpleReorg(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "B0", Height: 5},
		{Time: t0.Add(time.Second), Source: "a", Hash: "B1", Height: 6},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "T2", Height: 7},
		{Time: t0.Add(3 * time.Second), Source: "a", Hash: "T3", Height: 6}, // reorg: hash changed, height ≤ prev
	}
	events := ReorgsObserved(ss)
	if len(events) != 1 {
		t.Errorf("got %d reorgs, want 1", len(events))
	}
}

func TestReorgsObserved_noReorgOnAdvance(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "B0", Height: 5},
		{Time: t0.Add(time.Second), Source: "a", Hash: "B1", Height: 6},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "B2", Height: 7},
	}
	if events := ReorgsObserved(ss); len(events) != 0 {
		t.Errorf("got %d, want 0", len(events))
	}
}

func TestConvergedAt(t *testing.T) {
	t0 := time.Now()
	ss := []TipSnapshot{
		{Time: t0, Source: "a", Hash: "X", Height: 1},
		{Time: t0, Source: "b", Hash: "Y", Height: 1},
		{Time: t0.Add(2 * time.Second), Source: "a", Hash: "Z", Height: 2},
		{Time: t0.Add(2 * time.Second), Source: "b", Hash: "Z", Height: 2},
	}
	got := ConvergedAt(ss, t0, "Z")
	if got.IsZero() {
		t.Error("expected convergence")
	}
}
