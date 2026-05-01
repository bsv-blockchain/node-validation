# SP1 — Reportable Skeleton (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP1 / 10 — Reportable Skeleton
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-28
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Module path:** `github.com/bsv-blockchain/node-validation`
**Repo location:** `/Users/oskarsson/gitcheckout/node-validation/` (project files at repo root)
**Go version:** 1.22
**Status:** awaiting user review

---

## 1. Purpose

Build the foundation of the Teranode acceptance-test suite: a runnable Go program that loads
configuration, owns the canonical traceability matrix as Go data, executes zero tests in SP1, and
emits a complete 58-row report (text + JSON + HTML) with a verdict and exit code that honestly
reflect "no automated coverage yet."

SP1's job is to prove the harness is correct *before* a single test exists. Once SP1 is done,
later sub-projects add tests by registering them in `cmd/teranode-acceptance/register.go`; nothing
else about the harness needs to change.

## 2. Scope

### In scope for SP1

- `go.mod`, project layout, `Makefile` (`build`, `lint`, `test`, `test-short`, `cover`, `clean`).
- `.gitignore`, `config.example.yaml`, `README.md` skeleton, `docs/` skeleton.
- `internal/matrix/` — the static §3 matrix as Go data, plus invariants and lookup helpers.
- `config/` — YAML / env / flag loader with validation.
- `internal/testrunner/` — `Env`, `Suite`, `Result`, `Check`, `Severity`, `Status`, registry,
  panic recovery, ctx cancellation, fake-clock seam.
- `internal/testrunner/reporter.go`, `verdict.go` — text / JSON / HTML report emission and
  verdict / completeness-invariant logic.
- `internal/overrides/` — reviewer-overrides YAML loader.
- `cmd/teranode-acceptance/` — CLI entry point, version embedding, registry hook.
- `cmd/gen-traceability/` — codegen tool that emits `README.md`'s §3 matrix and
  `docs/traceability.md` from the manifest. (Eliminates manifest-vs-README drift permanently.)
- Unit tests for every package above; integration tests for the CLI's three definition-of-done
  scenarios.
- A `scripts/sp1-done-check.sh` script that mechanically asserts SP1's definition of done.

### Out of scope for SP1 (deferred to later sub-projects)

- Backend clients (`internal/teranode/`, `internal/svnode/`) — SP3.
- Transaction generator (`internal/txgen/`), comparison helpers (`internal/compare/`) — SP4.
- Any test in `tests/` — SP5+ in cost order.
- Discovery against the upstream Teranode repo — SP2 (parallel session).
- HTML field-level rendering golden tests — SP10.

## 3. Architecture

```
cmd/teranode-acceptance/main.go
        |
        v
config.Load(args, env, yaml)  -->  Config (validated)
        |
        v
testrunner.NewEnv(Config)     -->  Env (clients nil-safe in SP1, logger, manifest, clock)
        |
        v
testrunner.NewSuite(Env)
   |
   +-- Register(testID, fn)              # SP1: empty body of registerTests
   |
   +-- Run(ctx) -> []Result               # sequential dispatch with per-test timeout
   |
   +-- BuildReportModel(env, results, overrides)
              |
              v
       reporter.WriteText(stdout, model)
       reporter.WriteJSON(path, model)
       reporter.WriteHTML(path, model)
              |
              v
       exit(model.Verdict.ExitCode)
```

### Package responsibilities

| Package | Responsibility |
|---|---|
| `config` | Load YAML / env / flags, validate, expose typed `Config`. |
| `internal/matrix` | The canonical 58-row §3 manifest, lookup helpers, structural invariants, golden-file diff. |
| `internal/overrides` | Load and validate the reviewer-overrides YAML. |
| `internal/testrunner` | `Env`, `Suite`, `Result` types, registry, dispatch with panic / ctx handling, fake-clock seam. |
| `internal/testrunner/reporter.go` | Text streamer, JSON writer, HTML writer. |
| `internal/testrunner/verdict.go` | Verdict computation and completeness invariant enforcement. |
| `cmd/teranode-acceptance` | Wire-up, flag parsing, signal handler, run orchestration. |
| `cmd/gen-traceability` | Codegen for README §3 matrix and `docs/traceability.md`. |

