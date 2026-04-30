// Package tests — NEW-NFR11 implementation.
//
// Source: derived from NFR-11 (no source-plan test case). Severity Advisory.
//
// Objective:
//   Verify Teranode endpoints support the security posture NFR-11 demands.
//
// Method:
//  1. For each configured Teranode endpoint URL (rpc, rest, notifications,
//     metrics, health) plus svnode RPC: resolve scheme; if https/wss attempt
//     TLS handshake and record version + cipher; if http/ws record as a
//     finding (regtest plain transport, production must terminate TLS).
//  2. Probe authentication: try unauthenticated request to a protected
//     endpoint (Teranode RPC); try authenticated; record both outcomes.
//  3. Rate-limit headers (overlap with NEW-NFR13) — not parsed in this test.
//
// Acceptance criteria:
//   - TLS 1.2 or higher negotiated where TLS is in use.
//   - Authenticated endpoint rejects unauthenticated requests.
//   - No mandatory plain-text transport for production-relevant endpoints.
//
// Implementation notes:
//   - In docker regtest all transports are plain HTTP/WS — per spec §9 Q1=A,
//     plain HTTP findings record as Pass with a note explaining production
//     posture, not as a failure.
//   - Auth is exercised by constructing a fresh teranode.RPCClient with empty
//     credentials and confirming it's rejected.

package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/teranode"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func RunNEWNFR11(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "NEW-NFR11", Title: "Transport Security and Authentication Probe",
		Severity:              matrix.SeverityAdvisory,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"NFR-11"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil {
		return skipMissing(res, "Teranode client not configured")
	}

	// (1) URL-by-URL transport probe.
	urls := []struct{ name, raw string }{
		{"teranode.rpc", env.Cfg.Teranode.RPCURL},
		{"teranode.rest", env.Cfg.Teranode.RESTURL},
		{"teranode.notifications", env.Cfg.Teranode.NotificationURL},
		{"teranode.metrics", env.Cfg.Teranode.MetricsURL},
		{"teranode.health", env.Cfg.Teranode.HealthURL},
		{"svnode.rpc", env.Cfg.SVNode.RPCURL},
	}
	for _, u := range urls {
		if u.raw == "" {
			continue
		}
		parsed, err := url.Parse(u.raw)
		if err != nil {
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				fmt.Sprintf("[%s] URL parses", u.name),
				err.Error(),
			))
			continue
		}
		switch parsed.Scheme {
		case "https", "wss":
			info, err := probeTLS(ctx, parsed)
			res.AcceptanceChecks = append(res.AcceptanceChecks, required(
				fmt.Sprintf("[%s] TLS handshake succeeded with version >= 1.2", u.name),
				err == nil && info.Version >= tls.VersionTLS12,
				fmt.Sprintf("version=0x%04x cipher=%s err=%v", info.Version, info.Cipher, err),
			))
		case "http", "ws":
			res.AcceptanceChecks = append(res.AcceptanceChecks, ok(
				fmt.Sprintf("[%s] transport scheme is %q", u.name, parsed.Scheme),
				"regtest plain transport — production deployment must terminate TLS in front",
			))
		case "tcp":
			// SVNode ZMQ — not applicable.
		}
	}

	// (2) Auth probe — Teranode RPC requires Basic Auth.
	if env.Teranode.RPC != nil && env.Cfg.Teranode.RPCURL != "" {
		// Fresh client with empty credentials.
		rawNoAuth, err := teranode.NewRPCClient(env.Cfg.Teranode.RPCURL, "", "", env.Logger)
		if err != nil || rawNoAuth == nil {
			res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
				"Construct unauthenticated client for auth probe",
				fmt.Sprintf("err=%v", err),
			))
		} else {
			_, errNoAuth := rawNoAuth.GetBestBlockHash(ctx)
			isUnauthorised := errNoAuth != nil && (strings.Contains(errNoAuth.Error(), "401") ||
				strings.Contains(strings.ToLower(errNoAuth.Error()), "unauthorized"))
			res.AcceptanceChecks = append(res.AcceptanceChecks, required(
				"Teranode RPC rejects unauthenticated requests with 401",
				isUnauthorised,
				fmt.Sprintf("err=%v", errNoAuth),
			))
		}

		_, errAuth := env.Teranode.RPC.GetBestBlockHash(ctx)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			"Teranode RPC accepts authenticated requests",
			errAuth == nil,
			fmt.Sprintf("err=%v", errAuth),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}
