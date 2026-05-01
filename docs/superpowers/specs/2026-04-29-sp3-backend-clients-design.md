# SP3 — Backend Clients (design)

**Project:** Teranode Acceptance Tests
**Sub-project:** SP3 / 10 — Backend Clients
**Author:** drafted with siggi.oskarsson@bsvassociation.org
**Date:** 2026-04-29
**Source plan:** *TNG Teranode Requirements and Test Plan*, version 1.3, 28/04/2026
**Depends on:** SP1 (skeleton, tag `sp1-complete`), SP2 (discovery, tag `sp2-complete`)
**Status:** awaiting user review

---

## 1. Purpose

Build typed Go clients for both backends under test (Teranode and SV Node) plus a small
comparison helper. Clients honour the wire details surfaced in `docs/discovery.md` and are
constructed at startup from the validated `config.Config`. This sub-project produces no test
cases — it produces the client surface SP5+ tests will use.

Clients are nil-safe: a missing URL in config means the corresponding client is `nil`, and
SP5+ tests check for nil and skip with `"client not configured"`.

## 2. Scope

### In scope

- `internal/teranode/{rpc,rest,notifications,p2p_probe,metrics,health}.go` — six small clients
  matching the surfaces discovered in SP2.
- `internal/svnode/{rpc,zmq}.go` — bitcoind-compatible JSON-RPC client and ZMQ subscriber.
- `internal/compare/chainstate.go` — rejection-category mapping (Teranode error → SV Node
  error → comparable category enum).
- `httptest` / faked-WebSocket / faked-ZMQ unit tests for every public method.
- Wiring in `cmd/teranode-acceptance/main.go` to construct the clients from config and attach
  them to `Env`.
- `internal/testrunner` placeholder interfaces (`TeranodeClients`, `SVNodeClients`,
  `TxGenerator`) replaced with concrete struct types.

### Out of scope

- Any test in `tests/` — SP5+.
- Transaction generator (`internal/txgen/`) — SP4.
- Live network smoke against a real Teranode + SV Node pair — deferred until SP5 onwards.
- A full Centrifuge protocol re-implementation; SP3 uses the upstream `centrifuge-go` library.
- A cgo-based ZMQ binding; SP3 uses the pure-Go `go-zeromq/zmq4`.

## 3. Architecture

```
cmd/teranode-acceptance/main.go
    │
    ├── (existing) config.Load → cfg
    │
    ├── teranode.NewClients(cfg.Teranode, logger) → *teranode.Clients
    │       ├── RPC           (port 9292, Basic Auth)
    │       ├── REST          (port 8090, /api/v1)
    │       ├── Notifications (port 8090, /connection/websocket — Centrifuge)
    │       ├── P2PProbe      (port 8333/18333 + 9905, TCP probe)
    │       ├── Metrics       (port 9091, /metrics — Prometheus text)
    │       └── Health        (port 8000, /health/readiness)
    │
    ├── svnode.NewClients(cfg.SVNode, logger) → *svnode.Clients
    │       ├── RPC  (JSON-RPC 1.0 with Basic Auth)
    │       └── ZMQ  (block & tx subscribers, optional)
    │
    └── env := testrunner.NewEnv(cfg, logger, manifest, time.Now)
        env.Teranode = teranodeClients   // typed concrete struct
        env.SVNode   = svnodeClients
```

`internal/testrunner/types.go` `TeranodeClients` and `SVNodeClients` interfaces become typed
struct types (or the field types change directly to the concrete `*teranode.Clients`).

### Cross-cutting choices

- All clients accept `context.Context` and respect cancellation.
- All clients log structured (`*slog.Logger` injected via constructor).
- All clients return errors with rich context (URL, method, status code).
- No package-level state. All state hangs off the client struct.
- Constructors return `nil` plus error if the URL is malformed; `nil` plus nil-error if the
  URL is empty (deliberate skip).
- HTTP clients share a `*http.Client` with sensible timeouts (30s default, configurable).

## 4. Package designs

### 4.1 `internal/teranode/rpc.go`