### Cross-cutting choices

- **No init-time registries.** `cmd/teranode-acceptance/register.go` registers tests explicitly
  by name. SP1 leaves the body empty. `grep RegisterTest cmd/teranode-acceptance/register.go`
  always returns the full set.
- **No `init()`, no global state.** All state hangs off `Env`. `Env.Now()` is the sole clock
  seam; unit tests inject a stub clock through `NewEnv`.
- **Deterministic JSON output.** `Observations map[string]any` is marshalled with sorted keys
  via a custom `MarshalJSON`. No nondeterminism in the report.
- **Sequential test execution.** Concurrent tests against shared upstream nodes corrupt timing
  and divergence measurements. Per-test timeout (default 30 min) wrapped via
  `context.WithTimeout`. Panic recovery in dispatch converts panics to `StatusError` with stack
  trace in `Err`. SIGINT cancels the context; the in-flight test produces `StatusError` with
  `SkipReason: "interrupted"`.

## 4. `internal/matrix` design

### Types

```go
package matrix

type Kind string
const (
    KindFR  Kind = "FR"   // 11 entries
    KindNFR Kind = "NFR"  // 13 entries
    KindTE  Kind = "TE"   //  3 entries
    KindTC  Kind = "TC"   // 16 source-plan test cases
    KindNEW Kind = "NEW"  //  8 added tests
    KindR   Kind = "R"    //  7 risks
)
// Total: 58.

type CoverageStatus string  // FR / NFR
const (
    CoverageAutomated           CoverageStatus = "AUTOMATED"
    CoverageDocumentationReview CoverageStatus = "DOCUMENTATION_REVIEW"
    CoverageContractual         CoverageStatus = "CONTRACTUAL"
    CoverageLongTermObservation CoverageStatus = "LONG_TERM_OBSERVATION"
    CoveragePrivilegedAccess    CoverageStatus = "PRIVILEGED_ACCESS_REQUIRED"
    CoveragePartial             CoverageStatus = "PARTIAL"
)

type TestCaseStatus string  // TC / NEW
const (
    TCInScope               TestCaseStatus = "IN_SCOPE"
    TCExcludedSetup         TestCaseStatus = "EXCLUDED_SETUP"
    TCExcludedDocumentation TestCaseStatus = "EXCLUDED_DOCUMENTATION"
    TCExcludedPrivileged    TestCaseStatus = "EXCLUDED_PRIVILEGED"
)

type Severity string
const (
    SeverityCritical  Severity = "critical"
    SeverityImportant Severity = "important"
    SeverityAdvisory  Severity = "advisory"
)

type Entry struct {
    ID              string         // "FR-1", "PC-1", "NEW-FR7", "R3", "TE-1"
    Kind            Kind
    Title           string
    CoverageStatus  CoverageStatus // FR / NFR only
    TestCaseStatus  TestCaseStatus // TC / NEW only
    Severity        Severity       // TC / NEW only
    CoveredBy       []string       // FR/NFR -> test IDs; R -> mitigating tests
    SatisfiesReqs   []string       // TC / NEW only -> FR / NFR IDs covered
    ExclusionReason string         // TC only when EXCLUDED_*
    Notes           string
    PartialNote     string         // CoveragePartial: which parts are which
}

type Manifest struct{ Entries []Entry }

func Load() Manifest
func (m Manifest) ByID(id string) (Entry, bool)
func (m Manifest) ByKind(k Kind) []Entry
func (m Manifest) Requirements() []Entry      // FR + NFR
func (m Manifest) TestCases() []Entry         // TC + NEW
func (m Manifest) InScopeTestIDs() []string   // TC IN_SCOPE + all NEW
func (m Manifest) Risks() []Entry             // R
func (m Manifest) Validate() error            // structural checks; called by Load
```

