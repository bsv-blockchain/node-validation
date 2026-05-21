# ADR-004: Fix INTER-2 Propagation Percentage Denominator

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

INTER-2 submits transactions to each backend in isolated groups and then
measures what fraction of the "Teranode-only" group appears in the SVNode
mempool (and vice versa) within `DefaultPropagation` (10 s).

The percentage is computed as:

```go
// tests/inter2.go:256–257
teranodeToSVPct := 100.0 * float64(len(seenSV)) / float64(len(teranodeOnlyTxIDs))
svToTeranodePct := 100.0 * float64(len(seenTN)) / float64(len(svOnlyTxIDs))
```

`teranodeOnlyTxIDs` is built from the **full planned group** before any
submission happens (`tests/inter2.go:191–192`):

```go
teranodeOnlyTxIDs := txidsOf(groupTeranodeOnly)   // all planned, not all submitted
svOnlyTxIDs       := txidsOf(groupSVOnly)
```

`submitGroup` returns only a **count** of successful submissions. A txid that
was never submitted cannot appear in the remote mempool. If any submissions
fail (network blip, Aerospike transient, RPC error), the denominator is
inflated above the number of actually-submitted transactions, and the
propagation percentage is artificially deflated.

With the 99% gate this means a small number of local submission failures
(≥4 out of 333 for the Teranode-only group, or ≥4 out of 333 for the
SV-only group) can alone force `FAIL` with no real cross-mesh propagation
deficit.

---

## Decision

Change `submitGroup` to return the slice of successfully-submitted
`interTx` values instead of a count. Derive `teranodeOnlyTxIDs` and
`svOnlyTxIDs` from the returned slices so that:

1. The poll target only contains txids that were actually sent.
2. The percentage denominator (`len(teranodeOnlyTxIDs)`) equals the
   number of submitted transactions, eliminating the bias.

---

## Consequences

- Propagation percentage reflects cross-mesh relay fidelity, not submission
  success rate.
- A `FAIL` after this fix requires that submitted transactions genuinely
  failed to cross the mesh within the propagation window.
- The `teranode_only_submitted` / `sv_only_submitted` observations in
  `report.json` continue to be recorded so operators can see if submission
  success was lower than expected.
- If all submissions succeed (the normal case), behaviour is identical to
  before.

---

## Implementation

Changes in `tests/inter2.go`:

**1. Change `submitGroup` return type and body:**

```diff
-submitGroup := func(grp []interTx, submit func(context.Context, string) (string, error)) (sent int) {
+// Returns the subset of grp that was successfully submitted.
+submitGroup := func(grp []interTx, submit func(context.Context, string) (string, error)) []interTx {
     var wg sync.WaitGroup
     sem := make(chan struct{}, 10)
     var mu sync.Mutex
+    var submitted []interTx
     for _, t := range grp {
         wg.Add(1)
         sem <- struct{}{}
         go func(tx interTx) {
             defer wg.Done()
             defer func() { <-sem }()
             if _, err := submit(ctx, tx.hex); err == nil {
                 mu.Lock()
-                sent++
+                submitted = append(submitted, tx)
                 mu.Unlock()
             }
         }(t)
     }
     wg.Wait()
-    return sent
+    return submitted
 }
```

**2. Update call sites and derive txid slices from submitted results:**

```diff
-teranodeSent := submitGroup(groupTeranodeOnly, env.Teranode.RPC.SendRawTransaction)
-svSent := submitGroup(groupSVOnly, env.SVNode.RPC.SendRawTransaction)
+submittedTeranodeOnly := submitGroup(groupTeranodeOnly, env.Teranode.RPC.SendRawTransaction)
+submittedSVOnly       := submitGroup(groupSVOnly, env.SVNode.RPC.SendRawTransaction)
+teranodeSent := len(submittedTeranodeOnly)
+svSent       := len(submittedSVOnly)
```

**3. Move txid-slice derivation to after submission:**

Remove the pre-submission derivation:
```diff
-teranodeOnlyTxIDs := txidsOf(groupTeranodeOnly)
-svOnlyTxIDs := txidsOf(groupSVOnly)
```

Add post-submission derivation (after `bothSent` block):
```diff
+teranodeOnlyTxIDs := txidsOf(submittedTeranodeOnly)
+svOnlyTxIDs       := txidsOf(submittedSVOnly)
```
