// Command teranode-acceptance runs the acceptance-test suite against a
// configured Teranode + SV Node pair and emits a complete report.
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
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/overrides"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

var version = "dev"

func main() {
	exitCode := run(os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args, environ []string, stdout, stderr *os.File) int {
	cfg, err := config.Load(args, environ)
	switch {
	case errors.Is(err, config.ErrHelp):
		return 0
	case errors.Is(err, config.ErrVersion):
		fmt.Fprintf(stdout, "teranode-acceptance %s\n", version)
		return 0
	case err != nil:
		fmt.Fprintln(stderr, err)
		return 4
	}

	logger := newLogger(cfg.Verbose, stderr)
	manifest := matrix.Load()

	ovr, err := overrides.Load(cfg.ReviewerOverrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 4
	}

	env := testrunner.NewEnv(cfg, logger, manifest, time.Now)
	suite := testrunner.NewSuite(env)
	registerTests(suite)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	started := time.Now().UTC()
	results := suite.Run(ctx)
	finished := time.Now().UTC()

	model, err := testrunner.BuildReportModel(env, results, ovr, started, finished, version)
	if err != nil {
		fmt.Fprintf(stderr, "build report: %v\n", err)
		return 1
	}

	if err := testrunner.WriteText(stdout, model); err != nil {
		fmt.Fprintf(stderr, "write text: %v\n", err)
		return 1
	}
	if err := testrunner.WriteJSON(cfg.ReportJSON, model); err != nil {
		fmt.Fprintf(stderr, "write json: %v\n", err)
		return 1
	}
	if err := testrunner.WriteHTML(cfg.ReportHTML, model); err != nil {
		fmt.Fprintf(stderr, "write html: %v\n", err)
		return 1
	}

	return model.Verdict.ExitCode
}

func newLogger(verbose bool, w *os.File) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
