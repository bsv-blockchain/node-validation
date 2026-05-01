# SP8 — Notification + Fixture Tests — Code Review

**Reviewer:** senior code reviewer (automated)
**Date:** 2026-04-30
**Spec:** `docs/superpowers/specs/2026-04-30-sp8-notification-fixture-tests-design.md`
**Plan:** `docs/superpowers/plans/2026-04-30-sp8-notification-fixture-tests.md`
**Commits reviewed:** `a208bfe..5bb8311` (5 SP8 commits since `sp7-complete`)
**Diff:** +2533 / -6 across 17 files

## Verdict

APPROVE with minor suggestions. SP8 lands all four Critical tests, the
deterministic fixture generator, ≥30 PC-2 + ≥10 IBD-2 fixtures, the
16-test alphabetical register and the cascading done-check. Static gates
(`make build lint test verify`, `gofmt -l .`, `go vet ./...`,
`staticcheck`, `./scripts/sp8-done-check.sh`, deterministic re-run
`git diff --exit-code`) all pass. Tests follow the SP5/SP6/SP7 shape.
No Critical findings. No TODO/FIXME markers added.

---

## Critical (must fix)

None.

---

## Important (should fix)

### I1. CLIENT-1 leaks a goroutine after the disconnect simulation

`tests/client1.go:89-101` spawns the original block-tail goroutine
selecting on `ctx.Done()` and `notif.Blocks()`. When the mid-window
disconnect runs (`notif.Close()` at line 176), the original `notif`
channel is no longer used, but its goroutine remains parked on a
channel that may never close until `ctx` is cancelled. Then a *second*
goroutine (lines 185-197) is started for the fresh client. Both stay
alive for the duration of the test.

This is leak-equivalent rather than a correctness bug — the parent
context cancellation will eventually reap them, and the only state
they touch is `seenViaSub`/`subCount` under the same mutex. But the
older goroutine could still race-write `subCount` if the underlying
`Close()` does not close `Blocks()`. Suggest:

- Call `notif.Close()` *and* signal the old goroutine to exit (e.g. by
  capturing the original channel via a local `oldCh := notif.Blocks()`
  before reassigning, plus a `done` channel scoped to that goroutine).
- Or document explicitly in `internal/teranode/notifications.go` that
  `Close()` closes the `Blocks()` channel.

Same pattern appears in `tests/client3.go:74-85` and `:156-167` (less
severe because there is no observation loop afterwards — the test
exits shortly after re-subscribing).

### I2. CLIENT-3 reconnect happens *before* the txs have time to confirm

`tests/client3.go` submits all `count` (default 500) txs in a tight
loop (lines 130-143), then immediately does
`notif.Close() ... time.Sleep(2 * time.Second) ... freshNotif.Connect`
(lines 147-168), and only *after* that mines blocks (line 184). The
spec §3.6 calls this a "midpoint reconnection"; in practice the
"midpoint" is at the end of the broadcast burst before any of those
txs are mined. The block-ordering check that follows therefore only
sees the post-reconnect mining, never the splitter-confirmation block.
That weakens the "non-decreasing height order" check to a near-trivial
check on 2 blocks.

Suggest interleaving: send half the txs → close → reconnect → send
the other half → mine → check ordering across the *full* sequence of
heights observed. This better reflects the source plan’s
"controlled-order broadcast … catch-up after reconnect" intent.

### I3. CLIENT-1 disconnect timing is brittle

The mid-window disconnect predicate at `tests/client1.go:175`:

```go
if !disconnected && time.Now().After(deadline.Add(-obs/2)) {
```

This fires only inside the `restTicker` case. With `obs = 5min` and
`restInterval = 30s` we are guaranteed at least one tick after the
midpoint, but with `obs = 1h` and `restInterval = 60s` the window
is ample. However:

- The `time.Sleep(60 * time.Second)` *inside* the `select` case blocks
  the for-select loop, so during those 60s no mining ticks, no broadcast
  ticks and no REST cross-checks occur. That makes the disconnect
  window an effective 60-second hole in the dataset. In `--short` mode
  (5min total) that is a 20% blackout that artifactually inflates
  `restMissedBlocks`.

