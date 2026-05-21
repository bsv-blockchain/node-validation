# Teranode Acceptance Run Findings

Date: 2026-05-07
Run mode: SHORT (partial evidence)
Verdict: NO_GO (exit code 1)
Primary blocker: Critical test PC-1 reported FAIL

## Executive Summary

The run shows a mix of:
- Real interoperability/performance risk signals under current environment conditions.
- Harness/configuration issues that likely inflated FAIL/ERROR outcomes.

Most important observation: INTER-1 timeout in short mode appears structurally caused by timeout configuration mismatch, not necessarily protocol failure.

## Prioritized Findings

### 1. Critical: INTER-1 timeout is expected with current short-mode settings

What was observed:
- INTER-1 reported ERROR with reason timed out.

Why this likely happened:
- Short mode sets inter1_observation to 1h.
- INTER-1 observe phase runs for 80% of that window (~48m).
- Per-test timeout default is 30m.

Impact:
- INTER-1 can fail as ERROR before finishing by configuration alone.

Recommended fix:
- Run with --test-timeout 90m (or higher), or reduce short-mode INTER-1 observation duration.

### 2. Critical: PC-1 may produce false mismatches in tx validity comparison

What was observed:
- PC-1 failed, and this was the critical NO_GO trigger.

Why this may be partially harness-related:
- PC-1 submits the same tx to Teranode first, then SV Node.
- The second backend may return already-known semantics.
- Current category mapping can classify this as CONFLICTING on one side vs ACCEPTED on the other, reducing the match ratio.

Impact:
- Healthy propagation can still fail the >=95% tx-agreement criterion.

Recommended fix:
- Normalize duplicate-known outcomes as compatible with ACCEPTED for cross-backend comparison, or adjust submission strategy.

### 3. Critical: PC-3 has a required-fail gate that contradicts stated intent

What was observed:
- PC-3 reported FAIL.

Why this may be partially harness-related:
- Code comment states mempool propagation miss should not fail whole test.
- Implementation still adds a required failing check when propagation is not fully seen.

Impact:
- Transient propagation lag can force overall FAIL even if format/round-trip checks are otherwise valid.

Recommended fix:
- Make that propagation check non-required (observational), or convert to skip/deferred gating when environment is laggy.

### 4. Important: Multiple ERROR outcomes are likely transient infra/harness sensitivity

What was observed:
- PERF-1, CLIENT-2, NEW-FR7, NEW-FR11, NEW-NFR7 returned ERROR.

Likely causes:
- Early hard returns on bootstrap/setup RPC conditions.
- Fresh regtest state sensitivity.
- Splitter and mempool timing behaviors under local Docker load.

Impact:
- Loss of diagnostic resolution (ERROR instead of actionable FAIL/SKIP with criterion details).

Recommended fix:
- Add retries and clearer classification paths so transient setup faults become fail-with-detail or skip-with-reason where appropriate.

### 5. Advisory robustness gap: NEW-FR9 precondition does not guard REST dependency

What was observed:
- NEW-FR9 failed (not ERROR in this run), but there is still a code robustness gap.

Why this matters:
- Precheck validates RPC/P2PWS/TxGen/SVNode but not REST.
- Later logic uses REST to inspect mined block.

Impact:
- In some configurations this can panic or error unexpectedly.

Recommended fix:
- Add REST client presence check to NEW-FR9 preconditions.

## Test-by-Test Interpretation of This Run

- PC-1 FAIL:
  - Could be genuine divergence/reorg convergence issue.
  - Could also be amplified by duplicate-submission category mismatch logic.
- PC-3 FAIL:
  - Could be genuine format/propagation issue.
  - May be inflated by required mempool propagation gate behavior.
- INTER-1 ERROR timed out:
  - Very likely config-timeout mismatch.
- INTER-2 FAIL:
  - Likely a real propagation/mesh issue under strict 99% in 10s target.
- PERF-1 ERROR:
  - Likely setup/load sensitivity in local environment, not necessarily requirement failure.
- CLIENT-2 ERROR:
  - Likely setup/build/submit path fragility.
- NEW-FR7 ERROR:
  - Likely setup/mining/parse path fragility.
- NEW-FR11 ERROR:
  - Likely chain submit/setup path fragility.
- NEW-NFR7 ERROR:
  - Likely low-height/anchor or RPC-state sensitivity.

## Immediate Operational Actions (No Code Changes)

1. Re-run with extended timeout:
- Use --short --test-timeout 90m --config config.docker.yaml

2. Re-run failing tests in isolation:
- PC-1, PC-3, INTER-1, INTER-2, PERF-1, CLIENT-2, NEW-FR7, NEW-FR11, NEW-NFR7

3. Verify network mesh and endpoint assumptions:
- Confirm legacy peering overlays for INTER-2.
- Confirm websocket and service exposure assumptions used by client tests.

## Recommended Harness Changes

1. Align short mode with timeout behavior
- Prevent INTER-1 default timeout conflicts in short mode.

2. Harden cross-backend rejection categorization
- Treat duplicate-known acceptance semantics consistently in PC-1 comparisons.

3. Fix PC-3 propagation gate severity
- Make non-critical propagation lag observational, not required-fail.

4. Improve transient fault handling in setup-heavy tests
- Add bounded retries and clear SKIP/FAIL classification for known transient conditions.

5. Add missing precondition checks
- NEW-FR9 should validate REST dependency before use.

## Bottom Line

Current NO_GO is justified by critical FAIL/ERROR outcomes, but the run likely overstates product risk due to identifiable harness/config issues. Prioritize timeout/config alignment and two key harness fixes (PC-1 comparison normalization and PC-3 propagation gate) before using this suite run as final product evidence.
