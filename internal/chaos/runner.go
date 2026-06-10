package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sort"
	"time"
)

// Status is the outcome of a chaos test.
type Status string

const (
	StatusPass  Status = "PASS"
	StatusFail  Status = "FAIL"
	StatusError Status = "ERROR"
)

// Check is one acceptance-criterion check inside a chaos Result.
type Check struct {
	Description string `json:"description"`
	Pass        bool   `json:"pass"`
	Detail      string `json:"detail,omitempty"`
}

// Result is what a chaos test returns.
type Result struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Status       Status         `json:"status"`
	StartedAt    time.Time      `json:"started_at"`
	Duration     time.Duration  `json:"duration_ns"`
	Checks       []Check        `json:"checks,omitempty"`
	Observations map[string]any `json:"observations,omitempty"`
	Err          string         `json:"err,omitempty"`
}

// pass appends a passing check.
func (r *Result) pass(desc, detail string) { r.Checks = append(r.Checks, Check{desc, true, detail}) }

// fail appends a failing check.
func (r *Result) fail(desc, detail string) { r.Checks = append(r.Checks, Check{desc, false, detail}) }

// check appends a check from a boolean.
func (r *Result) check(desc string, ok bool, detail string) {
	r.Checks = append(r.Checks, Check{Description: desc, Pass: ok, Detail: detail})
}

// observe records an observation value.
func (r *Result) observe(key string, val any) {
	if r.Observations == nil {
		r.Observations = map[string]any{}
	}
	r.Observations[key] = val
}

// derive sets Status from the checks: any failing check -> FAIL, none -> ERROR.
func (r *Result) derive() {
	if r.Status == StatusError {
		return
	}
	if len(r.Checks) == 0 {
		r.Status = StatusError
		return
	}
	for _, c := range r.Checks {
		if !c.Pass {
			r.Status = StatusFail
			return
		}
	}
	r.Status = StatusPass
}

// TestFunc is the signature every chaos test implements. It receives the mesh
// and a logger and returns a populated Result. Tests MUST restore the mesh to
// a healthy connected state via deferred cleanup, even on failure.
type TestFunc func(ctx context.Context, m *Mesh, logger *slog.Logger) Result

type registration struct {
	ID    string
	Title string
	Fn    TestFunc
}

// Registry holds the chaos tests, kept separate from the acceptance Suite so
// it can never feed testrunner.ComputeVerdict.
type Registry struct {
	reg []registration
}

// Register adds a chaos test.
func (r *Registry) Register(id, title string, fn TestFunc) {
	r.reg = append(r.reg, registration{ID: id, Title: title, Fn: fn})
}

// Run executes every registered chaos test sequentially, recovering panics
// into ERROR results so a single broken test cannot abort the suite (and so
// its deferred mesh restore still runs).
func (r *Registry) Run(ctx context.Context, m *Mesh, logger *slog.Logger) []Result {
	results := make([]Result, 0, len(r.reg))
	for _, reg := range r.reg {
		results = append(results, runOne(ctx, m, logger, reg))
	}
	return results
}

func runOne(ctx context.Context, m *Mesh, logger *slog.Logger, reg registration) (out Result) {
	out = Result{ID: reg.ID, Title: reg.Title, Status: StatusError, StartedAt: time.Now().UTC()}
	defer func() {
		if rec := recover(); rec != nil {
			out.Status = StatusError
			out.Err = fmt.Sprintf("panic: %v\n%s", rec, debug.Stack())
		}
		out.Duration = time.Since(out.StartedAt)
	}()
	logger.Info("chaos: starting", "id", reg.ID, "title", reg.Title)
	res := reg.Fn(ctx, m, logger)
	res.ID = reg.ID
	if res.Title == "" {
		res.Title = reg.Title
	}
	if res.StartedAt.IsZero() {
		res.StartedAt = out.StartedAt
	}
	logger.Info("chaos: finished", "id", reg.ID, "status", res.Status)
	return res
}

// Report bundles a chaos run for serialization. It is explicitly labeled
// non-gating: this artifact never participates in the acceptance verdict.
type Report struct {
	Suite      string    `json:"suite"`
	Gating     bool      `json:"gating"`
	Note       string    `json:"note"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Network    string    `json:"network"`
	Results    []Result  `json:"results"`
}

const nonGatingNote = "NON-GATING privileged chaos suite. These results are an additional, " +
	"parallel artifact and never feed the acceptance verdict (testrunner.ComputeVerdict). " +
	"Manifest rows OPS-1/OPS-2 remain EXCLUDED_PRIVILEGED for the acceptance suite."

// NewReport assembles a Report from results.
func NewReport(results []Result, network string, started, finished time.Time) Report {
	return Report{
		Suite:      "chaos",
		Gating:     false,
		Note:       nonGatingNote,
		StartedAt:  started,
		FinishedAt: finished,
		Network:    network,
		Results:    results,
	}
}

// AllPassed reports whether every result is PASS.
func (rep Report) AllPassed() bool {
	for _, r := range rep.Results {
		if r.Status != StatusPass {
			return false
		}
	}
	return len(rep.Results) > 0
}

// WriteJSON writes the report as indented JSON to path (skipped if empty).
func WriteJSON(path string, rep Report) error {
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// WriteText renders a human-readable scorecard to w.
func WriteText(w io.Writer, rep Report) error {
	fmt.Fprintln(w, "============================================================")
	fmt.Fprintln(w, " TERANODE CHAOS SUITE — PRIVILEGED, NON-GATING")
	fmt.Fprintln(w, "============================================================")
	fmt.Fprintf(w, " network    : %s\n", rep.Network)
	fmt.Fprintf(w, " started    : %s\n", rep.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(w, " finished   : %s\n", rep.FinishedAt.Format(time.RFC3339))
	fmt.Fprintf(w, " gating     : %v  (results do NOT affect the acceptance verdict)\n", rep.Gating)
	fmt.Fprintln(w, "------------------------------------------------------------")
	for _, r := range rep.Results {
		fmt.Fprintf(w, "\n[%s] %s — %s  (%s)\n", r.Status, r.ID, r.Title, r.Duration.Round(time.Second))
		if r.Err != "" {
			fmt.Fprintf(w, "    error: %s\n", firstLine(r.Err))
		}
		for _, c := range r.Checks {
			mark := "PASS"
			if !c.Pass {
				mark = "FAIL"
			}
			fmt.Fprintf(w, "    [%s] %s\n", mark, c.Description)
			if c.Detail != "" {
				fmt.Fprintf(w, "           %s\n", c.Detail)
			}
		}
		if len(r.Observations) > 0 {
			fmt.Fprintln(w, "    observations:")
			keys := make([]string, 0, len(r.Observations))
			for k := range r.Observations {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "      - %s: %v\n", k, r.Observations[k])
			}
		}
	}
	fmt.Fprintln(w, "\n------------------------------------------------------------")
	pass, fail, errc := 0, 0, 0
	for _, r := range rep.Results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		default:
			errc++
		}
	}
	fmt.Fprintf(w, " summary: %d PASS, %d FAIL, %d ERROR (non-gating)\n", pass, fail, errc)
	fmt.Fprintln(w, "============================================================")
	return nil
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