Move the sleep + reconnect into a goroutine, OR shrink the disconnect
window to ~5–10s (still meaningful for the test), OR use a separate
ticker / state machine.

### I4. `sentMu` lock not held when reading `len(sentTxIDs)` for the early-exit check

`tests/client1.go:136-141` was rewritten from the plan to take the
lock — good — but the `continue` after the unlock then races with the
later `sentMu.Lock()` write at lines 151-153. Functionally fine
(the only consequence is one extra send beyond 50, which is tolerated
by the ≥40-of-50 acceptance check), but worth a comment.

### I5. Spec §3.6 acceptance criterion "≥99% of expected txs" vs implementation `restPct >= 0.99`

`tests/client3.go:200-207` correctly computes `restPct` and compares,
but the float-equality edge case at exactly 0.99 of 500 (495 confirmed)
is fine. Just flagging — no fix needed. Document that the criterion
includes the boundary.

---

## Minor / Suggestions

### M1. Generator code paths could share more

`cmd/gen-fixtures/pc2.go` repeats the `fixture{...}` literal for all
30 entries with constant `Provenance`/`ExpectedCategory` strings. A
small `mkPC2Fixture(id, category, description, hexTx, notes)` helper
would shrink the file by ~150 lines and make adding fixture #31
genuinely "a one-line code change" as the spec promises. Same for
`ibd2.go`'s `provIBD2`.

### M2. `dummyTxID(seed byte)` collisions

The seed namespace is shared across PC-2 categories (0x10..0x55) and
IBD-2 (0x60..0x73). They don't currently collide but the convention
is implicit. Suggest a tiny comment table, or namespace by category
(e.g. seeds 0x10-0x1F = pc2-p2sh, 0x20-0x2F = pc2-restricted, etc.).
Already mostly done — one paragraph in `cmd/gen-fixtures/pc2.go`
header would seal it.

### M3. `nonCanonicalSigWithTrailing` non-canonical-sig fixture creates R/S that might already be canonical

`cmd/gen-fixtures/pc2.go:577-607` builds a 32-byte R/S each starting
`0x01`. The construction sets `r[0] = 0x01` *after* loop-filling,
so technically R is `[0x01, 0x02, 0x03, ..., 0x20]` with high bit
unset → canonical. The "non-canonical" property of fixtures 1 and 2
relies entirely on the trailing garbage bytes after the sighash
flag, not the R/S structure. The current naming is honest; just
making sure the comment matches reality. Consider clarifying.

### M4. CLIENT-1 / CLIENT-3 "reconnect" check weakly verifies catch-up

Both tests just observe whether the fresh client's `Connect()` returns
nil. They don't verify that the new client receives the cached
`node_status` snapshot or that block events resume flowing into
`seenViaSub`/`blockHeights` post-reconnect. If `Connect()` succeeds
but the channel never delivers, the test still passes the reconnect
check. A simple post-reconnect assertion ("at least 1 block event
received via fresh client within 30s of post-disconnect mining")
would harden the test against silent-channel regressions.

### M5. PC-2 doc comment mentions verbatim source-plan but elides numbered bullets

`tests/pc2.go:1-21` and `tests/ibd2.go:1-14` use the standard SP5/6/7
header structure — good — but the "Acceptance criteria" lines drop
the leading bullet's numbering. Consistent with INTER-2; minor.

### M6. `go.mod` is on Go 1.23 — local `min()` helper was correctly omitted

The plan suggested either keeping a local helper or relying on the
1.21+ stdlib builtin. The implementation uses the builtin (good). Drop
the parenthetical "(If Go ≥1.21, the stdlib `min` is available; the
local helper is harmless.)" remark from the plan to avoid future
confusion.

### M7. Fixture YAMLs are human-readable but could include category banners

The two YAML files are 30+10 entries inline. A blank line / comment
banner between categories would help reviewers scanning by hand. Not
required — descriptions and IDs make grouping obvious.

### M8. `TestRegisterTests_SP8RegistersSixteen` could assert alphabetical

The test currently checks count + presence-of-all-IDs but not order.
Since `register.go`'s only documented invariant is "alphabetical",
add `sort.StringsAreSorted(ids)` to the test.