```go
package teranode

type RPCClient struct {
    base    *url.URL
    user    string
    pass    string
    http    *http.Client
    logger  *slog.Logger
    nextID  atomic.Int64
}

func NewRPCClient(rawURL, user, pass string, logger *slog.Logger) (*RPCClient, error)

// Call issues a JSON-RPC 1.0 request and returns the raw result.
// Errors carry the JSON-RPC error code in their type.
func (c *RPCClient) Call(ctx context.Context, method string, params []any, out any) error

// Convenience wrappers — only the methods SP5+ tests need:
func (c *RPCClient) GetBestBlockHash(ctx context.Context) (string, error)
func (c *RPCClient) GetBlock(ctx context.Context, hash string, verbosity uint32) (json.RawMessage, error)
func (c *RPCClient) GetBlockHeader(ctx context.Context, hash string, verbose bool) (json.RawMessage, error)
func (c *RPCClient) GetBlockHash(ctx context.Context, height int64) (string, error)
func (c *RPCClient) GetBlockchainInfo(ctx context.Context) (BlockchainInfo, error)
func (c *RPCClient) GetRawTransaction(ctx context.Context, txid string, verbose int) (json.RawMessage, error)
func (c *RPCClient) GetRawMempool(ctx context.Context) ([]string, error)
func (c *RPCClient) SendRawTransaction(ctx context.Context, hexTx string) (string, error)
func (c *RPCClient) GetMiningInfo(ctx context.Context) (json.RawMessage, error)
func (c *RPCClient) GetPeerInfo(ctx context.Context) (json.RawMessage, error)
func (c *RPCClient) GetChainTips(ctx context.Context) (json.RawMessage, error)
func (c *RPCClient) Version(ctx context.Context) (json.RawMessage, error)
```

`RPCError` is a named error type carrying `Code int` and `Message string` so tests can branch
on `errors.As`. JSON-RPC 1.0 wire format per discovery (`{"method":..., "params":[...], "id":...}`).

### 4.2 `internal/teranode/rest.go`

```go
type RESTClient struct {
    base   *url.URL  // includes /api/v1 prefix
    http   *http.Client
    logger *slog.Logger
}

func NewRESTClient(rawURL string, logger *slog.Logger) (*RESTClient, error)

// All getters honour the URL-suffix content-negotiation: /tx/{hash}/json returns JSON;
// /tx/{hash} returns binary; /tx/{hash}/hex returns hex.
func (c *RESTClient) GetTxBytes(ctx context.Context, hash string) ([]byte, error)
func (c *RESTClient) GetTxJSON(ctx context.Context, hash string) (json.RawMessage, error)
func (c *RESTClient) GetBlockBytes(ctx context.Context, hash string) ([]byte, error)
func (c *RESTClient) GetBlockJSON(ctx context.Context, hash string) (json.RawMessage, error)
func (c *RESTClient) GetBlockHeaderBytes(ctx context.Context, hash string) ([]byte, error)  // 80 bytes
func (c *RESTClient) GetBlockHeaderJSON(ctx context.Context, hash string) (json.RawMessage, error)
func (c *RESTClient) GetBestBlockHeaderJSON(ctx context.Context) (json.RawMessage, error)
func (c *RESTClient) GetUTXOJSON(ctx context.Context, utxoHash string) (json.RawMessage, error)
func (c *RESTClient) Search(ctx context.Context, q string) (json.RawMessage, error)
func (c *RESTClient) ListBlocks(ctx context.Context, offset, limit int) (json.RawMessage, error)
```

Error wrapping includes the path and HTTP status. `RESTError` named type with status code so
`errors.As` can branch on 404 vs 5xx.

### 4.3 `internal/teranode/notifications.go`

Uses `github.com/centrifugal/centrifuge-go` (pinned at `v0.10.x` to match the upstream
Teranode reference client). Subscribes to all five auto-subscribed channels (`block`,
`subtree`, `node_status`, `ping`, `mining_on`) and exposes per-event Go channels.

```go
type NotificationClient struct {
    cfg    *centrifuge.Client
    blocks chan BlockEvent
    txs    chan TxEvent  // (Teranode does not currently emit individual tx events; channel
                         //  reserved for future use; SP5+ relies on subtree/block events)
    nodeStatus chan NodeStatusEvent
    logger *slog.Logger
}

func NewNotificationClient(rawURL string, logger *slog.Logger) (*NotificationClient, error)

func (c *NotificationClient) Connect(ctx context.Context) error
func (c *NotificationClient) Close() error
func (c *NotificationClient) Blocks() <-chan BlockEvent
func (c *NotificationClient) Subtrees() <-chan SubtreeEvent
func (c *NotificationClient) NodeStatus() <-chan NodeStatusEvent

type BlockEvent struct {
    Timestamp  time.Time
    Hash       string
    Height     uint64
    BaseURL    string
    PeerID     string
    ClientName string
}
// SubtreeEvent, NodeStatusEvent: structs matching `services/p2p/server_helpers.go` shapes.
```

