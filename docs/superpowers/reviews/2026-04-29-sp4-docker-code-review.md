# SP4-DOCKER Code Review

**Reviewer:** Senior Code Reviewer (superpowers:code-reviewer agent)
**Date:** 2026-04-29
**Sub-project:** SP4-DOCKER / Test Environment
**Scope:** 8 commits between `sp4-complete` tag and HEAD (`b9590f1`)
**Spec:** `docs/superpowers/specs/2026-04-29-sp4-docker-design.md`
**Plan:** `docs/superpowers/plans/2026-04-29-sp4-docker.md`

## Outcome at a glance

- All planned deliverables exist and the static portion of the done-check passes.
- `make build lint test verify` all green; SP1-SP4 done-checks still pass.
- One **Critical** runtime defect: a host-port collision on `28332` will make `make compose-up` fail before bootstrap even runs.
- One **Important** Kafka listener mismatch that will break any host-side Kafka client.
- A handful of **Minor** issues (stale comments, dead env var, brittle WIF parsing).
- Spec itself contains the same `28332` collision; the implementation copied the bug faithfully. Fixing the implementation requires touching the spec port table too.

The "live boot" path of the done-check has provably never run end-to-end against the current compose file — if it had, the 28332 bind conflict would have been observed.

---

## Critical (must fix)

### C1. Host-port collision on `127.0.0.1:28332` between svnode-1 ZMQ-block and svnode-2 RPC

`compose/docker-compose.yml` lines 111 and 124:

```yaml
svnode-1:
  ports:
    - "127.0.0.1:28332:28332"   # ZMQ hashblock
    ...
svnode-2:
  ports:
    - "127.0.0.1:28332:18332"   # RPC
```

Both services try to bind host port `28332`. `docker compose config --quiet` validates schema only — it does not detect bind conflicts — so the compose file looks fine. The first `docker compose up -d` against this file will fail with:

```
Error response from daemon: Bind for 127.0.0.1:28332 failed: port is already allocated
```

This is the entire reason the live done-check has not been demonstrated. The done-check has only ever been observed in `SP4DOCKER_SKIP_LIVE=1` mode.

The spec's §3 port table has the same collision (`svnode-1 ZMQ block: localhost:28332` and `svnode-2 RPC: localhost:28332`), so this is a planning miss propagated into code, not a typo.

**Recommended fix.** Pick one:

- Move svnode-1 ZMQ block to host port `18331` (already mentioned in spec line 82 — "ZMQ block: localhost:18331") and update `config.docker.yaml`'s `zmq_block_url` accordingly.
- Or shift svnode-2/3 RPC to a non-conflicting prefix, e.g. svnode-2 RPC → `12832`, svnode-3 RPC → `13832`.

The first option matches the spec's own diagram (line 82) and only requires changing one port mapping plus one URL in `config.docker.yaml`.

### C2. Two SP4-DOCKER plan checkboxes are unchecked because live verification never happened

The plan's Task 9 step 1 lists "make compose-up boots all 12 services; bootstrap mines 110 blocks; the 6 nodes converge on one tip." This was never actually verified — see C1. Either:

- Run the live path after fixing C1 and capture results, or
- Note in this review that live verification is deferred to operator hardware.

The user-supplied review prompt acknowledges this ("Live boot of the stack is the operator's job"). That's an acceptable posture *only after* the bind conflict is fixed; otherwise the operator hits the conflict on their first run.

---

## Important (should fix)

### I1. Kafka EXTERNAL listener port-map mismatch

`compose/docker-compose.yml` lines 16–23:

```yaml
ports:
  - "127.0.0.1:19092:9092"          # host 19092 -> container 9092
environment:
  KAFKA_LISTENERS: PLAINTEXT://:9092,EXTERNAL://:19092
  KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092,EXTERNAL://localhost:19092
```

The EXTERNAL listener binds inside the container on port `19092`, but the host-side mapping forwards host `19092` to container `9092` (the PLAINTEXT listener). A host-side Kafka client connecting to `localhost:19092` will:

1. Hit the PLAINTEXT listener at container port 9092.
2. Receive an advertised broker address of `kafka:9092` (PLAINTEXT advertised address).
3. Try to resolve `kafka` from the host — fail (no Docker DNS).

Two consistent fixes:

- Map `127.0.0.1:19092:19092` so host 19092 actually reaches the EXTERNAL listener, OR
- Drop the EXTERNAL listener entirely and have host clients connect via Docker network only (rarely needed for tests anyway).

Today, no SP3 or SP4-txgen code points at Kafka from the host — so this is latent. SP5+ tests that subscribe to Kafka from host will trip on it. Worth fixing now.

### I2. `compose/teardown.sh` uses different `cd` semantics than `compose/bootstrap.sh`