### Layout

Single file `internal/matrix/manifest.go` declares all 58 entries in one ordered literal:
FR-1..FR-11, NFR-1..NFR-13, TE-1..TE-3, source TCs in source-plan order, NEW-* in derivation
order, R-1..R-7. ~300 lines of dense literals; one screen-scrollable diff per change.

### Severity assignment baked in

- **Critical** (8): PC-1, PC-2, PC-3, IBD-2, INTER-1, INTER-2, CLIENT-1, CLIENT-3.
- **Important** (3): PERF-1, OPS-3, CLIENT-2.
- **Advisory** (8): all NEW-*.
- **IBD-1** is `Severity: Critical` *and* `TestCaseStatus: EXCLUDED_DOCUMENTATION`. The runner
  cannot auto-pass it; it permanently demotes the verdict to `INCOMPLETE` unless reviewer
  overrides supply a PASS. This honours the source plan's "Critical Requirements" list which
  explicitly includes IBD-1 (source doc lines 1431-1436).
- **Excluded TCs** (PERF-2, PERF-3, OPS-1, OPS-2) carry their assigned severity for record but
  never run, so they don't affect the verdict.

### `Manifest.Validate()` invariants (each tested individually)

1. Exactly 58 entries.
2. Per-kind counts: 11 FR, 13 NFR, 3 TE, 16 TC, 8 NEW, 7 R.
3. No duplicate IDs.
4. ID format matches kind: `FR-<1..11>`, `NFR-<1..13>`, `TE-<1..3>`, the 16 known TC IDs, the 8
   known NEW IDs, `R<1..7>`.
5. Every cross-reference (`CoveredBy`, `SatisfiesReqs`) resolves to an entry.
6. `Severity` set iff `Kind ∈ {TC, NEW}`.
7. `CoverageStatus` set iff `Kind ∈ {FR, NFR}`.
8. `TestCaseStatus` set iff `Kind ∈ {TC, NEW}`.
9. `ExclusionReason` set iff `TestCaseStatus` is one of `EXCLUDED_*`.

### Tests

- `manifest_test.go` — calls `Validate()`; one `t.Run` per invariant with descriptive failures.
- `golden_test.go` — embeds a frozen YAML of `(ID, Kind, Title)` tuples; diffs `Load()` against
  it. Catches accidental rename / delete / reorder.
- Coverage target: ≥90%.

## 5. `config` design

### Files

```
config/
├── config.go         (types, Load, validation)
├── flags.go          (CLI flag definitions and binding)
├── env.go            (env-var overlay)
├── short.go          (--short duration substitution)
└── config_test.go
```

### Types