`Connect` blocks until the underlying centrifuge client is `connected` or ctx is done.
Internal goroutine pumps centrifuge publications into the typed channels.

### 4.4 `internal/teranode/p2p_probe.go`

Two probe modes per SP2 discovery:

```go
type P2PProbe struct {
    legacyAddr string  // host:port for Bitcoin P2P (8333/18333/etc.)
    libp2pAddr string  // host:port for libp2p (9905)
    logger     *slog.Logger
}

func NewP2PProbe(legacyAddr, libp2pAddr string, logger *slog.Logger) *P2PProbe

// LegacyHandshake performs a full BSV-wire version/verack exchange using the magic bytes
// for the given network. Returns the peer's reported version, services, user-agent, and
// best-known height. Sends user-agent containing "BSV" to avoid the upstream user-agent
// filter at services/legacy/peer_server.go:541.
func (p *P2PProbe) LegacyHandshake(ctx context.Context, network string) (PeerInfo, error)

// Libp2pPortOpen does a TCP SYN to the libp2p multiaddr's underlying host:port. Sufficient
// for "is the process listening?"; full multistream-select / noise / yamux negotiation is
// deferred to SP9 if a deeper test ever needs it.
func (p *P2PProbe) Libp2pPortOpen(ctx context.Context) error
```

`network` is one of `mainnet`, `testnet`, `regtest`, `teratestnet` mapping to the magic-byte
constants discovered in SP2 (`go-wire@v1.0.6/protocol.go:169-199`).

Wire serialisation: hand-rolled for `MsgVersion` and `MsgVerAck` to avoid the dep on
`go-wire`. Both messages are short and well-specified. ~120 LoC including tests.

### 4.5 `internal/teranode/metrics.go`

Prometheus text exposition parser, no third-party dep:

```go
type MetricsScraper struct {
    url    *url.URL
    http   *http.Client
    logger *slog.Logger
}

func NewMetricsScraper(rawURL string, logger *slog.Logger) (*MetricsScraper, error)

// Scrape returns parsed metric families keyed by name. Implements just enough of the
// Prometheus text format to extract gauges, counters, and histograms with labels.
// ~150 LoC parser; OK for OPS-3 plus PERF-1 resource-usage sampling.
func (m *MetricsScraper) Scrape(ctx context.Context) (map[string]MetricFamily, error)

type MetricFamily struct {
    Name    string
    Help    string
    Type    string  // gauge | counter | histogram | summary
    Samples []Sample
}

type Sample struct {
    Labels map[string]string
    Value  float64
}

// Convenience helpers for OPS-3:
func (m *MetricsScraper) BestBlockHeight(ctx context.Context) (uint64, error)
func (m *MetricsScraper) FSMState(ctx context.Context) (uint64, error)
func (m *MetricsScraper) CatchupActive(ctx context.Context) (bool, error)
```

### 4.6 `internal/teranode/health.go`

```go
type HealthProbe struct {
    base   *url.URL
    http   *http.Client
    logger *slog.Logger
}

func NewHealthProbe(rawURL string, logger *slog.Logger) (*HealthProbe, error)

// Readiness returns the parsed JSON body. Per discovery, response Content-Type is
// "text/plain; charset=utf-8" but body is JSON; the client tolerates either header.
func (h *HealthProbe) Readiness(ctx context.Context) (HealthReport, error)
func (h *HealthProbe) Liveness(ctx context.Context) (HealthReport, error)

type HealthReport struct {
    Status   string
    Services []ServiceHealth
}

type ServiceHealth struct {
    Service      string
    Status       string
    Dependencies []DependencyHealth  // string or []DependencyHealth depending on shape
    DependencyText string             // populated when "dependencies" arrives as plain text
}
```

### 4.7 `internal/teranode/clients.go`

