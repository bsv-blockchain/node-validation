// Package testrunner owns the per-test execution model: the Env injected
// into every test, the Suite that registers and dispatches tests, the
// Result type each test returns, and the verdict / report builder run
// once all tests have completed.
package testrunner

import (
	"log/slog"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/svnode"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

// Status is the outcome of a single test.
type Status string

const (
	StatusPass                Status = "PASS"
	StatusFail                Status = "FAIL"
	StatusSkipped             Status = "SKIPPED"
	StatusError               Status = "ERROR"
	StatusFeatureNotAvailable Status = "FEATURE_NOT_AVAILABLE"
	StatusDeferred            Status = "DEFERRED"
	StatusNotRun              Status = "NOT_RUN"
)

// Check is one acceptance-criterion check inside a Result.
type Check struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Pass        bool   `json:"pass"`
	Detail      string `json:"detail,omitempty"`
}

// Result is what a test function returns.
type Result struct {
	ID                    string          `json:"id"`
	Title                 string          `json:"title"`
	Severity              matrix.Severity `json:"severity"`
	Status                Status          `json:"status"`
	StartedAt             time.Time       `json:"started_at"`
	Duration              time.Duration   `json:"duration_ns"`
	AcceptanceChecks      []Check         `json:"acceptance_checks,omitempty"`
	Observations          map[string]any  `json:"observations,omitempty"`
	PartialEvidence       bool            `json:"partial_evidence,omitempty"`
	SkipReason            string          `json:"skip_reason,omitempty"`
	Err                   string          `json:"err,omitempty"`
	CapturedRisks         []string        `json:"captured_risks,omitempty"`
	SatisfiesRequirements []string        `json:"satisfies_requirements,omitempty"`
}

// Env is the per-run environment passed into every test.
type Env struct {
	Cfg      config.Config
	Logger   *slog.Logger
	Now      func() time.Time
	Manifest matrix.Manifest

	Teranode *teranode.Clients
	SVNode   *svnode.Clients
	TxGen    *txgen.Funder
}

// NewEnv constructs an Env with sane defaults.
func NewEnv(cfg config.Config, logger *slog.Logger, m matrix.Manifest, now func() time.Time) *Env {
	if now == nil {
		now = time.Now
	}
	return &Env{
		Cfg:      cfg,
		Logger:   logger,
		Now:      now,
		Manifest: m,
	}
}
