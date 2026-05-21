# Test Failure Root-Cause Analysis

Date: 2026-05-21
Run mode: SHORT (partial evidence)
Analysis type: Repo code + config audit against reported failures

## Purpose

This document provides a structured, evidence-based root-cause analysis of the FAIL and ERROR outcomes
observed in the SHORT-mode acceptance run. Each finding is classified as one of:

- **Repo coding/config issue** — the harness produced a misleading result independent of Teranode behaviour.
- **Mixed** — the harness measurement is biased but a real product signal may remain after correction.
- **Product issue** — evidence points to Teranode or the network mesh as the root cause.

Repo-side issues are tracked by ADR and fixed in this same PR.

---

## Findings (Highest Severity First)

### Finding 1 — INTER-1 ERROR: repo-side timeout misconfiguration (repo coding/config issue)

**Classification**: Repo coding/config issue.

**Observed outcome**: INTER-1 reported `ERROR` with `SkipReason: "timed out"`.

**Evidence chain**:

| Symptom | Source |
|---|---|
| Short mode sets `INTER1Observation = 1h` | `config/short.go:10` |
| Observe phase consumes 80% of observation window | `tests/inter1.go:63` — `observeUntil = env.Now().Add(window * 4 / 5)` |
| Per-test default timeout = 30 min | `config/defaults.go:63` |
| Suite runner hard-cancels test context at timeout; marks result ERROR | `internal/testrunner/suite.go:86–90`, `suite.go:120–128` |

**Calculation**: 80% of 1 h = 48 min observe phase. Default test timeout = 30 min. The observe phase
always exceeds the timeout by 18 min. The ERROR is structural — it requires no product defect.

**Conclusion**: This ERROR can happen by configuration alone. No product evidence can be read from it.

**Fix**: ADR-001 — reduce `INTER1Observation` in short mode to 20 min (observe phase = 16 min, well within the 30 min timeout).

---

### Finding 2 — PC-3 FAIL: required-fail gate contradicts stated intent (repo coding issue)

**Classification**: Repo coding issue.

**Observed outcome**: PC-3 reported `FAIL`.

**Evidence chain**:

| Symptom | Source |
|---|---|
| Code comment: "Don't fail the whole test — record as acceptance check" | `tests/pc3.go:137` |
| Immediately calls `fail()` for mempool propagation miss | `tests/pc3.go:138–141` |
| `fail()` creates a `Check` with `Required: true, Pass: false` | `tests/helper.go:72–74` |
| `deriveStatus` returns `FAIL` when any required check has `Pass: false` | `tests/helper.go:135–145` |

**Contradiction**: The developer comment explicitly states the propagation observation should not fail
the whole test. The implementation does the opposite: `fail()` unconditionally marks the check as
`Required: true`, making a mempool propagation timeout drive an overall `FAIL` verdict regardless
of whether the wire-format and round-trip checks passed.

**Conclusion**: The FAIL outcome may occur with no actual format or wire-protocol problem — transient
P2P relay lag between Teranode and SVNode-1 is sufficient.

**Fix**: ADR-002 — convert the propagation check to `Required: false` (observational).

---

### Finding 3 — PC-1 FAIL: duplicate-known semantics inflate mismatch rate (repo coding issue)

**Classification**: Repo coding issue (likely false-negative amplifier).

**Observed outcome**: PC-1 reported `FAIL` (primary NO_GO trigger).

**Evidence chain**:

| Symptom | Source |
|---|---|
| `submitDeterministicBatch` submits to Teranode first, then SVNode sequentially | `tests/pc1.go:204–205` |
| Category comparison is strict equality | `tests/pc1.go:206` — `compare.CompareCategories(terr, serr)` |
| `nil` error → `CategoryAccepted` for Teranode | `internal/compare/chainstate.go:36–38` |
| SVNode RPC error code `-27` → `CategoryConflicting` (already-in-chain / already-in-mempool) | `internal/compare/chainstate.go:77–78` |

**Failure mode**: When Teranode receives the tx first (returns `ACCEPTED`), P2P relay may deliver the
same tx to SVNode before the sequential RPC call arrives. SVNode then returns code `-27`
("already in mempool"), which maps to `CategoryConflicting`. `ACCEPTED ≠ CONFLICTING` →
counted as a mismatch. This happens for every healthy relay event during the batch window and
can push the mismatch rate above the 5% tolerance.

**Conclusion**: The ≥95% match criterion can fail due to timing alone, with no actual protocol divergence.
The residual mismatch rate after fixing the comparator is the genuine product signal.

**Fix**: ADR-003 — introduce `CategoryAlreadyKnown` for SVNode's `-27` code; treat
`(ACCEPTED, ALREADY_KNOWN)` as compatible in `CompareCategories`.

---

### Finding 4 — INTER-2 FAIL: propagation denominator skew (mixed)

**Classification**: Mixed (repo measurement bias + possible genuine propagation deficit).

**Observed outcome**: INTER-2 reported `FAIL`.

**Evidence chain**:

| Symptom | Source |
|---|---|
| Submission success counts are tracked per-group | `tests/inter2.go:220–221` |
| Propagation poll uses the full planned txid slice as poll target | `tests/inter2.go:253–254` |
| Percentage denominator is the full planned group size, not the submitted count | `tests/inter2.go:256–257` |
| Pass gate: ≥99% within `DefaultPropagation` (10 s default) | `tests/inter2.go:262–271` |