```go
type Network string
const (
    NetworkMainnet Network = "mainnet"
    NetworkTestnet Network = "testnet"
    NetworkRegtest Network = "regtest"
)

type Config struct {
    Network    Network        `yaml:"network"`
    Teranode   Teranode       `yaml:"teranode"`
    SVNode     SVNode         `yaml:"svnode"`
    Funding    Funding        `yaml:"funding"`
    Durations  Durations      `yaml:"durations"`
    Limits     Limits         `yaml:"limits"`

    // CLI-only fields (not in YAML)
    ConfigPath        string   `yaml:"-"`
    Only              []string `yaml:"-"`
    Skip              []string `yaml:"-"`
    ReportJSON        string   `yaml:"-"` // default "report.json"
    ReportHTML        string   `yaml:"-"` // default "report.html"
    Verbose           bool     `yaml:"-"`
    Short             bool     `yaml:"-"`
    AllowMainnetLoad  bool     `yaml:"-"`
    StrictConfig      bool     `yaml:"-"`
    ReviewerOverrides string   `yaml:"-"`
    TestTimeout       time.Duration `yaml:"-"`
}

type Teranode struct {
    RPCURL          string `yaml:"rpc_url"`
    RPCUser         string `yaml:"rpc_user"`
    RPCPass         string `yaml:"rpc_pass"`
    RESTURL         string `yaml:"rest_url"`
    NotificationURL string `yaml:"notification_url"`
    P2PAddress      string `yaml:"p2p_address"`
    MetricsURL      string `yaml:"metrics_url"`
    HealthURL       string `yaml:"health_url"`
}

type SVNode struct {
    RPCURL      string `yaml:"rpc_url"`
    RPCUser     string `yaml:"rpc_user"`
    RPCPass     string `yaml:"rpc_pass"`
    ZMQBlockURL string `yaml:"zmq_block_url"`
    ZMQTxURL    string `yaml:"zmq_tx_url"`
}

type Funding struct {
    WIF             string `yaml:"wif"`
    MinBalanceSats  uint64 `yaml:"min_balance_satoshis"`
}

type Durations struct {
    PC1Observation     time.Duration `yaml:"pc1_observation"`     // 168h default
    INTER1Observation  time.Duration `yaml:"inter1_observation"`  // 336h default
    PERF1PerRate       time.Duration `yaml:"perf1_per_rate"`      // 5m default
    DefaultPropagation time.Duration `yaml:"default_propagation"` // 10s default
    CLIENT1Observation time.Duration `yaml:"client1_observation"` // 1h default
    NewNFR7Iterations  int           `yaml:"new_nfr7_iterations"` // 100 default
}

type Limits struct {
    PERF1MaxTPS          int      `yaml:"perf1_max_tps"`           // 1000 default
    INTER2TxCount        int      `yaml:"inter2_tx_count"`         // 1000 default
    CLIENT3TxCount       int      `yaml:"client3_tx_count"`        // 500 default
    FR7ChainDepth        int      `yaml:"fr7_chain_depth"`         // 25 default
    FR10LatencyTargetMs  int      `yaml:"fr10_latency_target_ms"`  // 100 default
    FR8PriorityLevels    []string `yaml:"fr8_priority_levels"`     // [economy,standard,priority]
}
```

### Precedence

```
1. Built-in defaults (timeouts / limits only — no URLs, no secrets).
2. YAML file at --config or default ./config.yaml.
   - Default file missing: fine.
   - Explicitly named file missing: error.
3. Env-var overlay: TNG_<UPPER_SNAKE_OF_FIELD_PATH>. Empty values do not blank YAML values.
4. CLI flags (highest precedence).
5. If --short, short.Apply(&cfg) substitutes:
       PC1Observation:    168h -> 30m
       INTER1Observation: 336h -> 1h
       CLIENT1Observation:  1h -> 5m
       PERF1PerRate:        5m -> 30s
6. Validate(); return Config or aggregated error.
```

### Validation rules (errors aggregated, all reported in one message)

| Rule | Reason |
|---|---|
| `Network ∈ {mainnet, testnet, regtest}` | typo detection |
| Mainnet + load-generating tests requested without `--allow-mainnet-load` | safety |
| All Teranode URLs parse with valid scheme (`http`, `https`, `ws`, `wss`, `tcp`) | early failure |
| `Teranode.RPCURL` non-empty if any test reading RPC is requested; with `--strict-config`, required regardless | tests can't talk to nothing |
| `Funding.WIF` matches base58/WIF regex if non-empty | full check deferred to SP4 |
| All durations > 0 | YAML typos |
| `Durations.NewNFR7Iterations >= 1` | sanity |
| All `Limits.*` > 0 | sanity |
| `--only` / `--skip` IDs must exist in `matrix.Manifest` | typos like "PC1" vs "PC-1" |
| `--only` and `--skip` mutually exclusive | obvious |

Config error -> exit code **4**.

### Tests