---

## Strengths

- **Determinism is solid.** Running `./bin/gen-fixtures --out
  tests/testdata/` twice produces byte-identical YAML; `git diff
  --exit-code` is clean. The `make verify` extension correctly enforces
  this on CI.
- **Skip-on-missing-fixture tolerance.** `tests/pc2.go:54` and
  `tests/ibd2.go:47` correctly fall back to `StatusSkipped` when the
  fixture file is absent (e.g. running the binary outside the project
  root). This addresses the integration-test fix called out in the
  task brief.
- **Tests follow SP5/6/7 shape exactly.** Source-plan comment block,
  `defer Duration`, `skipMissing`, AcceptanceChecks per criterion,
  `deriveStatus` at end — all consistent across the four new tests.
- **Doc comments on all exported symbols.** `Fixture`, `LoadFixtures`,
  `RunPC2`, `RunIBD2`, `RunCLIENT1`, `RunCLIENT3`, plus generator
  category builders — all carry intent comments.
- **Acceptance-check semantics match the spec.** PC-2 and IBD-2
  measure cross-implementation parity (`compare.CompareCategories`),
  not absolute "valid"-ness — exactly the right framing for synthetic
  dummy-UTXO fixtures.
- **Architectural finding documented.** CLIENT-3 explicitly records
  the "no per-tx Centrifuge events" discovery from SP2 §3 inside the
  test result via an `ok()` check.
- **register_test now asserts 16, not 12 — kept simple.**
- **Done-check cascade is honest.** SP8 invokes SP1–SP7 checks and
  asserts the fixture-count thresholds (≥30, ≥10) directly with
  `grep -c '^- id:'`, plus the determinism gate.
- **Static gates clean.** `gofmt -l .`, `go vet ./...`, `staticcheck
  ./...`, `make build lint test verify`, all unit tests
  (`./tests/...`, `./cmd/gen-fixtures/...`) pass with `-race`.

---

## Spec coverage

| Spec section | Coverage | Notes |
|---|---|---|
| §3.1 Generator design | full | `cmd/gen-fixtures/{main,pc2,ibd2,main_test}.go`; deterministic; `make verify` enforces no-drift |
| §3.2 PC-2 30+ fixtures, 6/category | full | exactly 30, exactly 6 per category |
| §3.3 IBD-2 10+ fixtures | full | exactly 10, one per spec category |
| §3.4 PC-2/IBD-2 test shape | full | matches reference shape; uses `compare.CompareCategories` |
| §3.5 CLIENT-1 (subscribe + 50 broadcast + reconnect) | full | see I1, I3 for goroutine/timing concerns |
| §3.6 CLIENT-3 (500-tx + ordered + reconnect) | partial | see I2 — reconnect lands at end of broadcast burst, not mid-stream |
| §3.7 Reconnect via fresh client (Q4=A) | full | both tests construct `teranode.NewNotificationClient` + `Connect` |
| §4.1 Unit tests | full | `cmd/gen-fixtures/main_test.go` covers count, categories, no-empties, determinism (PC-2 + IBD-2) |
| §4.2 Done-check (static + live SP8_LIVE) | full | static path passes; live path gated by env |
| §4.3 Makefile `gen-fixtures` + `verify` extension | full | both present |
| §5 Definition of done | met | with the I1–I3 caveats above |
| §6 Risks | A,C,F mitigated; D,E documented; G enforced | |

No spec section is unimplemented.

---

## Recommendation

**Land SP8 as-is**, with three follow-ups as small fixes (or rolled
into SP9):

1. **I1** (goroutine cleanup on reconnect) — ~10 lines, no functional
   change but tightens the test against silent regressions.
2. **I2** (interleave CLIENT-3 reconnect mid-burst) — moves the
   reconnect to actually be midpoint; strengthens the block-ordering
   check.
3. **I3** (CLIENT-1 sleep blocks the for-select) — replace the inline
   60s `time.Sleep` with a goroutine or shrink to 5–10s.

The remaining items (M1–M8) are polish.

Tag `sp8-complete` is justified once I1–I3 are addressed or explicitly
deferred to SP9 with a tracking note.
