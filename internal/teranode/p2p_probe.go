package teranode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"
)

// P2PProbe issues TCP probes against Teranode's two P2P listener
// surfaces: the legacy BSV-wire listener (port 8333/18333/...) and the
// libp2p TCP listener (port 9905). Discovery: docs/discovery.md §4.
type P2PProbe struct {
	legacyAddr string
	libp2pAddr string
	logger     *slog.Logger
}

func NewP2PProbe(legacyAddr, libp2pAddr string, logger *slog.Logger) *P2PProbe {
	if logger == nil {
		logger = slog.Default()
	}
	return &P2PProbe{legacyAddr: legacyAddr, libp2pAddr: libp2pAddr, logger: logger}
}

// PeerInfo is the subset of the Bitcoin version message we surface.
type PeerInfo struct {
	ProtocolVersion int32
	Services        uint64
	UserAgent       string
	StartingHeight  int32
}

// Magic bytes per network — sourced from go-wire@v1.0.6/protocol.go:178-187
// (see docs/discovery.md §4).
var networkMagic = map[string]uint32{
	"mainnet":     0xe8f3e1e3,
	"testnet":     0xf4f3e5f4,
	"regtest":     0xfabfb5da,
	"teratestnet": 0x0c09010d,
}

// LegacyHandshake performs a Bitcoin P2P version/verack exchange.
func (p *P2PProbe) LegacyHandshake(ctx context.Context, network string) (PeerInfo, error) {
	if p.legacyAddr == "" {
		return PeerInfo{}, errors.New("legacy P2P address not configured")
	}
	magic, ok := networkMagic[network]
	if !ok {
		return PeerInfo{}, fmt.Errorf("unknown network %q", network)
	}

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", p.legacyAddr)
	if err != nil {
		return PeerInfo{}, fmt.Errorf("dial %s: %w", p.legacyAddr, err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	}

	// Send our version message.
	ver := buildVersionMessage(magic)
	if _, err := conn.Write(ver); err != nil {
		return PeerInfo{}, fmt.Errorf("write version: %w", err)
	}

	// Read the peer's version, then verack.
	pi, err := readVersionMessage(conn, magic)
	if err != nil {
		return PeerInfo{}, fmt.Errorf("read peer version: %w", err)
	}

	// Send verack.
	verack := buildEmptyMessage(magic, "verack")
	if _, err := conn.Write(verack); err != nil {
		return PeerInfo{}, fmt.Errorf("write verack: %w", err)
	}
	return pi, nil
}

// Libp2pPortOpen does a TCP SYN to the libp2p host:port.
func (p *P2PProbe) Libp2pPortOpen(ctx context.Context) error {
	if p.libp2pAddr == "" {
		return errors.New("libp2p address not configured")
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", p.libp2pAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", p.libp2pAddr, err)
	}
	_ = conn.Close()
	return nil
}

// --- wire helpers ---
//
// Bitcoin P2P message header layout (24 bytes):
//
//	magic   uint32 LE
//	command [12]byte (NUL-padded ASCII)
//	length  uint32 LE
//	chksum  uint32 (first 4 bytes of double-SHA256 of payload — for our
//	        outgoing version with empty-ish payload we use 0x5df6e0e2,
//	        the well-known checksum for an empty payload; for non-empty
//	        we compute it).
func writeMsgHeader(buf *bytes.Buffer, magic uint32, command string, payload []byte) {
	_ = binary.Write(buf, binary.LittleEndian, magic)
	cmdBytes := make([]byte, 12)
	copy(cmdBytes, command)
	buf.Write(cmdBytes)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(payload)))
	chk := doubleSHA256(payload)
	buf.Write(chk[:4])
	buf.Write(payload)
}

func buildVersionMessage(magic uint32) []byte {
	var pl bytes.Buffer
	_ = binary.Write(&pl, binary.LittleEndian, int32(70016))               // protocol version
	_ = binary.Write(&pl, binary.LittleEndian, uint64(1))                  // services: NODE_NETWORK
	_ = binary.Write(&pl, binary.LittleEndian, time.Now().Unix())          // timestamp
	pl.Write(make([]byte, 26))                                             // addr_recv (skipped)
	pl.Write(make([]byte, 26))                                             // addr_from (skipped)
	_ = binary.Write(&pl, binary.LittleEndian, uint64(0xdeadbeefcafebabe)) // nonce
	ua := "/tng-acceptance-bsv:0.1.0/"                                     // user agent (must contain "BSV")
	pl.WriteByte(byte(len(ua)))
	pl.WriteString(ua)
	_ = binary.Write(&pl, binary.LittleEndian, int32(0)) // start height
	pl.WriteByte(0x00)                                   // relay tx

	var buf bytes.Buffer
	writeMsgHeader(&buf, magic, "version", pl.Bytes())
	return buf.Bytes()
}

func buildEmptyMessage(magic uint32, cmd string) []byte {
	var buf bytes.Buffer
	writeMsgHeader(&buf, magic, cmd, nil)
	return buf.Bytes()
}

func readVersionMessage(r io.Reader, magic uint32) (PeerInfo, error) {
	var pi PeerInfo
	for {
		hdr := make([]byte, 24)
		if _, err := io.ReadFull(r, hdr); err != nil {
			return pi, fmt.Errorf("read header: %w", err)
		}
		gotMagic := binary.LittleEndian.Uint32(hdr[0:4])
		if gotMagic != magic {
			return pi, fmt.Errorf("magic mismatch: want %x got %x", magic, gotMagic)
		}
		cmd := string(bytes.TrimRight(hdr[4:16], "\x00"))
		pllen := binary.LittleEndian.Uint32(hdr[16:20])
		payload := make([]byte, pllen)
		if pllen > 0 {
			if _, err := io.ReadFull(r, payload); err != nil {
				return pi, fmt.Errorf("read payload for %s: %w", cmd, err)
			}
		}
		if cmd == "version" {
			return parseVersionPayload(payload)
		}
		// Other messages (sendaddrv2, sendcmpct, etc.) — ignore until version arrives.
	}
}

func parseVersionPayload(p []byte) (PeerInfo, error) {
	var pi PeerInfo
	if len(p) < 86 {
		return pi, fmt.Errorf("version payload too short: %d", len(p))
	}
	pi.ProtocolVersion = int32(binary.LittleEndian.Uint32(p[0:4]))
	pi.Services = binary.LittleEndian.Uint64(p[4:12])
	// timestamp 12:20, addr_recv 20:46, addr_from 46:72, nonce 72:80
	uaLen := int(p[80])
	if 81+uaLen > len(p) {
		return pi, fmt.Errorf("ua len %d out of bounds", uaLen)
	}
	pi.UserAgent = string(p[81 : 81+uaLen])
	off := 81 + uaLen
	if off+4 > len(p) {
		return pi, fmt.Errorf("starting height truncated")
	}
	pi.StartingHeight = int32(binary.LittleEndian.Uint32(p[off : off+4]))
	return pi, nil
}

// doubleSHA256 returns the Bitcoin double-SHA256 of b.
func doubleSHA256(b []byte) [32]byte {
	first := sha256Sum(b)
	return sha256Sum(first[:])
}

// sha256Sum is a tiny wrapper to keep the import block minimal in tests.
func sha256Sum(b []byte) [32]byte {
	return shaSum(b)
}
