# SP3 — Backend Clients code review

**Reviewer:** superpowers:code-reviewer (automated, Opus 4.7)
**Date:** 2026-04-29
**Scope:** the 10 SP3 commits between `sp2-complete` and `3533551 chore(sp3): add definition-of-done check`.
**Specs reviewed:** `docs/superpowers/specs/2026-04-29-sp3-backend-clients-design.md`, `docs/superpowers/plans/2026-04-29-sp3-backend-clients.md`.
**Verification:** `make build lint test verify` exits 0; `scripts/sp{1,2,3}-done-check.sh` all exit 0; `go vet`, `gofmt -l` clean; `go test -race ./...` green; SP3 package coverage 79.5/80.6/100/82.9% (teranode/svnode/compare/jsonrpc), total ≥80% per the done-check.

---

## Recommendation

**APPROVE_WITH_MINOR.** All 10 critical invariants in the review brief are satisfied, every §4 sub-section of the design spec has a concrete implementation with httptest / faked-transport unit tests, no live network calls, and the toolchain is clean. The two issues worth fixing before SP5 starts are both small: a P2P-probe wiring bug that aliases the legacy and libp2p addresses to the same config field, and `// indirect` annotations on the two new direct dependencies in `go.mod`. Neither blocks the SP3 closeout — both have safe fixes.

---

## Critical issues

None. All 10 invariants in the review brief hold:

1. All six Teranode sub-clients exist with httptest tests at the expected paths.
2. Both SV Node clients exist (`rpc.go`, `zmq.go`).
3. `internal/compare/chainstate.go` correctly distinguishes `CategoryUTXOSpent` (Teranode 70) from `CategoryConflicting` (Teranode 32/36, SV Node -27, or -26 with double-spend message), returns `CategoryAccepted` on nil, and `CategoryRPCError` on non-RPC errors. The dedicated `TestCompareCategories` even pins down the UTXO_SPENT vs CONFLICTING distinction.
4. `internal/jsonrpc` is small (103 LoC), well-documented, and used by both `teranode.RPCClient` and `svnode.RPCClient` via a shared `Caller` value.
5. Empty config URL → nil sub-client. Every constructor has a `Test*_NilOnEmptyURL` covering this.
6. `make build lint test verify` exits 0; SP1/SP2/SP3 done-checks all pass; total SP3-package coverage ≥80%.
7. `Env.Teranode` and `Env.SVNode` are the concrete `*teranode.Clients` and `*svnode.Clients` types in `internal/testrunner/types.go`, not interfaces. `NewEnv` keeps its `(cfg, logger, manifest, now)` signature.
8. No live network in any test. ZMQ tests use an in-process `zmq4.NewPub`. The notifications test exercises only the dispatch logic. The P2P probe test stands up an `httptest`-style `net.Listener` that speaks the wire protocol back.
9. `go.mod` adds `github.com/centrifugal/centrifuge-go` and `github.com/go-zeromq/zmq4`. Both are documented in the SP3 spec §5 / §10.
10. `p2p_probe.go:134` sets the user-agent to `"/tng-acceptance-bsv:0.1.0/"` — contains "BSV" (uppercase), so the upstream `peer_server.go:541-549` ban won't fire.

## Important issues

### I-1. `Teranode.NewClients` aliases legacy and libp2p addresses

`internal/teranode/clients.go:44-47`:

```go
var p2p *P2PProbe
if cfg.P2PAddress != "" {
    p2p = NewP2PProbe(cfg.P2PAddress, cfg.P2PAddress /* libp2p — caller may override */, logger)
}
```

`config.Teranode` only has `P2PAddress`. The two probes target very different ports (legacy 8333/18333, libp2p 9905), so plugging the same value into both means at most one of `LegacyHandshake` and `Libp2pPortOpen` can ever succeed in production. The "caller may override" comment doesn't help — there's no setter, and `Clients` exports `P2PProbe` as a value the caller can reach into, but nothing in `cmd/teranode-acceptance/main.go` does that. SP9's INTER tests will hit this.

**Fix:** add a second config field (e.g. `P2PLibp2pAddress` / `p2p_libp2p_address`) and pass both into `NewP2PProbe`. Either:
- bake the field into SP3 now, with a `// SP9` TODO on the test side to populate it; or
- defer to SP9 and add an issue/note tracking it.

The design spec §4.4 explicitly described both surfaces but the SP1 config schema only had one. The implementation took the path of least resistance; the cost is a latent bug.

