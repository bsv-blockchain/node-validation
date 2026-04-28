// internal/testrunner/suite.go
package testrunner

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

// TestFunc is the signature every test in the tests/ tree implements.
type TestFunc func(ctx context.Context, env *Env) Result

type registration struct {
	ID       string
	Severity matrix.Severity
	Fn       TestFunc
}

// Suite holds the registered tests and runs them in registration order.
type Suite struct {
	env *Env
	reg []registration
}

// NewSuite constructs an empty Suite.
func NewSuite(env *Env) *Suite { return &Suite{env: env} }

// Register adds a test by ID. Severity is read from the manifest. The
// function panics on programmer errors (unknown ID, non-IN_SCOPE row,
// duplicate registration); these are caught by `go test`.
func (s *Suite) Register(id string, fn TestFunc) {
	entry, ok := s.env.Manifest.ByID(id)
	if !ok {
		panic("testrunner: unknown ID " + id)
	}
	if entry.Kind != matrix.KindTC && entry.Kind != matrix.KindNEW {
		panic("testrunner: cannot register kind " + string(entry.Kind) + " (id=" + id + ")")
	}
	if entry.TestCaseStatus != matrix.TCInScope {
		panic("testrunner: cannot register non-IN_SCOPE test " + id)
	}
	for _, r := range s.reg {
		if r.ID == id {
			panic("testrunner: duplicate registration for " + id)
		}
	}
	s.reg = append(s.reg, registration{ID: id, Severity: entry.Severity, Fn: fn})
}

// Run executes registered tests sequentially. Tests filtered out by --only
// or --skip appear in the result slice with Status = NOT_RUN. Returns one
// Result per registered test, in registration order.
func (s *Suite) Run(ctx context.Context) []Result {
	results := make([]Result, 0, len(s.reg))
	for _, r := range s.reg {
		results = append(results, s.runOne(ctx, r))
	}
	return results
}

func (s *Suite) runOne(ctx context.Context, r registration) (out Result) {
	out.ID = r.ID
	out.Severity = r.Severity
	if entry, ok := s.env.Manifest.ByID(r.ID); ok {
		out.Title = entry.Title
		out.SatisfiesRequirements = append([]string(nil), entry.SatisfiesReqs...)
	}
	out.StartedAt = s.env.Now()

	// Filter handling.
	if filtered, reason := s.filterReason(r.ID); filtered {
		out.Status = StatusNotRun
		out.SkipReason = reason
		out.Duration = 0
		return out
	}

	timeout := s.env.Cfg.TestTimeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer func() {
		if rec := recover(); rec != nil {
			out.Status = StatusError
			out.Err = fmt.Sprintf("panic: %v\n%s", rec, debug.Stack())
		}
		out.Duration = s.env.Now().Sub(out.StartedAt)
	}()

	res := r.Fn(tctx, s.env)
	res.ID = r.ID
	if res.Title == "" {
		res.Title = out.Title
	}
	res.Severity = r.Severity
	res.StartedAt = out.StartedAt
	res.SatisfiesRequirements = out.SatisfiesRequirements
	out = res
	return out
}

func (s *Suite) filterReason(id string) (bool, string) {
	if len(s.env.Cfg.Only) > 0 && !slices.Contains(s.env.Cfg.Only, id) {
		return true, "filtered out by --only"
	}
	if len(s.env.Cfg.Skip) > 0 && slices.Contains(s.env.Cfg.Skip, id) {
		return true, "filtered out by --skip"
	}
	return false, ""
}
