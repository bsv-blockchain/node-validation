package teranode

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestP2P_LibP2PPortOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()
	p := NewP2PProbe("", ln.Addr().String(), nil)
	if err := p.Libp2pPortOpen(context.Background()); err != nil {
		t.Errorf("Libp2pPortOpen: %v", err)
	}
}

func TestP2P_LegacyHandshake_unknownNetwork(t *testing.T) {
	p := NewP2PProbe("127.0.0.1:1", "", nil)
	if _, err := p.LegacyHandshake(context.Background(), "neverland"); err == nil {
		t.Fatal("want error for unknown network")
	}
}

func TestP2P_LegacyHandshake_realServerSequence(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan PeerInfo, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		// Read the client version.
		pi, err := readVersionMessage(conn, networkMagic["regtest"])
		if err != nil {
			t.Errorf("server-side read version: %v", err)
			return
		}
		// Send our version reply.
		_, _ = conn.Write(buildVersionMessage(networkMagic["regtest"]))
		// Wait for the client's verack.
		_, _ = readVersionMessage(conn, networkMagic["regtest"]) // accepts any non-version msg too via loop; instead just close
		done <- pi
	}()

	p := NewP2PProbe(ln.Addr().String(), "", nil)
	pi, err := p.LegacyHandshake(context.Background(), "regtest")
	if err != nil {
		t.Fatalf("LegacyHandshake: %v", err)
	}
	if pi.ProtocolVersion != 70016 {
		t.Errorf("protocol version: %d", pi.ProtocolVersion)
	}
	clientPI := <-done
	if clientPI.UserAgent == "" || (clientPI.UserAgent != "/tng-acceptance-bsv:0.1.0/" && len(clientPI.UserAgent) == 0) {
		t.Errorf("server saw user-agent: %q", clientPI.UserAgent)
	}
}