### I-2. `go.mod` lists the two new direct deps as `// indirect`

```
require (
    github.com/centrifugal/centrifuge-go v0.10.4 // indirect
    github.com/go-zeromq/zmq4 v0.17.0// indirect
    ...
)
```

Both are imported directly from non-test product code (`internal/teranode/notifications.go`, `internal/svnode/zmq.go`). Running `go mod tidy` correctly promotes them to a `require (...)` block of direct deps. The current state is what `go get` produces during the first commit when no consumer existed yet — `go mod tidy` just wasn't re-run after the consumers landed.

**Fix:** `go mod tidy`. The diff is mechanical and harmless. Worth committing as a separate housekeeping commit so the SP3 dep-list inventory matches reality before SP10's README regen step.

### I-3. `url.Parse` validation is a no-op

`teranode/rpc.go:29`, `teranode/metrics.go:28`, `svnode/rpc.go:35` all do `if _, err := url.Parse(rawURL); err != nil { ... }`, but `url.Parse` is permissive: literal garbage like `"not a real url at all"` produces no error. So these guards never fire. Either drop them (config validation in SP1 already runs `url.Parse` for the same URLs and the check there is equally weak), or upgrade to `url.ParseRequestURI` which actually rejects non-absolute URLs. Cosmetic — the eventual HTTP `Do` call will surface a useful error anyway.

## Minor / nits

- **`teranode/clients.go:23` `NewClients`** never builds a P2P probe when only `P2PAddress` is set but at least one of `legacy/libp2p` would've made sense — see I-1.
- **`teranode/notifications.go` `Connect` and `Close` are 0% covered** in `cov.out`. The dispatch logic is well-tested but the actual centrifuge wire-up isn't. Acceptable per the spec ("dispatch tests only") but worth a follow-up if SP5 hits the real WS endpoint and finds surprises.
- **`teranode/sha256.go`** is 6 lines that just delegate to `crypto/sha256.Sum256`. The two-level indirection (`doubleSHA256` → `sha256Sum` → `shaSum` → `sha256.Sum256`) buys nothing. Inline `sha256.Sum256` directly into `doubleSHA256`.
- **`teranode/rest.go:50` `truncate`** is duplicated logic with `health.go:79` (which calls it). It's a single-package private helper, fine, but `health.go` reaches across files for it — gofmt won't move it but a future refactor should.
- **`teranode/notifications.go:94` `Close()`** sets `closed = true` but the field is never read. The dispatch goroutine doesn't check it. If `Close()` is called while the centrifuge connection is still pumping events, the channels stay open. Pragmatic for a simple test client — flag for SP5 if reconnect/teardown semantics matter.
- **`teranode/p2p_probe.go:200-205`** `doubleSHA256 → sha256Sum → shaSum`: same as above — three trampolines for one stdlib call.
- **`svnode/zmq.go:104-160` pump goroutines** read `closed` under lock then call `Recv` outside the lock; once `Close` runs, the in-flight `Recv` is what eventually unblocks the goroutine. Works fine, just worth a doc comment so the next reader doesn't think there's a race window.
- **`compare/chainstate.go:93` `categorizeRejectionMessage`** correctly notes the ordering between non-standard/non-mandatory and the broader script/verify-flag matcher (a real bug if reordered). Keep it.
- **`teranode/notifications.go:44` `centrifuge.Config{}`** uses defaults; no token / connect-data / read-buffer override. Fine for SP3 (we never connect). Worth revisiting in SP5 if the upstream demands a token.
- **No doc comment on exported `RejectionCategory` constants** — minor; the type doc covers the intent.
- **`teranode/metrics.go:50`** ignores the error from `http.NewRequestWithContext`. The signature can't fail with a fixed URL + GET, but it's still a lint smell.

## Strengths

