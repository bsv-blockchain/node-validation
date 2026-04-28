## 4. P2P listener

### Summary

Teranode operates **two separate P2P listener surfaces** on different transports and ports — the source of the "P2P gateway" terminology in project documentation:

1. **Legacy listener** (`services/legacy/`): speaks the **standard BSV-wire P2P protocol**. TCP port **8333** (mainnet), **18333** (testnet/teratestnet), **18444** (regtest). Port comes from `chaincfg.Params.DefaultPort`; overridable via `legacy_config_port`. Performs Bitcoin P2P handshake (`version`/`verack`) using `go-wire` serialisation, protocol version 70016. Peer discovery via DNS seeds (`seed.bitcoinsv.io` mainnet, `testnet-seed.bitcoinsv.io` testnet) plus optional `--addpeer`/`--connect`.
2. **Teranode-native listener** (`services/p2p/`): speaks **libp2p** (GossipSub/Kademlia-DHT), not Bitcoin wire. TCP port **9905** by default (`P2P_PORT = 9905` in `settings.conf`; gRPC/HTTP control plane on 9906). Protocol ID `/teranode/bitcoin/<network>/1.0.0`. Discovery via DHT + bootstrap peers (`teranode-bootstrap.bsvb.tech:9901`); no DNS seeds.

The legacy service translates between standard wire and Teranode's internal Kafka/gRPC architecture — the gateway described in project docs.

### Findings table

| Property | Value | Source ref |
|---|---|---|
| Legacy listen port (mainnet) | TCP 8333 | `go-chaincfg@v1.4.0/params.go:255` |
| Legacy listen port (testnet/teratestnet) | TCP 18333 | `go-chaincfg@v1.4.0/params.go:532,628` |
| Legacy listen port (regtest) | TCP 18444 | `go-chaincfg@v1.4.0/params.go:449` |
| Legacy listen port (STN) | TCP 9333 | `go-chaincfg@v1.4.0/params.go:370` |
| Legacy port override | `legacy_config_port` / `--port` | `services/legacy/config.go:146`, `services/legacy/peer_server.go:2685-2687` |
| Teranode-native P2P port | TCP 9905 (default) | `settings.conf:134` |
| Teranode-native gRPC/HTTP port | 9906 | `settings.conf:133` |
| Mainnet magic bytes | `0xe8f3e1e3` | `go-wire@v1.0.6/protocol.go:178` |
| Testnet magic bytes | `0xf4f3e5f4` | `go-wire@v1.0.6/protocol.go:184` |
| TeraTestNet magic bytes | `0x0c09010d` | `go-wire@v1.0.6/protocol.go:187` |
| RegTestNet magic bytes | `0xfabfb5da` | `go-wire@v1.0.6/protocol.go:181` |
| Wire protocol version (max) | 70016 | `go-wire@v1.0.6/protocol.go:17` |
| Min acceptable protocol version | 209 | `services/legacy/peer/peer.go:44` |
| Version message format | Standard `MsgVersion` | `go-wire@v1.0.6/msg_version.go:29-56` |
| User agent (legacy) | `/teranode-legacy-p2p:0.13.0` | `services/legacy/peer_server.go:87,91`; `services/legacy/version/version.go:19-21` |
| Services bits (full node) | `SFNodeNetwork \| SFNodeBloom \| SFNodeCF \| SFNodeBitcoinCash` | `services/legacy/peer_server.go:63-64` |
| Required services for outbound | `SFNodeNetwork` | `services/legacy/peer_server.go:68` |
| User-agent filter | Inbound peers banned if UA does not contain `"Bitcoin SV"` or `"BSV"` | `services/legacy/peer_server.go:541-549` |
| DNS seeds (mainnet) | `seed.bitcoinsv.io` | `go-chaincfg@v1.4.0/params.go:256-258` |
| DNS seeds (testnet) | `testnet-seed.bitcoinsv.io` | `go-chaincfg@v1.4.0/params.go:533-535` |
| DNS seeds (regtest/STN/teratestnet) | None (empty) | `go-chaincfg@v1.4.0/params.go:371,452,633` |
| DNS seed disable flag | `--nodnsseed` / `DisableDNSSeed` | `services/legacy/config.go:155` |
| libp2p bootstrap peers | `teranode-bootstrap.bsvb.tech:9901` (prod), `teranode-bootstrap-stage.bsvb.tech:9901` (teratestnet/dev) | `settings.conf:812-813` |
| libp2p protocol ID | `/teranode/bitcoin/<network>/1.0.0` | `services/p2p/Server.go:301` |
| libp2p topic prefix | `teranode/bitcoin/1.0.0/<network>` | `go-chaincfg@v1.4.0/params.go:254` |
| libp2p transport | TCP via multiaddr; DHT (Kademlia); GossipSub | `go-p2p-message-bus@v0.1.3/client.go:21-31` |
| libp2p discovery | DHT (server mode) + optional mDNS (default off) + bootstrap + static peers | `services/p2p/Server.go:321-325`; `settings.conf:388,392` |