```go
type Clients struct {
    RPC          *RPCClient
    REST         *RESTClient
    Notifications *NotificationClient
    P2PProbe     *P2PProbe
    Metrics      *MetricsScraper
    Health       *HealthProbe
}

// NewClients constructs each sub-client from cfg.Teranode. Empty URLs yield nil sub-clients
// so SP5+ tests can skip cleanly.
func NewClients(cfg config.Teranode, logger *slog.Logger) (*Clients, error)
```

### 4.8 `internal/svnode/rpc.go`

bitcoind-compatible JSON-RPC client. Same wire shape as `teranode.RPCClient`:

```go
type RPCClient struct {
    base   *url.URL
    user   string
    pass   string
    http   *http.Client
    logger *slog.Logger
    nextID atomic.Int64
}

func NewRPCClient(rawURL, user, pass string, logger *slog.Logger) (*RPCClient, error)

// Same signature surface as teranode.RPCClient. Methods are identical because Bitcoin SV
// uses the same JSON-RPC 1.0 schema.
func (c *RPCClient) Call(ctx context.Context, method string, params []any, out any) error
// (same convenience wrappers as teranode.RPCClient)
```

Implementation differences from `teranode.RPCClient`:

- SV Node speaks `getmempoolinfo`, `getmempoolentry`, `estimatefee`, `testmempoolaccept` — Teranode does not. The SV Node client exposes these; Teranode's does not.
- Cookie-file auth supported in addition to basic auth (bitcoind convention; cookie file path
  derived from datadir if both `RPCUser` and `RPCPass` are empty).

To avoid duplication: the JSON-RPC framing (request envelope, response decoding, error type)
lives in a shared internal package `internal/jsonrpc/` (~80 LoC). Both `teranode.RPCClient`
and `svnode.RPCClient` import it.

### 4.9 `internal/svnode/zmq.go`

Pure-Go ZMQ subscriber via `github.com/go-zeromq/zmq4`:

```go
type ZMQClient struct {
    blockURL string  // "tcp://host:28332"
    txURL    string  // "tcp://host:28333"
    blockSub zmq4.Socket
    txSub    zmq4.Socket
    blocks   chan BlockNotification
    txs      chan TxNotification
    logger   *slog.Logger
}

func NewZMQClient(blockURL, txURL string, logger *slog.Logger) (*ZMQClient, error)

func (z *ZMQClient) Connect(ctx context.Context) error
func (z *ZMQClient) Close() error
func (z *ZMQClient) Blocks() <-chan BlockNotification
func (z *ZMQClient) Txs() <-chan TxNotification

type BlockNotification struct {
    Hash    [32]byte
    Header  []byte  // 80-byte header
    Sequence uint32
}

type TxNotification struct {
    TxID    [32]byte
    RawTx   []byte
    Sequence uint32
}
```

Topics: `hashblock` (32-byte hash + 4-byte LE sequence) and `rawtx` (full serialised tx +
4-byte LE sequence) per bitcoind ZMQ convention.

### 4.10 `internal/compare/chainstate.go`

Maps backend-specific RPC errors to a small canonical category enum so PC-1, IBD-2 etc. can
compare "did Teranode and SV Node react to this transaction the same way?" without exact
string matching.

```go
package compare

type RejectionCategory string

const (
    CategoryAccepted          RejectionCategory = "ACCEPTED"
    CategoryUTXOSpent         RejectionCategory = "UTXO_SPENT"
    CategoryUTXOMissing       RejectionCategory = "UTXO_MISSING"
    CategoryScriptFailure     RejectionCategory = "SCRIPT_FAILURE"
    CategoryFeeTooLow         RejectionCategory = "FEE_TOO_LOW"
    CategoryDustOutput        RejectionCategory = "DUST_OUTPUT"
    CategoryNonStandard       RejectionCategory = "NON_STANDARD"
    CategoryConflicting       RejectionCategory = "CONFLICTING"  // alias for double-spend
    CategoryMalformed         RejectionCategory = "MALFORMED"
    CategoryRPCError          RejectionCategory = "RPC_ERROR"     // wire-level / auth / etc.
    CategoryUnknown           RejectionCategory = "UNKNOWN"
)

// CategorizeTeranode maps a Teranode RPC error to a canonical category.
// Teranode error codes per services/rpc/bsvjson/jsonrpcerr.go.
func CategorizeTeranode(err error) RejectionCategory

// CategorizeSVNode maps an SV Node RPC error to the same canonical category.
func CategorizeSVNode(err error) RejectionCategory

// CompareCategories returns true iff both backends produced the same category
// (or both accepted).
func CompareCategories(teranodeErr, svnodeErr error) (matched bool, teranodeCat, svnodeCat RejectionCategory)
```