- Round-trip YAML serialise / deserialise.
- One test per precedence layer (defaults < YAML < env < flags).
- `--short` substitution: only listed durations change.
- Validation table: ~15 bad configs, each producing a specific error substring.
- `--only` / `--skip` rejected when ID not in manifest. Imports `internal/matrix`.
- `--only` and `--skip` mutually exclusive.
- Mainnet-load gate fires only when load-generating tests are in the requested set.
- Coverage target: ≥80%.

### `config.example.yaml`

Ships with placeholder values matching every field. Inline comments mirror this section.
Secrets are commented out so `cp config.example.yaml config.yaml` produces a no-network-access
config.

## 6. `internal/testrunner` design

### Result types

```go
type Status string
const (
    StatusPass                Status = "PASS"
    StatusFail                Status = "FAIL"
    StatusSkipped             Status = "SKIPPED"
    StatusError               Status = "ERROR"
    StatusFeatureNotAvailable Status = "FEATURE_NOT_AVAILABLE"
    StatusDeferred            Status = "DEFERRED"  // documentation review / contractual / long-term
    StatusNotRun              Status = "NOT_RUN"   // in-scope test the run didn't execute
)

type Check struct {
    Description string
    Required    bool
    Pass        bool
    Detail      string
}

type Result struct {
    ID                    string
    Title                 string
    Severity              matrix.Severity
    Status                Status
    StartedAt             time.Time
    Duration              time.Duration
    AcceptanceChecks      []Check
    Observations          map[string]any
    PartialEvidence       bool
    SkipReason            string
    Err                   string
    CapturedRisks         []string
    SatisfiesRequirements []string
}
```

`StatusDeferred` and `StatusNotRun` are added vs build doc §8 — without them the report can't
distinguish "we never ran this" from "we ran it and it passed."

### Env

```go
type Env struct {
    Cfg      config.Config
    Logger   *slog.Logger
    Now      func() time.Time
    Manifest matrix.Manifest

    // Populated in later sub-projects; nil-safe in SP1.
    Teranode TeranodeClients
    SVNode   SVNodeClients
    TxGen    TxGenerator
}
```

Tests in SP5+ that need a nil client return `Status: StatusSkipped, SkipReason: "client not configured"`.

### Suite + dispatch

```go
type TestFunc func(ctx context.Context, env *Env) Result

type Suite struct {
    env *Env
    reg []registration
}

type registration struct {
    ID       string
    Severity matrix.Severity
    Fn       TestFunc
}

func NewSuite(env *Env) *Suite

// Register asserts:
//  - id is in env.Manifest as TC or NEW with TestCaseStatus IN_SCOPE
//  - id has not already been registered
// Severity is read from the manifest (single source of truth). Programmer
// errors panic — `go test` catches them.
func (s *Suite) Register(id string, fn TestFunc)

func (s *Suite) Run(ctx context.Context) []Result
```

`Run` semantics:

- Sequential dispatch.
- Per-test timeout from `cfg.TestTimeout` (default 30 min).
- Panic recovery converts to `StatusError` with stack trace.
- SIGINT cancels the context; in-flight test produces `StatusError, SkipReason: "interrupted"`.
- Tests excluded by `--only` / `--skip` still appear in results as `StatusNotRun, SkipReason: "filtered out by --only"`.

### Verdict

```go
type Verdict struct {
    Decision  string  // "GO" | "CONDITIONAL_GO" | "NO_GO" | "INCOMPLETE"
    ExitCode  int     // 0 | 2 | 1 | 3
    Rationale string
}

func ComputeVerdict(results []Result, m matrix.Manifest, ovr ReviewerOverrides) Verdict
```

Decision tree:

```
1. any in-scope Critical with Status FAIL or ERROR              -> NO_GO          (1)
2. any in-scope Critical with Status NOT_RUN                    -> INCOMPLETE     (3)
3. any Critical entry whose row is DOCUMENTATION_REVIEW
   and not satisfied by overrides                               -> INCOMPLETE     (3)
4. any in-scope Important with Status FAIL or ERROR             -> CONDITIONAL_GO (2)
5. any in-scope Important with Status NOT_RUN                   -> CONDITIONAL_GO (2)
   (Important SKIPPED is acceptable — source plan permits "mitigation plan".)
6. otherwise                                                    -> GO             (0)
```

