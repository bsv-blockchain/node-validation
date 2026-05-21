# ADR-001: Fix INTER-1 Short-Mode Observation Window

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

In `--short` mode the INTER-1 observation window is set to 1 hour
(`config/short.go: c.Durations.INTER1Observation = time.Hour`). The observe
phase in `tests/inter1.go` consumes the first 80% of that window:

```go
observeUntil := env.Now().Add(window * 4 / 5)   // 48 min at window=1h
```

The per-test runner applies a hard timeout (`config/defaults.go: TestTimeout = 30m`).
Because 48 min > 30 min, the suite runner cancels the test context before the
observe phase completes and marks the result `StatusError` with
`SkipReason: "timed out"` (`internal/testrunner/suite.go:120–128`).

This means INTER-1 **always** produces `ERROR` in short mode regardless of
protocol behaviour. The ERROR is entirely structural — no Teranode evidence can
be read from it.

---

## Decision

Reduce `INTER1Observation` in `applyShort` from `1*time.Hour` to
`20*time.Minute`.

With `window = 20m`:
- Observe phase: 80% × 20 min = **16 min** — well within the 30 min timeout.
- Reorg phase: 20% × 20 min = 4 min.
- Total budget including setup headroom: ~22 min. Fits within `TestTimeout = 30m`.

The short-mode intent is still satisfied: a reduced-duration run exercises the
observe/reorg mechanics without running for hours.

---

## Consequences

- INTER-1 in `--short` mode will complete within the existing 30 min
  `TestTimeout` and produce a meaningful `PASS`, `FAIL`, or `SKIP` rather
  than a structural `ERROR`.
- Observation coverage is reduced from 48 min to 16 min, which is the
  accepted trade-off for short mode.
- No changes to default (non-short) mode behaviour; the full 336 h observation
  window is unchanged.
- Operators who previously saw `INTER-1 ERROR timed out` in short runs will
  now see a substantive result on the next run.

---

## Implementation

Change in `config/short.go`:

```diff
-c.Durations.INTER1Observation = time.Hour
+c.Durations.INTER1Observation = 20 * time.Minute
```