The mapping table is ~40 cases each, derived from:
- Teranode: `errors/error.pb.go` enum (e.g. `ERR_UTXO_SPENT=70` → `UTXO_SPENT`).
- SV Node: `bsvjson/jsonrpcerr.go` codes plus message-substring matching for cases where the
  code is generic.

## 5. New external dependencies

Two deps added beyond build-doc §5's allow-list of 4 (`libsv/go-bt/v2`, `libsv/go-bk`,
`gopkg.in/yaml.v3`, `nhooyr.io/websocket`). Both are in the product binary (not just
tooling), so they must be tracked:

| Dep | Purpose | Why not stdlib | Choice |
|---|---|---|---|
| `github.com/centrifugal/centrifuge-go` v0.10.x | Centrifuge protocol v2 client | Custom protocol, ~250 LoC to reimplement, upstream Teranode itself uses this | per user (Q1) |
| `github.com/go-zeromq/zmq4` v0.17.x | bitcoind ZMQ subscriber | Pure-Go, no cgo; libzmq is a C library otherwise | per user (Q2) |

`nhooyr.io/websocket` is now redundant (the only WebSocket consumer was notifications, which
moves to `centrifuge-go`). Drop it from `go.mod` to keep the dep list accurate. Net dep
change: +2 -0 (drop -1 unused).

The dep-allow-list in build doc §5 was speculative; SP2 surfaced the real protocols. The
spec is updated to reflect reality. The README's "Dependencies" section will be regenerated
by SP10 from `go.mod`.

## 6. Wiring into `Env`

`internal/testrunner/types.go` placeholder interfaces become concrete struct pointers:

```go
type Env struct {
    Cfg      config.Config
    Logger   *slog.Logger
    Now      func() time.Time
    Manifest matrix.Manifest

    Teranode *teranode.Clients  // nil if cfg.Teranode.RPCURL is empty
    SVNode   *svnode.Clients    // nil if cfg.SVNode.RPCURL is empty
    TxGen    TxGenerator        // populated by SP4
}
```

This is a breaking change to `testrunner.NewEnv`'s signature — the import path is added but
the constructor's signature is unchanged (it still takes `cfg, logger, manifest, now`). The
`Env` struct field types change from interface to concrete `*Clients`.

`cmd/teranode-acceptance/main.go` constructs the clients after `config.Load`:

```go
teranodeClients, err := teranode.NewClients(cfg.Teranode, logger)
if err != nil { ... }
svnodeClients, err := svnode.NewClients(cfg.SVNode, logger)
if err != nil { ... }
env := testrunner.NewEnv(cfg, logger, manifest, time.Now)
env.Teranode = teranodeClients
env.SVNode   = svnodeClients
```

## 7. Verification & testing strategy

### Layer 1 — unit tests (`go test -race ./...`)