Advisory (NEW-*) results never change the verdict; they feed per-requirement satisfaction in
the requirements rows but cannot demote the verdict by themselves.

### Reviewer overrides

`internal/overrides/overrides.go` loads YAML of the form:

```yaml
reviewer: "Lars Jorgensen <l.jorgensen@teranode.group>"
reviewed_at: "2026-04-29T14:00:00Z"
overrides:
  IBD-1:
    decision: PASS         # or FAIL
    artefacts: ["bsva-audit-2026-q1.pdf"]
    note: "Reviewed BSVA's IBD report dated 2026-03-15."
```

Validation: `decision ∈ {PASS, FAIL}`; `artefacts` non-empty; `note` non-empty; every override
ID is a manifest entry whose `CoverageStatus` or `TestCaseStatus` permits override
(`DOCUMENTATION_REVIEW`, `CONTRACTUAL`, `LONG_TERM_OBSERVATION`). The override file content is
recorded into the JSON report under `run.reviewer_overrides` for audit.

### Reporter

All three sinks operate from a single `ReportModel` produced by `BuildReportModel`.

```go
type ReportModel struct {
    Run             RunHeader
    Verdict         Verdict
    Requirements    []RequirementRow      // FR + NFR (24)
    TestEnvironment []TestEnvironmentRow  // 3
    TestCases       []TestCaseRow         // 16 + 8 = 24
    Risks           []RiskRow             // 7
    Summary         SummaryCounts
}
```

`BuildReportModel` walks `env.Manifest.Entries`. For each entry it constructs the appropriate
row. If after the walk total rows ≠ 58 (24 + 3 + 24 + 7) or any kind count is off, returns an
error. The CLI exits 1 on a completeness-invariant failure.

Risk mitigation status is derived dynamically:

```
mapped       = entry.CoveredBy   (mitigating test IDs for an R-row)
all PASS     -> MITIGATED
some non-PASS -> PARTIALLY_MITIGATED
all FAIL/ERROR -> NOT_MITIGATED
EXCLUDED_DOCUMENTATION rows treated as DEFERRED unless overrides PASS them
```

Three sinks:

- **Text streamer** — one line per test as it completes; final summary block. `slog` text
  handler by default; JSON when `--verbose`.
- **JSON writer** — marshals `ReportModel`. Schema matches build doc §9.1 with the additions
  noted above (`reviewer_overrides`, `INCOMPLETE` verdict, `StatusDeferred`, `StatusNotRun`).
- **HTML writer** — `html/template` (std lib only). Single self-contained file with embedded
  CSS via `<style>`; no JS, no external assets. Layout follows build doc §9.2 sections 1-11.
  Verdict banner colours: green `#1f883d`, yellow `#d29922`, red `#cf222e`, grey `#6e7781`
  (INCOMPLETE).

### Tests

- `runner_test.go` — register/run synthetic TestFunc; panic→ERROR; ctx-cancel→ERROR;
  filter→NOT_RUN; severity sourced from manifest; duplicate Register panics.
- `verdict_test.go` — table-driven, ~25 synthetic Result sets covering every branch including
  the override-driven GO path and every demotion path.
- `reporter_test.go` — golden-file diff for JSON; HTML structural assertions (verdict banner,
  58 rows rendered, every `<details>` has summary + body).
- `completeness_test.go` — manifest missing one entry → BuildReportModel errors; manifest with
  extra entry → errors; healthy manifest → exactly 58 rows.
- Coverage targets: 70% reporter, 80%+ runner, 100% verdict branches.

## 7. `cmd/teranode-acceptance` design

### Flag set

