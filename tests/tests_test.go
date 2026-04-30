package tests

import (
	"context"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

// stubMempool is a mempoolReader that returns a fixed txid list.
type stubMempool struct{ txids []string }

func (s *stubMempool) GetRawMempool(_ context.Context) ([]string, error) {
	return s.txids, nil
}

func TestPollMempoolUntil_allPresent(t *testing.T) {
	rpc := &stubMempool{txids: []string{"aaa", "bbb", "ccc"}}
	seen, allSeen := pollMempoolUntil(context.Background(), rpc, []string{"aaa", "bbb"}, 2*time.Second)
	if !allSeen {
		t.Error("want allSeen=true")
	}
	if len(seen) != 2 {
		t.Errorf("seen count: %d want 2", len(seen))
	}
}

func TestPollMempoolUntil_timeoutPartial(t *testing.T) {
	rpc := &stubMempool{txids: []string{"aaa"}}
	seen, allSeen := pollMempoolUntil(context.Background(), rpc, []string{"aaa", "missing"}, 300*time.Millisecond)
	if allSeen {
		t.Error("want allSeen=false (missing not present)")
	}
	if !seen["aaa"] {
		t.Error("aaa should be seen")
	}
}

func TestDeriveStatus_allPass(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		ok("b", ""),
	}
	if got := deriveStatus(c); got != testrunner.StatusPass {
		t.Errorf("got %s want PASS", got)
	}
}

func TestDeriveStatus_anyRequiredFail(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		fail("b", "boom"),
	}
	if got := deriveStatus(c); got != testrunner.StatusFail {
		t.Errorf("got %s want FAIL", got)
	}
}

func TestDeriveStatus_emptyIsError(t *testing.T) {
	if got := deriveStatus(nil); got != testrunner.StatusError {
		t.Errorf("got %s want ERROR", got)
	}
}

func TestClassifyRateLimit_429(t *testing.T) {
	if _, ok := classifyRateLimit(errFromString("HTTP 429 Too Many Requests")); !ok {
		t.Error("want classified as limit")
	}
}

func TestClassifyRateLimit_nilNotLimit(t *testing.T) {
	if _, ok := classifyRateLimit(nil); ok {
		t.Error("nil should not be a limit")
	}
}

// errFromString is a tiny helper so tests don't need to import errors.
type errString string

func (e errString) Error() string { return string(e) }

func errFromString(s string) error { return errString(s) }

func TestMeasureLatency_p95(t *testing.T) {
	// Synthetic: probeFn sleeps for an increasing duration.
	calls := 0
	probe := func(_ string) error {
		calls++
		time.Sleep(time.Duration(calls) * time.Millisecond)
		return nil
	}
	inputs := intRange(1, 20)
	p95 := measureLatency(context.Background(), "synth", inputs, probe)
	// 20 inputs; p95 index = int(0.95*20) = 19 (last). Sleep was 1..20ms,
	// so p95 ≈ 19-20ms. Allow generous tolerance.
	if p95 < 15*time.Millisecond || p95 > 50*time.Millisecond {
		t.Errorf("p95 out of expected range: %v", p95)
	}
}

func TestIntRange(t *testing.T) {
	got := intRange(1, 3)
	if len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Errorf("intRange: %v", got)
	}
}
