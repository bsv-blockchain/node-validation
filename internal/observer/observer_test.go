package observer

import (
	"testing"
	"time"
)

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