```
--config string                 path to YAML config (default ./config.yaml)
--only string                   comma-separated test IDs to run
--skip string                   comma-separated test IDs to skip
--report-json string            JSON output path (default report.json)
--report-html string            HTML output path (default report.html)
--reviewer-overrides string     reviewer overrides YAML (default none)
--verbose                       slog JSON streaming output
--short                         shorten long-running observations
--allow-mainnet-load            permits load-generating tests against mainnet
--strict-config                 fail on missing endpoint URLs even if no test needs them
--test-timeout duration         per-test hard timeout (default 30m)
--version
--help
```

Std `flag` package — no cobra, no viper.

### Main flow

```go
func main() {
    cfg, err := config.Load(os.Args[1:], os.Environ())
    switch {
    case errors.Is(err, config.ErrHelp):    os.Exit(0)
    case errors.Is(err, config.ErrVersion): os.Exit(0)
    case err != nil:
        fmt.Fprintln(os.Stderr, err)
        os.Exit(4)
    }

    logger := newLogger(cfg.Verbose)
    manifest := matrix.Load()
    ovr, err := overrides.Load(cfg.ReviewerOverrides)
    if err != nil { logger.Error(...); os.Exit(4) }

    env := testrunner.NewEnv(cfg, logger, manifest, time.Now)

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    suite := testrunner.NewSuite(env)
    registerTests(suite) // SP1: empty body. SP5+: explicit Register lines.

    results := suite.Run(ctx)
    model := testrunner.BuildReportModel(env, results, ovr)

    reporter.WriteText(os.Stdout, model)
    reporter.WriteJSON(cfg.ReportJSON, model)
    reporter.WriteHTML(cfg.ReportHTML, model)

    os.Exit(model.Verdict.ExitCode)
}
```

### Version embedding

```go
var version = "dev"
```

Makefile sets `-ldflags "-X main.version=$(git describe --tags --always --dirty)"`.

### Tests

- `register_test.go` — assert `registerTests(empty suite)` produces zero registrations
  (regression guard for "someone added a test in SP1").
- Integration tests live in `cmd/teranode-acceptance/main_test.go` (see §9).

## 8. `cmd/gen-traceability` design

Codegen tool eliminating manifest / README / `docs/traceability.md` drift.

```go
// Reads internal/matrix.Load() and produces:
//   docs/traceability.md       (full standalone reference)
//   README.md §3 matrix block  (between markers <!-- TRACEABILITY:START --> ... END)
//
// Run via:
//   go run ./cmd/gen-traceability
//
// Makefile target `gen` runs it. CI runs it and `git diff --exit-code` fails the build if the
// committed README / docs are out of sync with manifest.go.
```

The README §3 block is delimited by HTML comments so the rest of the README is hand-authored
and untouched by codegen.

## 9. Makefile

```makefile
.PHONY: build lint test test-short cover gen verify clean

GO := go
LDFLAGS := -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/teranode-acceptance ./cmd/teranode-acceptance
	$(GO) build -o bin/gen-traceability ./cmd/gen-traceability

lint:
	$(GO) vet ./...
	gofmt -l . | (! grep .)
	staticcheck ./...

test:
	$(GO) test -race ./...

test-short: build
	./bin/teranode-acceptance --short --config config.yaml

cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

gen:
	./bin/gen-traceability

verify: gen
	@git diff --exit-code README.md docs/traceability.md \
	  || (echo "README / traceability.md out of sync with manifest" && exit 1)

clean:
	rm -rf bin/ report.json report.html coverage.out coverage.html
```

`make verify` is the CI gate that catches manifest drift.

## 10. Verification & testing strategy

### Layer 1 — unit tests (under `make test`)

| Package | Coverage |
|---|---|
| `config` | ≥80% |
| `internal/matrix` | ≥90% |
| `internal/overrides` | ≥80% |
| `internal/testrunner` (runner) | ≥80% |
| `internal/testrunner` (verdict) | 100% of branches |
| `internal/testrunner` (reporter) | ≥70% |
| `internal/testrunner` (completeness) | 100% |

