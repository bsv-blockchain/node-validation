package svnode

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	return addr.Port
}

func TestZMQ_BlocksRoundTrip(t *testing.T) {
	port := freeTCPPort(t)
	endpoint := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	pub := zmq4.NewPub(context.Background())
	if err := pub.Listen(endpoint); err != nil {
		t.Fatalf("pub listen: %v", err)
	}
	defer pub.Close()

	c, _ := NewZMQClient(endpoint, "", nil)
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// Brief wait for SUB to subscribe.
	time.Sleep(50 * time.Millisecond)

	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	seq := make([]byte, 4)
	binary.LittleEndian.PutUint32(seq, 7)
	if err := pub.Send(zmq4.NewMsgFrom([]byte("hashblock"), hash, seq)); err != nil {
		t.Fatalf("pub send: %v", err)
	}

	select {
	case b := <-c.Blocks():
		if b.Sequence != 7 {
			t.Errorf("seq: %d", b.Sequence)
		}
		if b.Hash[31] != 31 {
			t.Errorf("hash: %x", b.Hash[:])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no block received")
	}
}

func TestZMQ_NilOnAllEmpty(t *testing.T) {
	c, err := NewZMQClient("", "", nil)
	if err != nil || c != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", c, err)
	}
}