### Source references

- `go-chaincfg@v1.4.0/params.go:251-363` — `MainNetParams`, `TestNetParams`, `RegressionNetParams`, `TeraTestNetParams`, `StnParams`
- `go-wire@v1.0.6/protocol.go:169-199` — `BitcoinNet` constants; `ServiceFlag` definitions
- `go-wire@v1.0.6/msg_version.go:29-56` — `MsgVersion` struct
- `services/legacy/params.go:49-95` — Maps `chaincfg` to legacy service
- `services/legacy/config.go:144-146,155` — `Listeners`, `DisableDNSSeed`
- `services/legacy/Server.go:304-308,316-325` — `Init()` builds listen addresses from `activeNetParams.DefaultPort`
- `services/legacy/peer_server.go:63-68,87-91,534,541-549,2108,2223-2233,2685-2687,2956-2973` — `defaultServices`, user agent, listener setup
- `services/legacy/version/version.go:19-21` — `AppMajor=0`, `AppMinor=13`, `AppPatch=0`
- `services/legacy/connmgr/seed.go:35-79` — `SeedFromDNS()` iterates `chainParams.DNSSeeds`
- `services/p2p/Server.go:176-179,298-302,316-325,362-367` — `p2pPort` config, `bitcoinProtocolVersion`, `p2pMessageBus.Config`, topic name
- `go-p2p-message-bus@v0.1.3/client.go:21-31` — libp2p imports (libp2p, kad-dht, pubsub, mdns)
- `settings.conf:133-134,370-372,447,809-813,888,913,941-949` — port and peer settings
- `compose/docker-compose-3blasters.yml:174,182` — Docker port mappings confirm 9905

### Gaps / ambiguities

1. **Gateway/translation layer** — described in `services/legacy/Server.go:18-19` but distributed across `peer_server.go`, netsync handlers, Kafka producers; no consolidated file. SP3 must probe each listener separately.
2. **Port 9905 vs 9906** — 9905 is libp2p TCP listener; 9906 is the Echo HTTP / gRPC control plane. Without `p2p_listen_addresses` and `p2p_port` both set, libp2p may auto-detect.
3. **TeraTestNet DNS seeds absent** — relies entirely on static peers / bootstrap multiaddrs.
4. **libp2p protocol version separate from Bitcoin protocol version** — `1.0.0` libp2p, `70016` Bitcoin wire. External BSV nodes touch only the legacy layer.
5. **UPnP** — present (`services/legacy/upnp.go`) but disabled by default.
6. **ProtocolVersion 70016 supports extended messages (>4GB)** — BSV-specific extension; encoding details not documented in repo.

### Implementation notes for SP3

**Legacy listener (port 8333 / 18333 / 18444):**

- Transport: **TCP only** (`net.Listen("tcp4", ...)` and `tcp6`, `peer_server.go:2551-2575`).
- Handshake: **full Bitcoin P2P version handshake required** to avoid disconnect. Probe must reply with valid `version` (protocol ≥209, UA containing `"Bitcoin SV"` or `"BSV"`, otherwise IP gets banned `peer_server.go:541-549`). Then `verack`. Port-open check insufficient.
- Magic bytes per network: mainnet `0xe8f3e1e3`, testnet `0xf4f3e5f4`, teratestnet `0x0c09010d`.
- Services bits: advertise at minimum `SFNodeNetwork`.
- Recommended: use `go-wire` library directly to serialise `MsgVersion` / `MsgVerAck`.

**Teranode-native listener (port 9905):**

- Transport: **TCP via libp2p multiaddr** (multistream-select, noise encryption, yamux mux). Raw TCP fails to negotiate.
- A valid probe requires `go-libp2p` host, connecting to multiaddr `/ip4/<host>/tcp/9905`, verifying protocol ID `/teranode/bitcoin/<network>/1.0.0`.
- For SP3, **port-open check (TCP SYN) is sufficient** to confirm process is listening; full libp2p dial only needed for protocol verification. Consider `go-p2p-message-bus.NewClient` with `bitcoinProtocolVersion` string.