- Constructor signatures are uniform across all eight clients: `(rawURL, [auth/extra], logger) → (*Client, error)`, and the empty-URL → `(nil, nil)` rule is followed everywhere with a dedicated test per constructor.
- The shared `internal/jsonrpc` package is genuinely small and reused. `IsErrorCode` is a nice ergonomic helper for the compare table.
- Errors are wrapped with `fmt.Errorf("...: %w", err)` and carry context (URL, method, status, path, command) — `errors.As` works for `*jsonrpc.Error` and `*RESTError` and is exercised in tests.
- The Prometheus parser is exactly what the spec promised: ~150 LoC, handles HELP/TYPE/labels/values, drops exemplars and OpenMetrics extensions, with a fixture (`testdata/sample.prom`) covering the histogram label-list case.
- The compare/chainstate table is the most thorough piece in the diff — table tests cover every code path including the message-substring branches, and the explicit `TestCompareCategories` test pins the load-bearing UTXO_SPENT vs CONFLICTING distinction with a comment block explaining why they must NOT collapse.
- The P2P probe test stands up a real listener and runs the version/verack exchange both directions in process. Hand-rolled wire serialisation with the right header layout and the user-agent containing "BSV" — both spec callouts honoured.
- Health probe correctly tolerates the `text/plain` Content-Type quirk. The `AllOK` helper is tested for both top-level and per-service failure modes.
- ZMQ round-trip tests use real PUB/SUB sockets in-process — not mocks of the library — which gives strong confidence the wire-format frame layout is right.
- Coverage targets met for every package, with thoughtful skips on the centrifuge-Connect path that can't be exercised without a real server.

## Spec coverage gaps

None. Every §4 sub-section maps to a file:

| Spec | Implementation | Notes |
|---|---|---|
| §4.1 RPC client + 12 convenience | `teranode/rpc.go` | All 12 methods present (`Call` + 11 convenience). |
| §4.2 REST + 10+ endpoints + RESTError | `teranode/rest.go` | 10 endpoint methods, `RESTError` with `errors.As` test. |
| §4.3 Notifications | `teranode/notifications.go` | Connect/Close/Blocks/Subtrees/NodeStatus channels; dispatch tested. |
| §4.4 P2P probe (Legacy + Libp2p) | `teranode/p2p_probe.go` | Both methods; magic bytes for 4 networks; UA contains "BSV". (See I-1.) |
| §4.5 Metrics + 3 helpers | `teranode/metrics.go` | Scrape + BestBlockHeight + FSMState + CatchupActive. |
| §4.6 Health (Readiness, Liveness, AllOK) | `teranode/health.go` | All three; quirky Content-Type handled. |
| §4.7 teranode.Clients aggregator | `teranode/clients.go` | Six fields, nil-safe. |
| §4.8 SV Node RPC + bitcoind methods + cookie auth | `svnode/rpc.go` | TestMempoolAccept, EstimateFee, GetMempoolInfo present; `readCookie` covers the bitcoind paths. |
| §4.9 SV Node ZMQ | `svnode/zmq.go` | PUB/SUB; BlockNotification + TxNotification channels with round-trip tests. |
| §4.10 compare/chainstate | `compare/chainstate.go` | CategorizeTeranode, CategorizeSVNode, CompareCategories, full enum. |

## Plan-deviation analysis

Two intentional deviations, both fine:

- **Plan said `go mod tidy` would drop `nhooyr.io/websocket`.** The current `go.mod` has no `nhooyr.io/websocket` entry — the dep was already removed cleanly. No problem.
- **Plan §4.4 expected separate `legacyAddr` and `libp2pAddr` config fields.** Implementation collapsed them onto `cfg.P2PAddress`. This is the I-1 issue.

Everything else lines up with the plan: file paths, constructor signatures, the shared `jsonrpc` package, the cookie-auth fallback in svnode, the hand-rolled Prometheus parser, the per-package coverage targets.

## Practical sanity checklist

| Check | Result |
|---|---|
| `make build lint test verify` exit 0 | yes |
| `scripts/sp1-done-check.sh` | passes |
| `scripts/sp2-done-check.sh` | passes |
| `scripts/sp3-done-check.sh` | passes |
| `go test -race ./...` | passes |
| `gofmt -l .` | empty |
| `go vet ./...` | clean |
| TODO / FIXME strings | none in SP3 code |
| Doc comments on exported types/funcs | yes, comprehensive |

## Suggested follow-ups (not blocking SP3 closeout)

1. Run `go mod tidy`, commit as `chore(sp3): tidy go.mod direct-vs-indirect classification`.
2. Add `Teranode.P2PLibp2pAddress` config field + wire into `clients.go:NewClients`. Either as a small SP3 addendum commit or a dedicated SP9-prep commit; tag in `cfg.P2PAddress` doc to mention it's the legacy address.
3. Replace `url.Parse` validation with `url.ParseRequestURI` or drop the check entirely. One-liner each.
4. Inline the `sha256.Sum256` indirection in `teranode/sha256.go` and `p2p_probe.go`. Pure cosmetics.
