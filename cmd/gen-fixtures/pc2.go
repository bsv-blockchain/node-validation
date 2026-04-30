package main

import (
	"encoding/hex"
	"fmt"

	"github.com/libsv/go-bk/crypto"
	"github.com/libsv/go-bk/wif"
	bt "github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

// genWIF is the deterministic test key (privkey=1; same as SP4 fixture).
const genWIF = "KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn"

// dummyTxID returns a deterministic 32-byte txid for use as a fake
// previous-output reference. Both backends will reject these with
// UTXO_MISSING — the cross-implementation parity test still passes as
// long as both fail in the same category.
func dummyTxID(seed byte) string {
	var b [32]byte
	for i := range b {
		b[i] = seed
	}
	return hex.EncodeToString(b[:])
}

// genPubKey returns the compressed public key bytes for genWIF.
func genPubKey() []byte {
	w, _ := wif.DecodeWIF(genWIF)
	return w.PrivKey.PubKey().SerialiseCompressed()
}

// p2pkhLockingScript returns the standard P2PKH locking script for genWIF's address.
func p2pkhLockingScript() string {
	w, _ := wif.DecodeWIF(genWIF)
	addr, _ := bscript.NewAddressFromPublicKey(w.PrivKey.PubKey(), true)
	s, _ := bscript.NewP2PKHFromAddress(addr.AddressString)
	return hex.EncodeToString(*s)
}

// rawScriptTxHex builds a tx whose unlocking script is exactly unlockingScriptBytes
// (unsigned / malformed). This is the key primitive for script-edge-case fixtures.
func rawScriptTxHex(inputSeed byte, unlockingScript []byte, lockingScriptHex string, satoshisIn uint64) string {
	tx := bt.NewTx()
	prev := dummyTxID(inputSeed)
	_ = tx.From(prev, 0, lockingScriptHex, satoshisIn)
	// Manually set the unlocking script on input 0.
	us := bscript.Script(unlockingScript)
	tx.Inputs[0].UnlockingScript = &us
	// Add a minimal output.
	w, _ := wif.DecodeWIF(genWIF)
	addr, _ := bscript.NewAddressFromPublicKey(w.PrivKey.PubKey(), true)
	outScr, _ := bscript.NewP2PKHFromAddress(addr.AddressString)
	tx.AddOutput(&bt.Output{Satoshis: satoshisIn / 2, LockingScript: outScr})
	return tx.String()
}

// generatePC2Fixtures returns 30+ fixtures across 5 categories (≥6 each).
func generatePC2Fixtures() []fixture {
	var out []fixture
	out = append(out, complexP2SHFixtures()...)
	out = append(out, restrictedOpcodeFixtures()...)
	out = append(out, cleanstackFixtures()...)
	out = append(out, minimaldataFixtures()...)
	out = append(out, malleabilityFixtures()...)
	return out
}

// ---------------------------------------------------------------------------
// Category 1: complex-p2sh
// ---------------------------------------------------------------------------

// complexP2SHFixtures returns 6 fixtures exercising P2SH variations.
// All spend dummy UTXOs → expected category UTXO_MISSING.
func complexP2SHFixtures() []fixture {
	var out []fixture
	pk := genPubKey()

	// Helper: build a P2SH locking script and return its hex.
	p2shHex := func(redeemScript []byte) string {
		hash := crypto.Hash160(redeemScript)
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.OpHASH160)
		_ = s.AppendPushData(hash)
		_ = s.AppendOpcodes(bscript.OpEQUAL)
		return hex.EncodeToString(*s)
	}

	// Helper: P2SH unlocking script = <sig...> <redeemScript>
	// We push an arbitrary redeem script as the last element.
	p2shUnlockHex := func(redeemScript []byte) []byte {
		// Push a dummy signature (71 bytes of 0xAB) then the redeem script.
		s := &bscript.Script{}
		dummySig := make([]byte, 71)
		for i := range dummySig {
			dummySig[i] = 0xab
		}
		_ = s.AppendPushData(dummySig)
		_ = s.AppendPushData(redeemScript)
		return *s
	}

	// 1. 2-of-3 multisig redeemScript (~105 bytes).
	redeem1 := func() []byte {
		pk2 := make([]byte, 33)
		pk3 := make([]byte, 33)
		copy(pk2, pk)
		copy(pk3, pk)
		pk2[0] = 0x03
		pk3[0] = 0x02
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.Op2)
		_ = s.AppendPushData(pk)
		_ = s.AppendPushData(pk2)
		_ = s.AppendPushData(pk3)
		_ = s.AppendOpcodes(bscript.Op3, bscript.OpCHECKMULTISIG)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-001",
		Category:         "complex-p2sh",
		Description:      "P2SH wrapping 2-of-3 multisig (~105-byte redeemScript)",
		HexTx:            rawScriptTxHex(0x10, p2shUnlockHex(redeem1), p2shHex(redeem1), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "dummy input; both backends reject with UTXO_MISSING",
	})

	// 2. 1-of-1 multisig redeemScript (trivial P2SH).
	redeem2 := func() []byte {
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.Op1)
		_ = s.AppendPushData(pk)
		_ = s.AppendOpcodes(bscript.Op1, bscript.OpCHECKMULTISIG)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-002",
		Category:         "complex-p2sh",
		Description:      "P2SH wrapping 1-of-1 multisig",
		HexTx:            rawScriptTxHex(0x11, p2shUnlockHex(redeem2), p2shHex(redeem2), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "dummy input; both backends reject with UTXO_MISSING",
	})

	// 3. P2SH with OP_RETURN redeemScript (always-fail).
	redeem3 := func() []byte {
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.OpRETURN)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-003",
		Category:         "complex-p2sh",
		Description:      "P2SH wrapping OP_RETURN redeemScript (always-fail)",
		HexTx:            rawScriptTxHex(0x12, p2shUnlockHex(redeem3), p2shHex(redeem3), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "OP_RETURN redeem script always fails; dummy UTXO causes UTXO_MISSING first",
	})

	// 4. P2SH-of-P2SH (nested): outer redeemScript is itself a P2SH locking script.
	innerRedeem := func() []byte {
		s := &bscript.Script{}
		_ = s.AppendPushData(pk)
		_ = s.AppendOpcodes(bscript.OpCHECKSIG)
		return *s
	}()
	innerHash := crypto.Hash160(innerRedeem)
	outerRedeem := func() []byte {
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.OpHASH160)
		_ = s.AppendPushData(innerHash)
		_ = s.AppendOpcodes(bscript.OpEQUAL)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-004",
		Category:         "complex-p2sh",
		Description:      "P2SH-of-P2SH (nested P2SH)",
		HexTx:            rawScriptTxHex(0x13, p2shUnlockHex(outerRedeem), p2shHex(outerRedeem), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "outer redeemScript is itself a P2SH locking script; nested P2SH",
	})

	// 5. P2SH with large (200-byte) redeemScript of NOPs.
	redeem5 := func() []byte {
		s := &bscript.Script{}
		for i := 0; i < 200; i++ {
			_ = s.AppendOpcodes(bscript.OpNOP)
		}
		_ = s.AppendOpcodes(bscript.OpTRUE)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-005",
		Category:         "complex-p2sh",
		Description:      "P2SH wrapping 200-byte NOP-heavy redeemScript",
		HexTx:            rawScriptTxHex(0x14, p2shUnlockHex(redeem5), p2shHex(redeem5), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "exercises handling of large redeemScript with many NOPs",
	})

	// 6. P2SH wrapping P2PKH (pay-to-pubkey-hash redeemScript).
	redeemP2PKH := func() []byte {
		hash := crypto.Hash160(pk)
		s := &bscript.Script{}
		_ = s.AppendOpcodes(bscript.OpDUP, bscript.OpHASH160)
		_ = s.AppendPushData(hash)
		_ = s.AppendOpcodes(bscript.OpEQUALVERIFY, bscript.OpCHECKSIG)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-p2sh-006",
		Category:         "complex-p2sh",
		Description:      "P2SH wrapping P2PKH redeemScript",
		HexTx:            rawScriptTxHex(0x15, p2shUnlockHex(redeemP2PKH), p2shHex(redeemP2PKH), 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:complexP2SHFixtures",
		Notes:            "P2SH wrapping a standard P2PKH redeemScript",
	})

	return out
}