- `bootstrap.sh` does `cd "$(dirname "$0")/.."` (project root) so it can refer to `bin/derive-address` and `config.docker.yaml`.
- `teardown.sh` does `cd "$(dirname "$0")"` (the `compose/` dir) and references `docker-compose.yml`.
- The Makefile's `compose-up` does `$(COMPOSE) up -d` from project root using `-f compose/docker-compose.yml`, but then calls `./compose/teardown.sh` which `cd`s into `compose/` and runs `docker compose -f docker-compose.yml down -v`.

Both work. But the inconsistency adds cognitive load. Consistency suggestion: make `teardown.sh` also `cd "$(dirname "$0")/.."` and run `docker compose -f compose/docker-compose.yml down -v` (or invoke `$COMPOSE` consistently from `Makefile`).

Not blocking; flag as cleanup.

### I3. Stale comment + dead env var: `PORT_PREFIX`

`compose/teranode/settings.docker.conf` line 3 says:

```
# Each Teranode container sets PORT_PREFIX=1, 2, or 3 to select its
# namespace + database + topic.
```

But the actual mechanism uses `SETTINGS_APPLICATION=teranodeN` plus `.teranodeN` overlay keys. `PORT_PREFIX` only appears as `PORT_PREFIX: ""` on teranode-1 (line 65 of compose) and is otherwise unused. Either:

- Delete the `PORT_PREFIX: ""` line on teranode-1, and update the settings comment to refer to `SETTINGS_APPLICATION`.
- Or actually wire `PORT_PREFIX` to something useful (e.g. interpolate into the listening URLs); not recommended since the current overlay-key approach is cleaner.

Fix: prefer the first option.

---

## Minor

### M1. WIF detection by first character is brittle

`cmd/derive-address/main.go` lines 30-34:

```go
mainnet := true
if c := wifStr[0]; c == 'c' || c == '9' {
    mainnet = false
}
```

This is opaque; it works for the known test vectors but doesn't actually read the network byte from the decoded WIF. Since `wif.DecodeWIF` already gives you `w.PrivKey` and the network byte was inside the WIF you decoded, the proper signal is on `w` itself (the `go-bk` `WIF` struct exposes `WIF.NetworkType` or similar — check the version pinned in go.mod). At minimum, document the heuristic with a comment that names the ranges (mainnet K/L/5, testnet/regtest c/9).

The downstream effect of getting this wrong is a wrong address being funded — the bootstrap then funds the wrong address, the test wallet stays empty, tests fail with confusing "no UTXOs" errors. Worth tightening.

### M2. Bootstrap WIF parser is fragile

`compose/bootstrap.sh` line 74:

```bash
WIF=$(grep -E '^\s+wif:' config.docker.yaml | head -1 | awk -F\" '{print $2}')
```

This works only for a quoted WIF on the same line as `wif:`. Any YAML reformat (e.g. block scalar, single quotes, or comment-out-and-add) breaks it silently — the script then `exit 1`s with "WIF not found", which is at least loud, but a real YAML parser (`yq`) would be more robust. `yq` is already a near-ubiquitous companion to `jq`. Consider documenting `yq` as a soft prerequisite or switching to `python3 -c 'import yaml,sys; print(yaml.safe_load(sys.stdin)["funding"]["wif"])' < config.docker.yaml`.

Not blocking.

### M3. `bootstrap.sh` polls Teranode RPC with method `"version"` — verify that exists in v0.15.0-beta-2

`bootstrap.sh` line 39: `wait_for "teranode-1 RPC" 19292 "version" 120`. The SP2 discovery doc enumerates Teranode RPC methods; `version` is a Bitcoin RPC method but Teranode's RPC surface may not implement it. If it doesn't, the wait loop will spin for 120 seconds and exit with "FAIL". A safer pick is `getblockchaininfo` (which the spec §10 done-check uses against Teranode ports later).

Cross-check `internal/teranode/jsonrpc.go` (or SP2 discovery) before next compose-up to confirm `version` works.

### M4. `bootstrap.sh` mesh-convergence loop computes `UNIQ` only inside the loop

If the loop exits at iteration 1 with `[ "$UNIQ" = "1" ]`, fine. If it never converges, the post-loop `if [ "$UNIQ" != "1" ] || echo "$TIPS" | grep -q ERR` re-evaluates `$UNIQ` — but if every iteration had `UNIQ=1` *with* an `ERR` token (i.e. all RPCs erroring), the loop never breaks and the WARN block fires correctly. Edge case OK on inspection, but fragile. Consider returning explicitly via `converged=1` flag.

### M5. Done-check exits 0 silently when Docker is missing in non-skip mode

