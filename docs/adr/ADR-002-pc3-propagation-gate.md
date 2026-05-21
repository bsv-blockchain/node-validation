# ADR-002: Fix PC-3 Mempool Propagation Check Severity

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

PC-3 tests wire-format round-trip and block parsing. One step waits for
three submitted transactions to appear in SVNode-1's mempool before mining:

```go
// tests/pc3.go:132–141
if err := waitForMempoolEntries(ctx, env.SVNode.RPC, []string{...}, 30*time.Second); err != nil {
    // Don't fail the whole test — record as acceptance check.
    res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
        "All 3 test txs propagated to svnode-1 mempool",
        err.Error(),
    ))
}
```

The developer comment explicitly states the propagation miss **should not fail
the whole test**. However, `fail()` (`tests/helper.go:72–74`) returns a
`testrunner.Check` with `Required: true, Pass: false`, and `deriveStatus`
(`tests/helper.go:135–145`) returns `StatusFail` when any required check has
`Pass: false`.

The result: a transient P2P relay delay between Teranode and SVNode-1 drives
an overall `FAIL` verdict for PC-3, even when all wire-format and round-trip
acceptance criteria pass. This contradicts both the stated intent in the comment
and the actual objective of the test.

---

## Decision

Replace the `fail(...)` call with an explicit `testrunner.Check` struct where
`Required: false`. This makes the mempool propagation observation non-gating —
it is recorded in the report for visibility but does not affect the overall
`PASS`/`FAIL` verdict.

---

## Consequences

- PC-3 verdict is determined solely by the wire-format and round-trip checks,
  which is the stated objective of the test.
- Transient Teranode→SVNode relay delays no longer produce false `FAIL`
  outcomes for PC-3.
- The propagation observation remains visible in `report.json` under
  `acceptance_checks` with `required: false` so operators can still detect
  persistent relay problems.
- If SVNode-1 mines an empty block because the txs hadn't arrived yet, the
  "Block contains X test tx" required checks will still catch it correctly.

---

## Implementation

Change in `tests/pc3.go` at the propagation check (lines 132–141):

```diff
 	if err := waitForMempoolEntries(ctx, env.SVNode.RPC, []string{
 		hex.EncodeToString(bres.TxID[:]),
 		hex.EncodeToString(bres2.TxID[:]),
 		hex.EncodeToString(bres3.TxID[:]),
 	}, 30*time.Second); err != nil {
-		// Don't fail the whole test — record as acceptance check.
-		res.AcceptanceChecks = append(res.AcceptanceChecks, fail(
-			"All 3 test txs propagated to svnode-1 mempool",
-			err.Error(),
-		))
+		res.AcceptanceChecks = append(res.AcceptanceChecks, testrunner.Check{
+			Description: "All 3 test txs propagated to svnode-1 mempool",
+			Required:    false,
+			Pass:        false,
+			Detail:      err.Error(),
+		})
 	} else {
```