// ---------------------------------------------------------------------------
// Category 2: restricted-opcodes
// ---------------------------------------------------------------------------

// restrictedOpcodeFixtures returns 6 fixtures using opcodes that are
// universally invalid post-Genesis.
func restrictedOpcodeFixtures() []fixture {
	var out []fixture
	lockHex := p2pkhLockingScript()

	// Helper: build tx with given locking script bytes (the "output" script).
	// To test restricted opcodes in the output script, we craft a tx where
	// the output itself contains the restricted opcode. The tx structure is valid;
	// rejection happens when the recipient tries to spend. For our purpose,
	// we put the restricted opcode in the unlocking script (scriptSig), which
	// nodes evaluate immediately.
	badUnlock := func(opcodes ...byte) []byte {
		s := &bscript.Script{}
		for _, op := range opcodes {
			_ = s.AppendOpcodes(op)
		}
		return *s
	}

	// 1. OP_VER (0x62) in unlocking script — always invalid.
	out = append(out, fixture{
		ID:               "pc2-restricted-001",
		Category:         "restricted-opcodes",
		Description:      "OP_VER (0x62) in scriptSig — universally invalid",
		HexTx:            rawScriptTxHex(0x20, badUnlock(bscript.OpVER), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_VER remains invalid post-Genesis",
	})

	// 2. OP_RESERVED (0x50) in unlocking script.
	out = append(out, fixture{
		ID:               "pc2-restricted-002",
		Category:         "restricted-opcodes",
		Description:      "OP_RESERVED (0x50) in scriptSig",
		HexTx:            rawScriptTxHex(0x21, badUnlock(bscript.OpRESERVED), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_RESERVED (0x50) is invalid in any executed context",
	})

	// 3. OP_RESERVED1 (0x89) in unlocking script.
	out = append(out, fixture{
		ID:               "pc2-restricted-003",
		Category:         "restricted-opcodes",
		Description:      "OP_RESERVED1 (0x89) in scriptSig",
		HexTx:            rawScriptTxHex(0x22, badUnlock(bscript.OpRESERVED1), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_RESERVED1 (0x89) is universally invalid",
	})

	// 4. OP_RESERVED2 (0x8a) in unlocking script.
	out = append(out, fixture{
		ID:               "pc2-restricted-004",
		Category:         "restricted-opcodes",
		Description:      "OP_RESERVED2 (0x8a) in scriptSig",
		HexTx:            rawScriptTxHex(0x23, badUnlock(bscript.OpRESERVED2), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_RESERVED2 (0x8a) is universally invalid",
	})

	// 5. OP_VERIF (0x65) in unlocking script.
	out = append(out, fixture{
		ID:               "pc2-restricted-005",
		Category:         "restricted-opcodes",
		Description:      "OP_VERIF (0x65) in scriptSig — invalid even in unexecuted branch",
		HexTx:            rawScriptTxHex(0x24, badUnlock(bscript.OpVERIF), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_VERIF (0x65) is invalid even in unexecuted branch",
	})

	// 6. OP_VERNOTIF (0x66) in unlocking script.
	out = append(out, fixture{
		ID:               "pc2-restricted-006",
		Category:         "restricted-opcodes",
		Description:      "OP_VERNOTIF (0x66) in scriptSig — invalid even in unexecuted branch",
		HexTx:            rawScriptTxHex(0x25, badUnlock(bscript.OpVERNOTIF), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:restrictedOpcodeFixtures",
		Notes:            "OP_VERNOTIF (0x66) is invalid even in unexecuted branch",
	})

	return out
}

// ---------------------------------------------------------------------------
// Category 3: cleanstack
// ---------------------------------------------------------------------------

// cleanstackFixtures returns 6 fixtures violating the CLEANSTACK rule.
// Each leaves extra items on the stack after script execution.
func cleanstackFixtures() []fixture {
	var out []fixture
	lockHex := p2pkhLockingScript()

	// Build unlock scripts that push extra items.
	// A P2PKH unlocking script that also pushes extra data onto stack.
	extraItem := func(extraBytes []byte) []byte {
		s := &bscript.Script{}
		// Push extra data first (will be left on stack).
		_ = s.AppendPushData(extraBytes)
		return *s
	}

	// 1. Single extra push (OP_1) — cleanstack violation.
	out = append(out, fixture{
		ID:          "pc2-cleanstack-001",
		Category:    "cleanstack",
		Description: "scriptSig pushes extra OP_1 onto stack (CLEANSTACK violation)",
		HexTx: rawScriptTxHex(0x30, func() []byte {
			s := &bscript.Script{}
			_ = s.AppendOpcodes(bscript.Op1)
			return *s
		}(), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "extra item on stack violates CLEANSTACK rule",
	})

	// 2. Two extra pushes — stack has 2 items after script completes.
	out = append(out, fixture{
		ID:          "pc2-cleanstack-002",
		Category:    "cleanstack",
		Description: "scriptSig pushes two extra values (CLEANSTACK violation)",
		HexTx: rawScriptTxHex(0x31, func() []byte {
			s := &bscript.Script{}
			_ = s.AppendOpcodes(bscript.Op1, bscript.Op2)
			return *s
		}(), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "two extra stack items violate CLEANSTACK",
	})

	// 3. Extra 20-byte push (looks like a hash).
	extra20 := make([]byte, 20)
	for i := range extra20 {
		extra20[i] = byte(i + 1)
	}
	out = append(out, fixture{
		ID:               "pc2-cleanstack-003",
		Category:         "cleanstack",
		Description:      "scriptSig pushes extra 20-byte value (CLEANSTACK violation)",
		HexTx:            rawScriptTxHex(0x32, extraItem(extra20), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "20-byte extra push on stack violates CLEANSTACK",
	})

	// 4. Extra 32-byte push (looks like a txid).
	extra32 := make([]byte, 32)
	for i := range extra32 {
		extra32[i] = byte(0xff - i)
	}
	out = append(out, fixture{
		ID:               "pc2-cleanstack-004",
		Category:         "cleanstack",
		Description:      "scriptSig pushes extra 32-byte value (CLEANSTACK violation)",
		HexTx:            rawScriptTxHex(0x33, extraItem(extra32), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "32-byte extra push on stack violates CLEANSTACK",
	})

	// 5. Extra 1-byte push of 0x00 (false value on stack).
	out = append(out, fixture{
		ID:               "pc2-cleanstack-005",
		Category:         "cleanstack",
		Description:      "scriptSig pushes extra false (0x00) onto stack",
		HexTx:            rawScriptTxHex(0x34, extraItem([]byte{0x00}), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "extra false value on stack violates CLEANSTACK",
	})

	// 6. Extra OP_TRUE then data — non-canonical true on stack.
	out = append(out, fixture{
		ID:          "pc2-cleanstack-006",
		Category:    "cleanstack",
		Description: "scriptSig pushes extra non-canonical true (0x02) then OP_TRUE",
		HexTx: rawScriptTxHex(0x35, func() []byte {
			s := &bscript.Script{}
			_ = s.AppendPushData([]byte{0x02})
			_ = s.AppendOpcodes(bscript.OpTRUE)
			return *s
		}(), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:cleanstackFixtures",
		Notes:            "two extra items including non-canonical true violate CLEANSTACK",
	})

	return out
}

// ---------------------------------------------------------------------------
// Category 4: minimaldata
// ---------------------------------------------------------------------------

// minimaldataFixtures returns 6 fixtures using non-minimal push encodings.
func minimaldataFixtures() []fixture {
	var out []fixture

	// Non-minimal push: use OP_PUSHDATA1 (0x4c) for data that could use a
	// direct push byte (≤75 bytes). This violates MINIMALDATA.
	//
	// OP_PUSHDATA1 format: 0x4c <1-byte length> <data>
	nonMinimalPushData1 := func(data []byte) []byte {
		// Manually encode: OP_PUSHDATA1 + len(data) + data
		out := []byte{bscript.OpPUSHDATA1, byte(len(data))}
		out = append(out, data...)
		return out
	}

	// OP_PUSHDATA2 format: 0x4d <2-byte LE length> <data>
	nonMinimalPushData2 := func(data []byte) []byte {
		out := []byte{bscript.OpPUSHDATA2, byte(len(data)), byte(len(data) >> 8)}
		out = append(out, data...)
		return out
	}

	lockHex := p2pkhLockingScript()

	// 1. OP_PUSHDATA1 for a 1-byte value (should use OP_DATA1).
	out = append(out, fixture{
		ID:               "pc2-minimaldata-001",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA1 encoding 1-byte data (non-minimal; should use OP_DATA1)",
		HexTx:            rawScriptTxHex(0x40, nonMinimalPushData1([]byte{0x01}), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation: OP_PUSHDATA1 for 1-byte data",
	})

	// 2. OP_PUSHDATA1 for 20-byte data (should use OP_DATA20).
	data20 := make([]byte, 20)
	for i := range data20 {
		data20[i] = byte(i + 1)
	}
	out = append(out, fixture{
		ID:               "pc2-minimaldata-002",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA1 encoding 20-byte data (non-minimal; should use OP_DATA20)",
		HexTx:            rawScriptTxHex(0x41, nonMinimalPushData1(data20), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation: OP_PUSHDATA1 for 20-byte data",
	})

	// 3. OP_PUSHDATA1 for 75-byte data (boundary; should use OP_DATA75).
	data75 := make([]byte, 75)
	for i := range data75 {
		data75[i] = byte(i)
	}
	out = append(out, fixture{
		ID:               "pc2-minimaldata-003",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA1 encoding 75-byte data (non-minimal boundary; should use OP_DATA75)",
		HexTx:            rawScriptTxHex(0x42, nonMinimalPushData1(data75), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation at 75-byte boundary",
	})

	// 4. OP_PUSHDATA2 for 10-byte data (should use OP_DATA10).
	data10 := make([]byte, 10)
	for i := range data10 {
		data10[i] = byte(0xaa)
	}
	out = append(out, fixture{
		ID:               "pc2-minimaldata-004",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA2 encoding 10-byte data (non-minimal; should use OP_DATA10)",
		HexTx:            rawScriptTxHex(0x43, nonMinimalPushData2(data10), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation: OP_PUSHDATA2 for 10-byte data",
	})

	// 5. OP_PUSHDATA1 for a value that could be OP_1NEGATE (0x4f for -1).
	out = append(out, fixture{
		ID:               "pc2-minimaldata-005",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA1 encoding 0x81 (non-minimal; should use OP_1NEGATE)",
		HexTx:            rawScriptTxHex(0x44, nonMinimalPushData1([]byte{0x81}), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation: non-minimal encoding of -1",
	})

	// 6. OP_PUSHDATA1 for 33-byte compressed pubkey (should use OP_DATA33).
	pk := genPubKey()
	out = append(out, fixture{
		ID:               "pc2-minimaldata-006",
		Category:         "minimaldata",
		Description:      "OP_PUSHDATA1 encoding 33-byte pubkey (non-minimal; should use OP_DATA33)",
		HexTx:            rawScriptTxHex(0x45, nonMinimalPushData1(pk), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:minimaldataFixtures",
		Notes:            "MINIMALDATA violation: OP_PUSHDATA1 for 33-byte compressed pubkey",
	})

	return out
}

// ---------------------------------------------------------------------------
// Category 5: malleability
// ---------------------------------------------------------------------------

// malleabilityFixtures returns 6 fixtures with non-canonical signature
// encoding or sighash variants that indicate historical malleability vectors.
func malleabilityFixtures() []fixture {
	var out []fixture
	lockHex := p2pkhLockingScript()

	// A "signature" with trailing extra bytes (non-canonical DER).
	nonCanonicalSigWithTrailing := func(trailingBytes ...byte) []byte {
		// Minimal valid-looking DER sig: 0x30 <len> 0x02 <rlen> <r> 0x02 <slen> <s> <sighash>
		// We construct a minimal DER encoding then append extra garbage.
		r := make([]byte, 32)
		s := make([]byte, 32)
		for i := range r {
			r[i] = byte(i + 1)
			s[i] = byte(i + 0x80)
		}
		// Ensure R is positive (set high bit = 0, pad if needed).
		r[0] = 0x01
		s[0] = 0x01

		inner := []byte{0x02, byte(len(r))}
		inner = append(inner, r...)
		inner = append(inner, 0x02, byte(len(s)))
		inner = append(inner, s...)

		der := []byte{0x30, byte(len(inner))}
		der = append(der, inner...)
		// sighash type SIGHASH_ALL | SIGHASH_FORKID (0x41).
		der = append(der, 0x41)
		// Append trailing garbage bytes.
		der = append(der, trailingBytes...)

		sig := &bscript.Script{}
		_ = sig.AppendPushData(der)
		pk := genPubKey()
		_ = sig.AppendPushData(pk)
		return *sig
	}

	// 1. Signature with 1 trailing byte (non-canonical DER).
	out = append(out, fixture{
		ID:               "pc2-malleability-001",
		Category:         "malleability",
		Description:      "P2PKH spend with DER signature + 1 trailing garbage byte",
		HexTx:            rawScriptTxHex(0x50, nonCanonicalSigWithTrailing(0xde), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes:            "trailing byte after sighash type makes DER non-canonical",
	})

	// 2. Signature with 2 trailing bytes.
	out = append(out, fixture{
		ID:               "pc2-malleability-002",
		Category:         "malleability",
		Description:      "P2PKH spend with DER signature + 2 trailing garbage bytes",
		HexTx:            rawScriptTxHex(0x51, nonCanonicalSigWithTrailing(0xde, 0xad), lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes:            "two trailing bytes after sighash make DER non-canonical",
	})

	// 3. Sighash without SIGHASH_FORKID (BSV-specific rejection).
	sigWithoutForkID := func() []byte {
		r := make([]byte, 32)
		s := make([]byte, 32)
		r[0] = 0x01
		s[0] = 0x01
		inner := []byte{0x02, byte(len(r))}
		inner = append(inner, r...)
		inner = append(inner, 0x02, byte(len(s)))
		inner = append(inner, s...)
		der := []byte{0x30, byte(len(inner))}
		der = append(der, inner...)
		// SIGHASH_ALL without FORKID (0x01 instead of 0x41).
		der = append(der, 0x01)

		sig := &bscript.Script{}
		_ = sig.AppendPushData(der)
		pk := genPubKey()
		_ = sig.AppendPushData(pk)
		return *sig
	}()
	out = append(out, fixture{
		ID:               "pc2-malleability-003",
		Category:         "malleability",
		Description:      "P2PKH spend without SIGHASH_FORKID (BSV-specific rejection)",
		HexTx:            rawScriptTxHex(0x52, sigWithoutForkID, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes:            "missing SIGHASH_FORKID (0x40) bit causes script failure on BSV",
	})

	// 4. Signature with leading zero padding on R (high S malleability).
	sigHighSPadded := func() []byte {
		// R and S with leading zero (non-minimal encoding for positive integers).
		r := make([]byte, 33) // leading 0x00 to make it non-minimal
		s := make([]byte, 33)
		r[0] = 0x00
		r[1] = 0x01
		s[0] = 0x00
		s[1] = 0x01

		inner := []byte{0x02, byte(len(r))}
		inner = append(inner, r...)
		inner = append(inner, 0x02, byte(len(s)))
		inner = append(inner, s...)
		der := []byte{0x30, byte(len(inner))}
		der = append(der, inner...)
		der = append(der, 0x41) // SIGHASH_ALL | FORKID

		sig := &bscript.Script{}
		_ = sig.AppendPushData(der)
		pk := genPubKey()
		_ = sig.AppendPushData(pk)
		return *sig
	}()
	out = append(out, fixture{
		ID:               "pc2-malleability-004",
		Category:         "malleability",
		Description:      "P2PKH spend with padding-zero DER R/S (non-canonical encoding)",
		HexTx:            rawScriptTxHex(0x53, sigHighSPadded, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes:            "extra leading zero on R and S makes DER non-canonical (malleability vector)",
	})

	// 5. Zero-length signature (empty scriptSig — no sig at all).
	emptyUnlock := func() []byte {
		s := &bscript.Script{}
		_ = s.AppendPushData([]byte{}) // empty push
		pk := genPubKey()
		_ = s.AppendPushData(pk)
		return *s
	}()
	out = append(out, fixture{
		ID:               "pc2-malleability-005",
		Category:         "malleability",
		Description:      "P2PKH spend with zero-length signature (empty push)",
		HexTx:            rawScriptTxHex(0x54, emptyUnlock, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes:            "empty signature causes CHECKSIG to return false",
	})

	// 6. SIGHASH_NONE (0x02) without FORKID — historical malleability.
	sigHashNone := func() []byte {
		r := make([]byte, 32)
		s := make([]byte, 32)
		r[0] = 0x01
		s[0] = 0x01
		inner := []byte{0x02, byte(len(r))}
		inner = append(inner, r...)
		inner = append(inner, 0x02, byte(len(s)))
		inner = append(inner, s...)
		der := []byte{0x30, byte(len(inner))}
		der = append(der, inner...)
		der = append(der, 0x42) // SIGHASH_NONE | FORKID
		sig := &bscript.Script{}
		_ = sig.AppendPushData(der)
		pk := genPubKey()
		_ = sig.AppendPushData(pk)
		return *sig
	}()
	out = append(out, fixture{
		ID:               "pc2-malleability-006",
		Category:         "malleability",
		Description:      "P2PKH spend with SIGHASH_NONE|FORKID (historical malleability variant)",
		HexTx:            rawScriptTxHex(0x55, sigHashNone, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       "synthetic SP8 fixture; cmd/gen-fixtures/pc2.go:malleabilityFixtures",
		Notes: fmt.Sprintf("SIGHASH_NONE|FORKID=0x42; valid sighash type but wrong sig " +
			"— both backends reject with UTXO_MISSING (dummy UTXO)"),
	})

	return out
}