`scripts/sp4-docker-done-check.sh` line 21 runs `docker compose ... config --quiet`. If `docker` is not installed, `set -e` propagates the failure and the script exits non-zero — that's correct. Fine, just noting the behaviour matches expectations.

### M6. Compose YAML has copy-pasted Teranode and SV node blocks with no anchors

The 3 Teranode services are fully duplicated (image, depends_on, volumes, networks) with only `ports` and `SETTINGS_APPLICATION` differing. Same for the 3 SV nodes. YAML anchors (`x-teranode-base: &teranode-base { ... }` then `<<: *teranode-base`) would deduplicate. Either approach is acceptable per the spec's quality concern; on inspection, **the duplicates have no unintended divergences** — every block has the same image, depends_on, volume mount, and network. Flag only as "future cleanup".

### M7. `relaypriority=0` in `bitcoin.conf` is unusual

The base SV node config sets `relaypriority=0` with no comment. SV node 1.1.0 may or may not honor this flag. Not load-bearing for the test environment, but undocumented. Add a comment or remove if not actually needed.

### M8. `.gitignore` pattern in `compose/.gitignore` is overly broad

```
data/
*.log
```

`*.log` matches any `.log` anywhere in `compose/`. This is fine but worth noting it's not directory-scoped. Probably intended.

---

## Strengths

- **Spec-faithful structure.** Every section §3-§9 of the design has a concrete deliverable. The 8 commits cleanly map to the plan's Tasks 1–8.
- **Solid hygiene**: all 3 shell scripts use `set -euo pipefail`; bootstrap calls `require curl`/`require jq` upfront and `exit 1`s with clear messages on any precondition miss.
- **127.0.0.1 binding everywhere.** All 29 port mappings are loopback-bound; no exposure to other interfaces. This matches the security posture of the spec.
- **Idempotent bootstrap.** Re-running mines more blocks (regtest is fine with this); the funding step only fails if RPC errors. Honest about its assumptions.
- **Static done-check is fast and useful.** The `SP4DOCKER_SKIP_LIVE=1` path completes in <5s and exercises every artefact it can without Docker. CI-friendly.
- **`docs/compose.md`** covers all the items called for in §11 of the spec (prerequisites, port table, debugging, reset, version note).
- **Genesis + Chronicle activation** correctly forced from block 1 in `settings.docker.conf` (lines 11-12).
- **Wallet asymmetry** correct: svnode-1 has `disablewallet=0`; svnode-2 and svnode-3 have `disablewallet=1`.
- **`derive-address` is properly out-of-band.** Shell scripts don't embed key derivation; the Go binary is built by `make build` and called by bootstrap.
- **SP1–SP4 done-checks all pass.** No regression in earlier sub-projects.
- **`make build lint test verify` exits 0.** No new staticcheck or vet warnings.

---

## Spec coverage gaps

- **§3 port table**: the spec itself contains the C1 port collision (line 82's "svnode-1 ZMQ block: localhost:28332" vs line 81's "svnode-2: 28332"). Spec needs a coordinated update with the implementation.
- **§7 bootstrap flow**: spec uses `getbestblockhash` to readiness-check Teranode; the implementation uses `version` (M3). Consider bringing them back into agreement.
- **§9 Makefile**: spec has `compose-down` calling `$(COMPOSE) down -v` directly; implementation calls `./compose/teardown.sh` which calls the same thing but adds a layer of indirection. Functionally equivalent.
- **§10 done-check**: spec's done-check uses `make compose-test || true` for the smoke run; implementation uses `./bin/teranode-acceptance --short --config config.docker.yaml || true` directly. Same effect; minor divergence.

None of these are problematic — but the C1 collision in §3 must be reconciled in the spec when the code is fixed.

---

## Recommendation

**Status: APPROVED WITH CHANGES.**

Block on:
- **C1** (port-bind conflict) — must fix before any operator runs `make compose-up`.

Address before merging or before tagging `sp4-docker-complete`:
- **I1** (Kafka EXTERNAL listener) — latent now, breaks SP5+ tests.
- **I3** (stale `PORT_PREFIX` cruft) — small cleanup, removes confusion.

Defer to follow-up commits (no blocker):
- **I2** (teardown.sh `cd` consistency)
- **M1–M8** (minor cleanups and hardening)

After C1 is fixed and the spec port table updated to match, re-run `scripts/sp4-docker-done-check.sh` *without* `SP4DOCKER_SKIP_LIVE=1` on a host with Docker available to capture the full end-to-end execution. That run is the missing evidence for the plan's Task 9 verification.

Excellent work overall — this is a clean, well-documented compose stack. The C1 defect is a planning oversight that simply propagated forward; no one has run the live boot yet, so it never surfaced. Fix it once, ship it.
