// internal/testrunner/suite_test.go
package testrunner

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

func newTestEnv(t *testing.T) *Env {
	t.Helper()
	cfg := config.Config{TestTimeout: 5 * time.Second}
	return NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
}

func TestRegister_unknownIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	s := NewSuite(newTestEnv(t))
	s.Register("DOES-NOT-EXIST", func(context.Context, *Env) Result { return Result{} })
}

func TestRegister_excludedTCPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-IN_SCOPE id")
		}
	}()
	s := NewSuite(newTestEnv(t))
	s.Register("IBD-1", func(context.Context, *Env) Result { return Result{} })
}

func TestRegister_duplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected duplicate-register panic")
		}
	}()
	s := NewSuite(newTestEnv(t))
	fn := func(context.Context, *Env) Result { return Result{Status: StatusPass} }
	s.Register("PC-1", fn)
	s.Register("PC-1", fn)
}

func TestRun_happyPath(t *testing.T) {
	s := NewSuite(newTestEnv(t))
	s.Register("PC-1", func(context.Context, *Env) Result {
		return Result{ID: "PC-1", Status: StatusPass}
	})
	results := s.Run(context.Background())
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("want PASS, got %s", results[0].Status)
	}
	if results[0].Severity != matrix.SeverityCritical {
		t.Errorf("severity should be sourced from manifest: got %s", results[0].Severity)
	}
}

func TestRun_panicBecomesError(t *testing.T) {
	s := NewSuite(newTestEnv(t))
	s.Register("PC-1", func(context.Context, *Env) Result { panic("boom") })
	results := s.Run(context.Background())
	if results[0].Status != StatusError || !strings.Contains(results[0].Err, "boom") {
		t.Fatalf("want ERROR with boom, got %+v", results[0])
	}
}

func TestRun_ctxCancelInterrupts(t *testing.T) {
	env := newTestEnv(t)
	s := NewSuite(env)
	s.Register("PC-1", func(ctx context.Context, e *Env) Result {
		<-ctx.Done()
		return Result{ID: "PC-1", Status: StatusError, SkipReason: "interrupted"}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := s.Run(ctx)
	if results[0].Status != StatusError {
		t.Errorf("want ERROR after cancel, got %s", results[0].Status)
	}
}

func TestRun_filterOnlyMarksOthersNotRun(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.Only = []string{"PC-1"}
	s := NewSuite(env)
	pc1Ran := false
	s.Register("PC-1", func(context.Context, *Env) Result {
		pc1Ran = true
		return Result{ID: "PC-1", Status: StatusPass}
	})
	s.Register("PC-2", func(context.Context, *Env) Result {
		t.Fatal("PC-2 should not run")
		return Result{}
	})
	results := s.Run(context.Background())
	if !pc1Ran {
		t.Fatal("PC-1 should have run")
	}
	var seenPC2 bool
	for _, r := range results {
		if r.ID == "PC-2" {
			seenPC2 = true
			if r.Status != StatusNotRun {
				t.Errorf("PC-2 status: want NOT_RUN, got %s", r.Status)
			}
			if !strings.Contains(r.SkipReason, "filtered") {
				t.Errorf("PC-2 skip reason: %q", r.SkipReason)
			}
		}
	}
	if !seenPC2 {
		t.Fatal("PC-2 must appear in results as NOT_RUN")
	}
}

func TestRun_timeoutWhenTestIgnoresCtx(t *testing.T) {
	env := newTestEnv(t)
	env.Cfg.TestTimeout = 50 * time.Millisecond
	s := NewSuite(env)
	s.Register("PC-1", func(ctx context.Context, e *Env) Result {
		// Deliberately ignore ctx — sleep longer than timeout.
		time.Sleep(500 * time.Millisecond)
		return Result{ID: "PC-1", Status: StatusPass}
	})
	results := s.Run(context.Background())
	if len(results) != 1 || results[0].Status != StatusError {
		t.Fatalf("want 1 ERROR result on timeout, got %+v", results)
	}
	if results[0].SkipReason != "timed out" {
		t.Errorf("want skip reason 'timed out', got %q", results[0].SkipReason)
	}
}

var _ = errors.New
