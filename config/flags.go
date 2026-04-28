// config/flags.go
package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

// ErrHelp is returned when the user asked for --help.
var ErrHelp = errors.New("help requested")

// ErrVersion is returned when the user asked for --version.
var ErrVersion = errors.New("version requested")

// applyFlags binds CLI flags into cfg. It returns ErrHelp/ErrVersion as
// sentinels so the caller can exit with code 0 cleanly.
func applyFlags(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("teranode-acceptance", flag.ContinueOnError)
	fs.SetOutput(&strings.Builder{}) // suppress flag.PrintDefaults default error output

	var (
		configPath, only, skip, reportJSON, reportHTML, overrides string
		verbose, short, allowMainnetLoad, strict, helpReq, verReq bool
		testTimeout                                               time.Duration
	)
	fs.StringVar(&configPath, "config", "config.yaml", "path to YAML config")
	fs.StringVar(&only, "only", "", "comma-separated test IDs to run")
	fs.StringVar(&skip, "skip", "", "comma-separated test IDs to skip")
	fs.StringVar(&reportJSON, "report-json", "report.json", "JSON report output path")
	fs.StringVar(&reportHTML, "report-html", "report.html", "HTML report output path")
	fs.StringVar(&overrides, "reviewer-overrides", "", "reviewer overrides YAML (optional)")
	fs.BoolVar(&verbose, "verbose", false, "enable slog JSON streaming output")
	fs.BoolVar(&short, "short", false, "shorten long-running observations")
	fs.BoolVar(&allowMainnetLoad, "allow-mainnet-load", false, "permit load-generating tests on mainnet")
	fs.BoolVar(&strict, "strict-config", false, "require all endpoint URLs even if no test needs them")
	fs.DurationVar(&testTimeout, "test-timeout", 30*time.Minute, "per-test hard timeout")
	fs.BoolVar(&helpReq, "help", false, "print help and exit")
	fs.BoolVar(&verReq, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	if helpReq {
		return ErrHelp
	}
	if verReq {
		return ErrVersion
	}

	cfg.ConfigPath = configPath
	if only != "" {
		cfg.Only = splitCSV(only)
	}
	if skip != "" {
		cfg.Skip = splitCSV(skip)
	}
	cfg.ReportJSON = reportJSON
	cfg.ReportHTML = reportHTML
	cfg.ReviewerOverrides = overrides
	cfg.Verbose = verbose
	cfg.Short = short
	cfg.AllowMainnetLoad = allowMainnetLoad
	cfg.StrictConfig = strict
	cfg.TestTimeout = testTimeout
	return nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