| Package | Mechanism | Coverage target |
|---|---|---|
| `internal/jsonrpc` | golden request/response shapes; error decoding | ≥85% |
| `internal/teranode/rpc` | `httptest.Server` returning canned JSON-RPC responses; request body assertions; error mapping | ≥80% |
| `internal/teranode/rest` | `httptest.Server` per endpoint; binary / hex / json suffix variants | ≥80% |
| `internal/teranode/notifications` | Faked Centrifuge client — inject a `centrifuge.Client` mock or use a minimal in-process Centrifuge server | ≥70% |
| `internal/teranode/p2p_probe` | Faked `net.Listener` accepting Bitcoin wire protocol; tests serialise expected bytes | ≥80% |
| `internal/teranode/metrics` | `httptest.Server` returning Prometheus text fixtures (testdata/*.prom) | ≥85% |
| `internal/teranode/health` | `httptest.Server` returning JSON fixtures including the `text/plain` content-type quirk | ≥85% |
| `internal/svnode/rpc` | `httptest.Server` with bitcoind-style responses; cookie-file auth path | ≥80% |
| `internal/svnode/zmq` | In-process zmq4 PUB socket sending fixture frames | ≥70% |
| `internal/compare` | Table-driven mapping tests (~40 fixtures per backend) | ≥95% |

`testdata/` directories under each package hold the canned response fixtures. Real
upstream paths and line numbers from `docs/discovery.md` inform the fixtures so they don't
drift from reality.

### Layer 2 — integration (no live network in SP3)

`cmd/teranode-acceptance/main_test.go` integration scenarios from SP1 still pass; no new
scenarios in SP3. Live end-to-end is deferred.

### Layer 3 — static checks (`make lint`)

`gofmt -l`, `go vet`, `staticcheck` clean.

### Layer 4 — SP3 done-check (`scripts/sp3-done-check.sh`)

Mechanical:

```bash
make build lint test verify
go test -race -coverprofile=cov.out ./internal/teranode/... ./internal/svnode/... ./internal/compare/...
go tool cover -func=cov.out | grep -v '^total:' | awk '{ if ($3+0 < 70.0) print }' | (! grep .)
# Coverage on each public function ≥70%; total enforced separately at ≥80%.
```

## 8. Definition of done

- All ten files in `internal/teranode/`, both files in `internal/svnode/`, and
  `internal/compare/chainstate.go` exist with their accompanying `*_test.go`.
- `make build lint test verify` exits 0.
- Coverage targets met per §7.
- `cmd/teranode-acceptance/main.go` constructs the clients and attaches them to `Env`.
- Running the CLI against an example config produces no functional change to the SP1
  zero-test report (still INCOMPLETE, exit 3, 58 rows) — the clients exist but no tests
  use them yet.
- `scripts/sp3-done-check.sh` exits 0.
- Code review (`superpowers:code-reviewer`) approves the deliverables.

## 9. Tracked risks

| # | Risk | Mitigation |
|---|---|---|
| A | `centrifuge-go` API drift (it's pre-1.0) | Pin minor version in `go.mod`; cover with table tests; if upstream breaks, re-run SP3 review. |
| B | `go-zeromq/zmq4` is less battle-tested than libzmq | Use only PUB/SUB topic semantics (well-supported); fall back to polling for INTER-1/INTER-2 if ZMQ proves flaky in SP9 (already the spec's plan B). |
| C | Centrifuge auto-subscribe behaviour means we can't unsubscribe | Document; SP5+ tests filter at the consumer side. |
| D | `metrics.go` Prometheus parser handles only basic format | Document the unsupported edge cases (exemplars, OpenMetrics-specific extensions); OPS-3 / PERF-1 don't need them. |
| E | SV Node ZMQ assumes bitcoind wire format unchanged | Reasonable assumption for SVNode; failure surfaces cleanly via tests in SP9. |
| F | `internal/jsonrpc` shared package becomes a leaky abstraction | Keep it minimal (request envelope + error type); both clients can override per-method behaviour. |
| G | Two new external deps may pull transitive deps that conflict with stdlib choices | Run `go mod tidy` and audit `go.sum` after dep additions. |

## 10. Decisions locked

| # | Decision | Note |
|---|---|---|
| 1 | `centrifuge-go` added as 5th dep | per user (Q1=A) |
| 2 | `go-zeromq/zmq4` added as 6th dep | per user (Q2=A) |
| 3 | Dedicated `metrics.go` and `health.go` packages in SP3 | per user (Q3=A) |
| 4 | `nhooyr.io/websocket` dropped (no longer used) | drafter — keeps deps accurate |
| 5 | Shared `internal/jsonrpc` package between teranode and svnode RPC clients | drafter — DRY |
| 6 | `Env.Teranode` / `Env.SVNode` become concrete `*Clients` (no interface indirection) | drafter — interfaces are SP1 placeholders only |
| 7 | Hand-rolled BSV wire `MsgVersion` / `MsgVerAck` for P2P probe | drafter — avoids `go-wire` dep |
| 8 | Hand-rolled Prometheus text parser (~150 LoC) | drafter — avoids `prometheus/client_model` dep |
| 9 | Cookie-file auth supported on SV Node only (Teranode does not implement) | drafter — matches SP2 finding |
| 10 | Shared HTTP client across all sub-clients with 30s default timeout | drafter — `Clients.HTTPTimeout` configurable |

## 11. Out-of-scope reminders

- No live network calls in SP3 tests. Everything is `httptest` / faked.
- No changes to the `internal/matrix` package, `config` package validation, or the runner /
  reporter code paths from SP1.
- No tests in `tests/`. SP5 onwards.