### Layer 2 — integration tests (`cmd/teranode-acceptance/main_test.go`)

Builds the binary, invokes three scenarios:

1. **Filter typo** — `--only nonexistent-test` → exit 4 (config error).
2. **Zero-test happy path** — valid config, no `--only`, no overrides → exit 3 (INCOMPLETE);
   JSON has 58 rows; HTML exists.
3. **Zero-test with overrides marking IBD-1, FR-4, NFR-1 PASS** — overrides supplied → still
   exit 3 because all in-scope Critical tests are NOT_RUN. Overrides don't paper over missing
   automation.

### Layer 3 — static checks (`make lint`)

```
gofmt -l . | (! grep .)
go vet ./...
staticcheck ./...
```

No `golangci-lint` in SP1; `staticcheck` alone is sufficient.

### Layer 4 — definition-of-done (`scripts/sp1-done-check.sh`)

```bash
#!/usr/bin/env bash
set -euo pipefail
make build lint test verify
make test-short || true   # exit 3 expected
test -s report.json
test -s report.html
jq '.requirements      | length' report.json | grep -qx 24
jq '.test_environment  | length' report.json | grep -qx 3
jq '.test_cases        | length' report.json | grep -qx 24
jq '.risks             | length' report.json | grep -qx 7
jq -r '.verdict.decision' report.json | grep -qx INCOMPLETE
```

If every line passes, SP1 is done.

## 11. Definition of done

All true:

- `make build lint test verify` clean.
- `make test-short` runs and produces report.json + report.html with verdict INCOMPLETE,
  exit 3, all 58 rows present.
- Codegen invariant: `git diff --exit-code README.md docs/traceability.md` after `make gen`.
- Unit tests pass with -race. Coverage targets met per §10.
- Integration tests in `main_test.go` pass.
- Manifest golden-file diff passes.
- `scripts/sp1-done-check.sh` exits 0.
- Project compiles and runs without any source from `bsv-blockchain/teranode` present.

## 12. Risks tracked through SP1

| # | Risk | Mitigation |
|---|---|---|
| A | HTML template field-level drift | Structural-only assertions in SP1; full golden in SP10. |
| B | Override-file injection (PASS without artefact verification) | Schema requires non-empty `artefacts` and `note`; runner records the override file in the report for audit. Artefact validity is the human reviewer's responsibility. |
| C | `--short` runs producing reports that look complete | Every Result from `--short` carries `PartialEvidence: true`; reporter renders a yellow badge in HTML. |
| D | Manifest / README drift | Codegen tool `cmd/gen-traceability` and CI gate `make verify` make drift impossible. |
| E | Non-standard exit code 3 (INCOMPLETE) | Documented prominently in README §"Exit codes". |

## 13. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | Module path `github.com/bsv-blockchain/node-validation` | per user |
| 2 | Project files at repo root | per user |
| 3 | Verdict `INCOMPLETE` (exit 3) added to source plan's three | per user |
| 4 | Config error exit code 4 (separate from NO_GO) | per user |
| 5 | Codegen README §3 from manifest in SP1 | per user |
| 6 | Code-reviewer agent invoked after each sub-project | per user |
| 7 | SP1 + SP2 run as parallel sessions | per user |
| 8 | WebSocket library `nhooyr.io/websocket` | drafter |
| 9 | Go version 1.22 floor | drafter |
| 10 | Sequential test dispatch | drafter |
| 11 | Reviewer overrides in their own YAML, not main config | drafter |
| 12 | Reporter sub-package lives under `internal/testrunner/` | drafter (matches build doc §4) |
| 13 | `Severity` sourced from manifest, not Register parameter | drafter |
| 14 | `StatusNotRun` and `StatusDeferred` added to source-plan Status enum | drafter |

## 14. Out-of-scope reminders

This sub-project does **not** add tests, clients, txgen, or discovery output. It produces a
honest INCOMPLETE report with zero tests. Subsequent sub-projects layer functionality on the
foundation laid here.
