# ADR-006: Harden Error Handling in Setup-Sensitive Tests

**Date**: 2026-05-21
**Status**: Accepted
**Deciders**: Martyn Harler

---

## Context

Several tests return `StatusError` for conditions that are either known
transient environment states or well-understood environmental preconditions,
not product defects.

### NEW-NFR7 — Low chain height on fresh regtest

NEW-NFR7 anchors its determinism checks at `height = tip - iterations - 10`.
On a fresh Docker regtest environment the tip may be very low (e.g. 5–20
blocks). The anchor calculation produces a value ≤ 0, and the current code
returns `errorResult`:

```go
// tests/new_nfr7.go:62–79
info, err := env.Teranode.RPC.GetBlockchainInfo(ctx)
if err != nil {
    return errorResult(res, fmt.Errorf("getblockchaininfo: %w", err))
}
anchorHeight := int64(info.Blocks) - int64(iterations) - 10
if anchorHeight < 1 {
    anchorHeight = 1    // silently clips; but this also makes the test
}                       // use block 1 regardless, which may have unstable
                        // "confirmations" in verbose responses
```

Actually the current code silently clips to 1 rather than erroring, but at
very low heights the baseline block-hash and block-hex captures at height 1
may behave unexpectedly on a chain still being built. The correct action for
a chain that is too short is `SKIP` with a clear message.

### CLIENT-2, NEW-FR7, NEW-FR11 — Bootstrap FAIL_FORBIDDEN not converted to SKIP

Commits `3ad0064` added FAIL_FORBIDDEN→SKIP handling for PERF-1 and INTER-2
splitter submissions. The same Aerospike lock-contention pattern can affect
the `bootstrapConfirmed` calls in CLIENT-2, NEW-FR7, and NEW-FR11. When that
happens, these tests return `errorResult("bootstrap: ...")` rather than
`SKIPPED` with an actionable reason, making the result harder to triage.

---

## Decision

1. **NEW-NFR7**: Add an explicit minimum-chain-height guard. If
   `anchorHeight < 1` (chain has fewer blocks than `iterations + 11`),
   return `skipMissing` with a message indicating the required minimum.

2. **CLIENT-2, NEW-FR7, NEW-FR11**: After each `bootstrapConfirmed` call,
   check whether the returned error contains `"FAIL_FORBIDDEN"` and convert
   that to `skipMissing` (consistent with PERF-1 and INTER-2).

---

## Consequences

- `StatusError` outcomes caused by known environment conditions become
  `StatusSkipped` with actionable messages. Operators can distinguish
  "environment not ready" from "product defect".
- Genuine bootstrap failures (insufficient funds, mining RPC failure) still
  produce `StatusError`.
- No change to the pass/fail logic once setup succeeds.

---

## Implementation

### NEW-NFR7 — minimum chain height guard (`tests/new_nfr7.go`)

```diff
 anchorHeight := int64(info.Blocks) - int64(iterations) - 10
 if anchorHeight < 1 {
-    anchorHeight = 1
+    return skipMissing(res, fmt.Sprintf(
+        "chain too short for determinism test: need at least %d blocks, have %d",
+        int64(iterations)+11, info.Blocks,
+    ))
 }
```

### CLIENT-2 — bootstrap FAIL_FORBIDDEN→SKIP (`tests/client2.go`)

```diff
 if funder.Balance() < 100_000_000 {
     if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
+        if strings.Contains(err.Error(), "FAIL_FORBIDDEN") {
+            return skipMissing(res, "bootstrap: Aerospike lock contention: "+err.Error())
+        }
         return errorResult(res, fmt.Errorf("bootstrap: %w", err))
     }
```

### NEW-FR7 — bootstrap FAIL_FORBIDDEN→SKIP (`tests/new_fr7.go`)

```diff
 if funder.Balance() < 100_000_000 {
     if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
+        if strings.Contains(err.Error(), "FAIL_FORBIDDEN") {
+            return skipMissing(res, "bootstrap: Aerospike lock contention: "+err.Error())
+        }
         return errorResult(res, fmt.Errorf("bootstrap: %w", err))
     }
```

### NEW-FR11 — bootstrap FAIL_FORBIDDEN→SKIP (`tests/new_fr11.go`)

```diff
 if funder.Balance() < 100_000_000 {
     if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
+        if strings.Contains(err.Error(), "FAIL_FORBIDDEN") {
+            return skipMissing(res, "bootstrap: Aerospike lock contention: "+err.Error())
+        }
         return errorResult(res, err)
     }
```
