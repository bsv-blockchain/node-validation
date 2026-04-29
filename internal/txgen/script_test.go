package txgen

import (
	"bytes"
	"testing"

	"github.com/libsv/go-bt/v2/bscript"
)

func TestP2PKHScript_validAddress(t *testing.T) {
	addr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" // Genesis coinbase address (mainnet)
	s, err := P2PKHScript(addr)
	if err != nil {
		t.Fatalf("P2PKHScript: %v", err)
	}
	if len(s) != 25 {
		t.Errorf("p2pkh script len: %d want 25", len(s))
	}
	if s[0] != 0x76 || s[1] != 0xa9 || s[2] != 0x14 || s[23] != 0x88 || s[24] != 0xac {
		t.Errorf("p2pkh structure: % x", s)
	}
}

func TestOpReturnScript_dataPreserved(t *testing.T) {
	data := []byte("hello, BSV")
	s, err := OpReturnScript(data)
	if err != nil {
		t.Fatalf("OpReturnScript: %v", err)
	}
	parsed := bscript.Script(s)
	parts := []byte(parsed)
	if !bytes.Contains(parts, data) {
		t.Errorf("data not in script: % x", parts)
	}
}

func TestOpReturnScript_tooLarge(t *testing.T) {
	_, err := OpReturnScript(make([]byte, opReturnMaxSize+1))
	if err == nil {
		t.Error("expected size error")
	}
}

func TestMultisigScript_2of3(t *testing.T) {
	pks := [][]byte{
		make([]byte, 33), make([]byte, 33), make([]byte, 33),
	}
	for i, pk := range pks {
		pk[0] = 0x02
		pk[32] = byte(i + 1)
	}
	s, err := MultisigScript(2, pks)
	if err != nil {
		t.Fatalf("MultisigScript: %v", err)
	}
	if s[0] != bscript.Op1+1 { // OP_2
		t.Errorf("first byte: %x want %x", s[0], bscript.Op1+1)
	}
	if s[len(s)-1] != bscript.OpCHECKMULTISIG {
		t.Errorf("last byte: %x", s[len(s)-1])
	}
}

func TestMultisigScript_invalidM(t *testing.T) {
	pks := [][]byte{make([]byte, 33), make([]byte, 33)}
	if _, err := MultisigScript(0, pks); err == nil {
		t.Error("m=0 should error")
	}
	if _, err := MultisigScript(3, pks); err == nil {
		t.Error("m>n should error")
	}
}

func TestP2SHScript_structure(t *testing.T) {
	redeem := []byte{0x51} // OP_1
	s, err := P2SHScript(redeem)
	if err != nil {
		t.Fatalf("P2SHScript: %v", err)
	}
	if s[0] != bscript.OpHASH160 {
		t.Errorf("first byte: %x want OP_HASH160", s[0])
	}
	if s[len(s)-1] != bscript.OpEQUAL {
		t.Errorf("last byte: %x want OP_EQUAL", s[len(s)-1])
	}
}
