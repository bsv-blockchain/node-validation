# ADR-005: Add Missing REST Precondition Guard in NEW-FR9

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

NEW-FR9 tests double-spend detection. Its precondition block checks for
`Teranode.RPC`, `Teranode.P2PWS`, `TxGen`, and `SVNode`, but not for
`Teranode.REST`:

```go
// tests/new_fr9.go:50–52
if env.Teranode == nil || env.Teranode.RPC == nil ||
    env.Teranode.P2PWS == nil || env.TxGen == nil || env.SVNode == nil {
    return skipMissing(res, "Teranode RPC, P2PWS, TxGen, or SVNode not configured")
}
```

Later in the test, `env.Teranode.REST` is dereferenced unconditionally to
fetch the mined block for tx-winner verification:

```go
// tests/new_fr9.go:168
blockBytes, err := env.Teranode.REST.GetBlockLegacyBytes(ctx, mined[0])
```

If `env.Teranode.REST` is `nil` — which happens when the REST URL is absent
from the config — the test bypasses the precondition guard and panics on the
nil dereference, or the suite runner catches the panic and reports
`StatusError`. The error message gives no indication that REST is the missing
dependency, making the failure hard to diagnose.

---

## Decision

Add `env.Teranode.REST == nil` to the precondition guard and update the skip
message to include REST in the list of required dependencies.

---

## Consequences

- In configurations where REST is not configured, NEW-FR9 returns `SKIPPED`
  with a clear "REST not configured" message instead of `ERROR`/panic.
- The test's double-spend detection checks (RPC rejection, P2PWS notification)
  can run independently of block inspection once REST is available.
- No behavioural change in fully-configured environments.

---

## Implementation

Change in `tests/new_fr9.go` at the precondition guard (lines 50–52):

```diff
-if env.Teranode == nil || env.Teranode.RPC == nil ||
-    env.Teranode.P2PWS == nil || env.TxGen == nil || env.SVNode == nil {
-    return skipMissing(res, "Teranode RPC, P2PWS, TxGen, or SVNode not configured")
+if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil ||
+    env.Teranode.P2PWS == nil || env.TxGen == nil || env.SVNode == nil {
+    return skipMissing(res, "Teranode RPC, REST, P2PWS, TxGen, or SVNode not configured")
 }
```
