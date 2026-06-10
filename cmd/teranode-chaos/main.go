// Command teranode-chaos runs the privileged, NON-GATING chaos test suite
// (OPS-1 service failure & recovery, OPS-2 network partition & reorg
// convergence) against the local docker-compose mesh.
//
// It is intentionally a separate binary from teranode-acceptance: its results
// are reported on their own scorecard and never feed the acceptance verdict
// (testrunner.ComputeVerdict). The manifest rows OPS-1/OPS-2 remain
// EXCLUDED_PRIVILEGED for the acceptance suite.
//
// Run via `make chaos` (which builds and points it at config.docker.yaml).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/chaos"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

func run(args, environ []string, stdout, stderr *os.File) int {
	cfg, err := config.Load(args, environ)
	switch {
	case errors.Is(err, config.ErrHelp):
		return 0
	case errors.Is(err, config.ErrVersion):
		fmt.Fprintf(stdout, "teranode-chaos %s\n", version)
		return 0
	case err != nil:
		fmt.Fprintln(stderr, err)
		return 4
	}

	logger := newLogger(cfg.Verbose, stderr)

	// Chaos runs only against the regtest docker mesh: it shells out to docker
	// to kill/partition real containers. Refuse anything else outright.
	if cfg.Network != config.NetworkRegtest {
		fmt.Fprintf(stderr, "teranode-chaos refuses to run on network %q; it is destructive and regtest-only\n", cfg.Network)
		return 4
	}

	docker := chaos.NewDocker(logger)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := docker.Available(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return 4
	}

	mesh, err := chaos.NewMesh(docker, cfg, logger)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 4
	}

	reg := &chaos.Registry{}
	// OPS-2 first (network partition), then OPS-1 (service failure). The
	// shared --only/--skip flags select a subset, which is essential for
	// bounded iteration: a single OPS-1 run can be exercised without the
	// long combined suite.
	register := func(id, title string, fn chaos.TestFunc) {
		if !selected(id, cfg.Only, cfg.Skip) {
			return
		}
		reg.Register(id, title, fn)
	}
	register("OPS-2", "Network Partition and Reorg Convergence", chaos.RunOPS2)
	register("OPS-1", "Service Failure and Recovery", chaos.RunOPS1)

	started := time.Now().UTC()
	results := reg.Run(ctx, mesh, logger)
	finished := time.Now().UTC()

	report := chaos.NewReport(results, string(cfg.Network), started, finished)

	if err := chaos.WriteText(stdout, report); err != nil {
		fmt.Fprintf(stderr, "write text: %v\n", err)
		return 1
	}
	reportPath := reportJSONPath(environ)
	if err := chaos.WriteJSON(reportPath, report); err != nil {
		fmt.Fprintf(stderr, "write json: %v\n", err)
		return 1
	}

	// Non-gating: this exit code is the chaos suite's own signal for operators
	// and CI; it is wholly independent of the acceptance verdict computed by
	// the separate teranode-acceptance binary.
	if !report.AllPassed() {
		return 1
	}
	return 0
}

// selected reports whether a chaos test ID should run given the shared
// --only / --skip flag sets. --only wins when non-empty; otherwise a test
// runs unless it appears in --skip. (config.Validate already rejects setting
// both at once.)
func selected(id string, only, skip []string) bool {
	if len(only) > 0 {
		for _, o := range only {
			if o == id {
				return true
			}
		}
		return false
	}
	for _, s := range skip {
		if s == id {
			return false
		}
	}
	return true
}

// reportJSONPath returns the chaos JSON report path. Defaults to
// "chaos-report.json"; override with CHAOS_REPORT_JSON. (Kept off the shared
// config flagset so the acceptance flags stay untouched.)
func reportJSONPath(environ []string) string {
	const key = "CHAOS_REPORT_JSON="
	for _, kv := range environ {
		if len(kv) > len(key) && kv[:len(key)] == key {
			return kv[len(key):]
		}
	}
	return "chaos-report.json"
}

func newLogger(verbose bool, w *os.File) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
