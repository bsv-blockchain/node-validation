# ADR-003: Fix PC-1 Duplicate-Known Category Mismatch

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

PC-1 submits the same transaction to both backends and compares their
acceptance/rejection categories. The batch submission is sequential:

```go
// tests/pc1.go:204–206
_, terr := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
_, serr := env.SVNode.RPC.SendRawTransaction(ctx, bres.HexTx)
isMatch, _, _ := compare.CompareCategories(terr, serr)
```

The comparison uses strict category equality
(`internal/compare/chainstate.go:113–118`):

```go
func CompareCategories(teranodeErr, svnodeErr error) (matched bool, ...) {
    teranodeCat = CategorizeTeranode(teranodeErr)
    svnodeCat   = CategorizeSVNode(svnodeErr)
    matched     = teranodeCat == svnodeCat   // strict equality only
    return
}
```

SVNode's JSON-RPC error code `-27` is documented as "tx already in chain /
already in mempool" — it is returned when a tx the node already knows is
resubmitted. The current categoriser maps `-27` to `CategoryConflicting`:

```go
// internal/compare/chainstate.go:77–78
case -27:
    return CategoryConflicting // tx already in chain / already in mempool
```

**Failure mode**: When the Teranode submission succeeds (`ACCEPTED`) and
P2P relay delivers the tx to SVNode before the sequential RPC call arrives,
SVNode returns `-27`. `ACCEPTED ≠ CONFLICTING` counts as a mismatch. This
can happen for every healthy relay event within the batch window and pushes
the match rate below the 95% threshold, triggering `FAIL` — the primary
NO_GO trigger in the observed run.

The `-27` code represents "already seen/accepted" semantics, not a true
double-spend conflict. Treating it the same as a conflicting transaction
(code `-26` with "double-spend" in the message) is incorrect.

---

## Decision

1. Introduce a new canonical category `CategoryAlreadyKnown` for SVNode's
   `-27` code, separating the "duplicate-known" case from true conflicts.

2. Update `CompareCategories` to treat `(ACCEPTED, ALREADY_KNOWN)` and
   `(ALREADY_KNOWN, ACCEPTED)` as compatible outcomes. Both represent a
   node that has already processed the transaction successfully — they differ
   only in which side received it first.

This is a targeted fix confined to `internal/compare/chainstate.go`. No
changes to test logic are required.

---

## Consequences

- Healthy duplicate-relay events no longer count as mismatches in PC-1,
  removing the false-negative amplifier.
- True double-spend conflicts (code `-26` with conflict message, or Teranode
  codes 32/36) continue to count as mismatches as before.
- `CategoryAlreadyKnown` appears in `report.json` observation data where
  previously `CONFLICTING` was logged for `-27` rejections.
- Any genuine PC-1 mismatch that remains after this fix is stronger evidence
  of a real protocol divergence.

---

## Implementation

Changes in `internal/compare/chainstate.go`:

```diff
+	CategoryAlreadyKnown  RejectionCategory = "ALREADY_KNOWN"

 // CategorizeSVNode maps an SV Node RPC error to the same canonical category.
 func CategorizeSVNode(err error) RejectionCategory {
     ...
     case -27:
-        return CategoryConflicting // tx already in chain / already in mempool
+        return CategoryAlreadyKnown // tx already in chain / already in mempool (duplicate)
     ...
 }

 func CompareCategories(teranodeErr, svnodeErr error) (matched bool, ...) {
     teranodeCat = CategorizeTeranode(teranodeErr)
     svnodeCat   = CategorizeSVNode(svnodeErr)
-    matched = teranodeCat == svnodeCat
+    matched = teranodeCat == svnodeCat ||
+        // ACCEPTED and ALREADY_KNOWN are compatible: both nodes processed the tx
+        // successfully. Arises when sequential submission lets P2P relay deliver
+        // the tx to the second backend before the RPC call arrives.
+        (teranodeCat == CategoryAccepted && svnodeCat == CategoryAlreadyKnown) ||
+        (teranodeCat == CategoryAlreadyKnown && svnodeCat == CategoryAccepted)
     return
 }
```
