// Package tests — NEW-NFR13 implementation.
//
// Source: derived from NFR-13. Severity Advisory.
//
// Objective:
//   Verify documented rate limits exist, are consistently enforced, and
//   that 429 / equivalent responses include retry guidance.
//
// Method:
//  1. Issue probe requests at maxRate against getbestblockhash for duration
//     time, OR until the server returns a rate-limit response.
//  2. On first rate-limit response: record status, retry-after header (best
//     effort), body. Wait briefly; verify normal service resumes.
//  3. Report observed limit (or "no_limit_reached").
//
// Acceptance criteria:
//   - Probing exposes a limit OR documented ceiling reached without one.
//   - If a limit is hit, response includes retry-after-style guidance (best
//     effort given the current RPC client's error type).
//   - Service resumes after the retry period.
//
// Implementation notes:
//   - Configured via Cfg.Limits.NFR13MaxProbeRate (default 1000 req/s) and
//     NFR13ProbeDuration (default 5s). 0 in either disables.
//   - The current RPC client doesn't surface HTTP retry-after headers; this
//     is a known limitation captured by SP10. The test asserts service
//     resumes after a 2-second sleep instead.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWNFR13(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-NFR13", Title: "Rate Limit Discovery and Error Semantics",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-13"},
		Observations:          map[string]any{},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.RPC == nil {
		return skipMissing(res, "Teranode RPC not configured")
	}
	maxRate := env.Cfg.Limits.NFR13MaxProbeRate
	duration := env.Cfg.Limits.NFR13ProbeDuration
	if maxRate <= 0 || duration <= 0 {
		return skipMissing(res, "rate-limit probe disabled in config")
	}

	deadline := env.Now().Add(duration)
	interval := time.Second / time.Duration(maxRate)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}

	var (
		sent        uint64
		succeeded   uint64
		firstStatus int
		firstErr    error
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for env.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return errorResult(res, ctx.Err())
		case <-ticker.C:
			sent++
			_, err := env.Teranode.RPC.GetBestBlockHash(ctx)
			if err == nil {
				succeeded++
				continue
			}
			if status, isLimit := classifyRateLimit(err); isLimit {
				firstStatus = status
				firstErr = err
				goto LimitObserved
			}
			// Other errors — keep probing; might be transient.
		}
	}

LimitObserved:
	res.Observations["sent"] = sent
	res.Observations["succeeded"] = succeeded
	res.Observations["max_rate_req_per_s"] = maxRate
	res.Observations["probe_duration"] = duration.String()

	if firstErr == nil {
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Rate-limit probe completed without hitting a limit",
			fmt.Sprintf("sent=%d succeeded=%d max_rate=%d duration=%v observed=no_limit_reached",
				sent, succeeded, maxRate, duration),
		))
		res.Observations["limit_observed"] = false
	} else {
		res.Observations["limit_observed"] = true
		res.Observations["limit_status"] = firstStatus
		res.Observations["limit_error"] = firstErr.Error()
		res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
			"Rate limit observed during probe",
			fmt.Sprintf("sent=%d succeeded=%d firstStatus=%d firstErr=%v",
				sent, succeeded, firstStatus, firstErr),
		))
		// Service resumes after a brief wait.
		time.Sleep(2 * time.Second)
		_, err := env.Teranode.RPC.GetBestBlockHash(ctx)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			"Service resumes after brief wait",
			err == nil,
			fmt.Sprintf("err=%v", err),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