**Failure mode**: If some submission calls fail (network blip, Aerospike transient, rate limit),
`teranodeSent < len(groupTeranodeOnly)`. Txids that were never submitted cannot appear in
the remote mempool. Dividing seen-count by planned-total (not submitted-total) artificially
deflates the propagation percentage. The 99% gate is then failed by local submission drops,
not by a cross-mesh propagation deficit.

**Conclusion**: A portion of FAIL can arise from harness submission drops. Any remaining deficit
after correcting the denominator to use submitted-only txids is stronger evidence of a real
Teranode/mesh propagation issue.

**Fix**: ADR-004 — change `submitGroup` to return the successfully-submitted tx slice; use that
slice as both the poll target and the implicit denominator.

---

### Finding 5 — ERROR-heavy tests: setup/transient hard exits (mostly repo/harness)

**Classification**: Mostly repo/harness robustness issues (not sufficient to call Teranode defects).

**Affected tests**: PERF-1, CLIENT-2, NEW-FR7, NEW-FR11, NEW-NFR7.

**Evidence chain**:

| Test | Hard-error path | Source |
|---|---|---|
| PERF-1 | Bootstrap/splitter/mining errorResult returns | `tests/perf1.go:100–110` |
| CLIENT-2 | Bootstrap errorResult on funding path | `tests/client2.go:60–65` |
| NEW-FR7 | Bootstrap + BuildChain errorResult | `tests/new_fr7.go:63–79` |
| NEW-FR11 | Bootstrap + chain submit errorResult | `tests/new_fr11.go:53–73` |
| NEW-NFR7 | `anchorHeight < 1` produces errorResult on low-chain-height | `tests/new_nfr7.go:62–79` |

Note: PERF-1 and INTER-2 already have FAIL_FORBIDDEN→SKIP handling added in commit `3ad0064`.

**Failure modes**:
- Fresh Docker regtest environments start with very few confirmed blocks; NEW-NFR7's anchor
  height calculation (`tip - iterations - 10`) goes negative and the test hard-errors.
- Bootstrap path can fail transiently with Aerospike lock contention (FAIL_FORBIDDEN) in
  tests that don't yet have the retry+skip guard added in `3ad0064`.

**Conclusion**: These ERROR outcomes are largely harness fragility and environment sensitivity.
They obscure whether the tested features work. The fix is to classify known environment gaps
as SKIP (not ERROR) so the status is actionable.

**Fix**: ADR-006 — add minimum-chain-height guard in NEW-NFR7; propagate FAIL_FORBIDDEN→SKIP
pattern to CLIENT-2, NEW-FR7, and NEW-FR11 bootstrap paths.

---

### Finding 6 — NEW-FR9 robustness gap: missing REST precondition (repo coding issue)

**Classification**: Repo coding issue.

**Evidence chain**:

| Symptom | Source |
|---|---|
| Preconditions check `RPC`, `P2PWS`, `TxGen`, `SVNode` but NOT `REST` | `tests/new_fr9.go:50–52` |
| `env.Teranode.REST.GetBlockLegacyBytes(...)` called unconditionally later | `tests/new_fr9.go:168` |

**Failure mode**: When `env.Teranode.REST` is `nil` (e.g., REST URL not configured),
the test proceeds past preconditions and panics or returns `StatusError` on the nil dereference.
The error message gives no indication that REST is the missing dependency.

**Conclusion**: The precondition guard is incomplete. In configurations without REST, the test
should return `SKIPPED` with a clear reason, not ERROR or panic.

**Fix**: ADR-005 — add `env.Teranode.REST == nil` to the precondition guard.

---

## Per-Failure Verdict Summary

| Test | Outcome | Primary Cause | Residual Product Signal |
|---|---|---|---|
| PC-1 | FAIL | Repo: duplicate-known comparison inflates mismatch | Possible — measure after ADR-003 |
| PC-3 | FAIL | Repo: required-fail gate on optional propagation check | Possible — measure after ADR-002 |
| INTER-1 | ERROR (timed out) | Repo: observation window > test timeout in short mode | None readable from this run |
| INTER-2 | FAIL | Mixed: denominator skew + possible mesh lag | Investigate after ADR-004 |
| PERF-1 | ERROR | Harness: Aerospike FAIL_FORBIDDEN (already fixed in 3ad0064) | Re-run to assess |
| CLIENT-2 | ERROR | Harness: bootstrap path sensitivity | Re-run after ADR-006 |
| NEW-FR7 | ERROR | Harness: bootstrap/chain-build path fragility | Re-run after ADR-006 |
| NEW-FR11 | ERROR | Harness: bootstrap/chain-submit path fragility | Re-run after ADR-006 |
| NEW-NFR7 | ERROR | Harness: low chain height on fresh regtest | Re-run after ADR-006 |
| NEW-FR9 | (robustness gap) | Repo: missing REST precondition guard | Not observable without fix |

---

## ADR Index

| ADR | Issue Fixed | Primary Files |
|---|---|---|
| ADR-001 | INTER-1 short-mode timeout | `config/short.go` |
| ADR-002 | PC-3 propagation gate severity | `tests/pc3.go` |
| ADR-003 | PC-1 duplicate-known mismatch | `internal/compare/chainstate.go` |
| ADR-004 | INTER-2 submission denominator | `tests/inter2.go` |
| ADR-005 | NEW-FR9 missing REST precondition | `tests/new_fr9.go` |
| ADR-006 | Harness error hardening | `tests/new_nfr7.go`, `tests/new_fr7.go`, `tests/new_fr11.go`, `tests/client2.go` |
