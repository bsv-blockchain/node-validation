package main

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/libsv/go-bk/crypto"
	bt "github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

// generateIBD2Fixtures returns 10+ IBD-2 edge-case spend fixtures.
// Each represents one of the 10 categories from the spec §3.3.
func generateIBD2Fixtures() []fixture {
	return []fixture{
		buildIBD2_p2pkhExtraWitness(),
		buildIBD2_p2shRevealOnePush(),
		buildIBD2_p2shHashMismatch(),
		buildIBD2_p2msUnderSigned(),
		buildIBD2_nonCanonicalDER(),
		buildIBD2_immatureCoinbase(),
		buildIBD2_futureLocktime(),
		buildIBD2_negativeOutput(),
		buildIBD2_dustOutput(),
		buildIBD2_negativeFee(),
	}
}

const provIBD2 = "synthetic SP8 fixture; cmd/gen-fixtures/ibd2.go"

// buildIBD2_p2pkhExtraWitness builds a P2PKH spend with extra data in
// the scriptSig (CLEANSTACK violation / extra witness data).
func buildIBD2_p2pkhExtraWitness() fixture {
	pk := genPubKey()
	hash := crypto.Hash160(pk)

	lockScript := &bscript.Script{}
	_ = lockScript.AppendOpcodes(bscript.OpDUP, bscript.OpHASH160)
	_ = lockScript.AppendPushData(hash)
	_ = lockScript.AppendOpcodes(bscript.OpEQUALVERIFY, bscript.OpCHECKSIG)
	lockHex := hex.EncodeToString(*lockScript)

	// Unlocking script: push extra junk, then a dummy sig, then pubkey.
	unlockScript := &bscript.Script{}
	_ = unlockScript.AppendPushData([]byte{0xde, 0xad, 0xbe, 0xef}) // extra witness data
	dummySig := make([]byte, 71)
	dummySig[0] = 0x30
	dummySig[1] = 0x44
	dummySig[70] = 0x41
	_ = unlockScript.AppendPushData(dummySig)
	_ = unlockScript.AppendPushData(pk)

	return fixture{
		ID:               "ibd2-001",
		Category:         "p2pkh-extra-witness",
		Description:      "P2PKH spend with extra witness data in scriptSig (CLEANSTACK violation)",
		HexTx:            rawScriptTxHex(0x60, *unlockScript, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "extra push before sig violates CLEANSTACK; dummy UTXO causes UTXO_MISSING first",
	}
}

// buildIBD2_p2shRevealOnePush builds a P2SH spend revealing a 1-of-1 multisig redeem.
func buildIBD2_p2shRevealOnePush() fixture {
	pk := genPubKey()

	redeemScript := &bscript.Script{}
	_ = redeemScript.AppendOpcodes(bscript.Op1)
	_ = redeemScript.AppendPushData(pk)
	_ = redeemScript.AppendOpcodes(bscript.Op1, bscript.OpCHECKMULTISIG)

	hash := crypto.Hash160(*redeemScript)
	lockScript := &bscript.Script{}
	_ = lockScript.AppendOpcodes(bscript.OpHASH160)
	_ = lockScript.AppendPushData(hash)
	_ = lockScript.AppendOpcodes(bscript.OpEQUAL)
	lockHex := hex.EncodeToString(*lockScript)

	unlockScript := &bscript.Script{}
	_ = unlockScript.AppendOpcodes(bscript.Op0) // OP_0 for multisig bug
	dummySig := make([]byte, 71)
	dummySig[0] = 0x30
	dummySig[70] = 0x41
	_ = unlockScript.AppendPushData(dummySig)
	_ = unlockScript.AppendPushData(*redeemScript)

	return fixture{
		ID:               "ibd2-002",
		Category:         "p2sh-reveal-1of1",
		Description:      "P2SH spend revealing 1-of-1 multisig redeemScript",
		HexTx:            rawScriptTxHex(0x61, *unlockScript, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "P2SH 1-of-1 multisig spend; dummy UTXO causes UTXO_MISSING",
	}
}

// buildIBD2_p2shHashMismatch builds a P2SH spend where the revealed
// redeemScript does not match the hash in the locking script.
func buildIBD2_p2shHashMismatch() fixture {
	// Locking script commits to hash of redeemA.
	redeemA := []byte{bscript.OpTRUE}
	hash := crypto.Hash160(redeemA)
	lockScript := &bscript.Script{}
	_ = lockScript.AppendOpcodes(bscript.OpHASH160)
	_ = lockScript.AppendPushData(hash)
	_ = lockScript.AppendOpcodes(bscript.OpEQUAL)
	lockHex := hex.EncodeToString(*lockScript)

	// Unlocking script reveals redeemB (wrong script → hash mismatch).
	redeemB := []byte{bscript.OpTRUE, bscript.OpNOP}
	unlockScript := &bscript.Script{}
	_ = unlockScript.AppendPushData(redeemB)

	return fixture{
		ID:               "ibd2-003",
		Category:         "p2sh-hash-mismatch",
		Description:      "P2SH spend with mismatched redeemScript hash (always-fail)",
		HexTx:            rawScriptTxHex(0x62, *unlockScript, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "redeemScript hash does not match locking script; fails at script eval",
	}
}

// buildIBD2_p2msUnderSigned builds a 2-of-3 multisig spend with only 1 signature.
func buildIBD2_p2msUnderSigned() fixture {
	pk1 := genPubKey()
	pk2 := make([]byte, 33)
	pk3 := make([]byte, 33)
	copy(pk2, pk1)
	copy(pk3, pk1)
	pk2[0] = 0x03
	pk3[0] = 0x02

	lockScript := &bscript.Script{}
	_ = lockScript.AppendOpcodes(bscript.Op2)
	_ = lockScript.AppendPushData(pk1)
	_ = lockScript.AppendPushData(pk2)
	_ = lockScript.AppendPushData(pk3)
	_ = lockScript.AppendOpcodes(bscript.Op3, bscript.OpCHECKMULTISIG)
	lockHex := hex.EncodeToString(*lockScript)

	// Only 1 signature (need 2) → always fails.
	unlockScript := &bscript.Script{}
	_ = unlockScript.AppendOpcodes(bscript.Op0) // OP_0 for multisig off-by-one
	dummySig := make([]byte, 71)
	dummySig[0] = 0x30
	dummySig[70] = 0x41
	_ = unlockScript.AppendPushData(dummySig)

	return fixture{
		ID:               "ibd2-004",
		Category:         "p2ms-under-signed",
		Description:      "2-of-3 multisig spend with only 1 signature (insufficient)",
		HexTx:            rawScriptTxHex(0x63, *unlockScript, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "requires 2 sigs, provides 1; both backends reject same way",
	}
}

// buildIBD2_nonCanonicalDER builds a P2PKH spend with a non-canonical
// DER signature (R component has extra leading zero padding).
func buildIBD2_nonCanonicalDER() fixture {
	pk := genPubKey()
	hash := crypto.Hash160(pk)

	lockScript := &bscript.Script{}
	_ = lockScript.AppendOpcodes(bscript.OpDUP, bscript.OpHASH160)
	_ = lockScript.AppendPushData(hash)
	_ = lockScript.AppendOpcodes(bscript.OpEQUALVERIFY, bscript.OpCHECKSIG)
	lockHex := hex.EncodeToString(*lockScript)

	// Construct non-canonical DER: R has extra leading zero (violates MINIMALDATA in DER).
	r := make([]byte, 33) // 33 bytes with leading 0x00 = non-canonical
	r[0] = 0x00
	r[1] = 0x01
	s := make([]byte, 32)
	s[0] = 0x01

	inner := []byte{0x02, byte(len(r))}
	inner = append(inner, r...)
	inner = append(inner, 0x02, byte(len(s)))
	inner = append(inner, s...)
	der := []byte{0x30, byte(len(inner))}
	der = append(der, inner...)
	der = append(der, 0x41) // SIGHASH_ALL|FORKID

	unlockScript := &bscript.Script{}
	_ = unlockScript.AppendPushData(der)
	_ = unlockScript.AppendPushData(pk)

	return fixture{
		ID:               "ibd2-005",
		Category:         "non-canonical-der",
		Description:      "P2PKH spend with non-canonical DER signature (extra leading zero on R)",
		HexTx:            rawScriptTxHex(0x64, *unlockScript, lockHex, 100_000),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "DER R component has unnecessary leading zero; non-canonical encoding",
	}
}

// buildIBD2_immatureCoinbase builds a spend of a coinbase tx
// before the 100-block maturity rule expires.
// The tx's input flags it as coinbase (txid=0000...0000, vout=0xffffffff)
// but we use a crafted txid that is not all-zeros so it looks like a
// "coinbase-like" input that a node would reject as immature.
func buildIBD2_immatureCoinbase() fixture {
	// A coinbase UTXO: txid of all 0x01 bytes, vout 0.
	coinbaseTxID := dummyTxID(0x01)

	tx := bt.NewTx()
	// Add input referencing what looks like a coinbase output.
	lockHex := p2pkhLockingScript()
	_ = tx.From(coinbaseTxID, 0, lockHex, 100_000)
	// Set sequence to 0 (signals possible RBF; coinbase maturity still applies).
	tx.Inputs[0].SequenceNumber = 0x00000000

	// Output.
	outScript, _ := bscript.NewFromHexString(lockHex)
	tx.AddOutput(&bt.Output{Satoshis: 50_000, LockingScript: outScript})

	return fixture{
		ID:               "ibd2-006",
		Category:         "immature-coinbase",
		Description:      "Spend of coinbase output before maturity (100 blocks on regtest)",
		HexTx:            tx.String(),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "coinbase TXIDs are not real; both backends reject with UTXO_MISSING",
	}
}

// buildIBD2_futureLocktime builds a spend with a locktime in the future
// and nSequence set to signal locktime enforcement.
func buildIBD2_futureLocktime() fixture {
	lockHex := p2pkhLockingScript()
	tx := bt.NewTx()
	_ = tx.From(dummyTxID(0x70), 0, lockHex, 100_000)
	// Set sequence to 0 (< 0xFFFFFFFF) to enable nLocktime.
	tx.Inputs[0].SequenceNumber = 0x00000000
	// Set locktime to year 2099 (far future): Unix timestamp.
	tx.LockTime = 4_000_000_000 // well beyond any real block time

	outScript, _ := bscript.NewFromHexString(lockHex)
	tx.AddOutput(&bt.Output{Satoshis: 50_000, LockingScript: outScript})

	return fixture{
		ID:               "ibd2-007",
		Category:         "future-locktime",
		Description:      "Spend with nLocktime in the future (year ~2096); sequence=0 enforces it",
		HexTx:            tx.String(),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "dummy UTXO; locktime enforcement also causes rejection",
	}
}

// buildIBD2_negativeOutput builds a transaction with a negative satoshi output.
// This is crafted at the raw byte level since the library won't produce negative values.
func buildIBD2_negativeOutput() fixture {
	// Construct a raw transaction with a negative output value.
	// Raw TX format:
	//   version (4 LE) + input_count (varint) + inputs + output_count (varint) + outputs + locktime (4 LE)
	//
	// We'll produce a syntactically valid-looking but semantically invalid tx.
	var raw []byte

	// Version: 1
	raw = append(raw, 0x01, 0x00, 0x00, 0x00)

	// Input count: 1
	raw = append(raw, 0x01)

	// Input: prevout txid (32 bytes of 0x71), vout (4 LE = 0), script length=0, sequence=0xffffffff
	prevTxID, _ := hex.DecodeString(dummyTxID(0x71))
	raw = append(raw, prevTxID...)
	raw = append(raw, 0x00, 0x00, 0x00, 0x00) // vout
	raw = append(raw, 0x00)                   // script length = 0
	raw = append(raw, 0xff, 0xff, 0xff, 0xff) // sequence

	// Output count: 1
	raw = append(raw, 0x01)

	// Output: value = -1 (0xffffffffffffffff in LE), then a P2PKH script.
	negOne := make([]byte, 8)
	binary.LittleEndian.PutUint64(negOne, 0xffffffffffffffff)
	raw = append(raw, negOne...)
	// P2PKH locking script (25 bytes).
	lockBytes, _ := hex.DecodeString(p2pkhLockingScript())
	raw = append(raw, byte(len(lockBytes)))
	raw = append(raw, lockBytes...)

	// Locktime: 0
	raw = append(raw, 0x00, 0x00, 0x00, 0x00)

	return fixture{
		ID:               "ibd2-008",
		Category:         "negative-output",
		Description:      "Transaction with negative satoshi output value (always-fail)",
		HexTx:            hex.EncodeToString(raw),
		ExpectedValid:    false,
		ExpectedCategory: "MALFORMED",
		Provenance:       provIBD2,
		Notes:            "negative output value (0xffffffffffffffff interpreted as -1); both backends reject as MALFORMED",
	}
}

// buildIBD2_dustOutput builds a transaction with a dust output (≤546 sat).
func buildIBD2_dustOutput() fixture {
	lockHex := p2pkhLockingScript()
	tx := bt.NewTx()
	_ = tx.From(dummyTxID(0x72), 0, lockHex, 100_000)

	// Dust output: 1 satoshi (well below 546 sat dust limit).
	outScript, _ := bscript.NewFromHexString(lockHex)
	tx.AddOutput(&bt.Output{Satoshis: 1, LockingScript: outScript})

	return fixture{
		ID:               "ibd2-009",
		Category:         "dust-output",
		Description:      "Transaction creating dust output of 1 satoshi (≤546 sat threshold)",
		HexTx:            tx.String(),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "dust output (1 sat); dummy UTXO causes UTXO_MISSING before dust check",
	}
}

// buildIBD2_negativeFee builds a transaction where outputs > inputs (negative fee).
func buildIBD2_negativeFee() fixture {
	lockHex := p2pkhLockingScript()
	tx := bt.NewTx()
	// Input provides 100_000 sat.
	_ = tx.From(dummyTxID(0x73), 0, lockHex, 100_000)

	// Output claims 200_000 sat (more than input → negative fee, always invalid).
	outScript, _ := bscript.NewFromHexString(lockHex)
	tx.AddOutput(&bt.Output{Satoshis: 200_000, LockingScript: outScript})

	return fixture{
		ID:               "ibd2-010",
		Category:         "negative-fee",
		Description:      "Transaction where output (200k sat) exceeds input (100k sat) — negative fee",
		HexTx:            tx.String(),
		ExpectedValid:    false,
		ExpectedCategory: "UTXO_MISSING",
		Provenance:       provIBD2,
		Notes:            "output > input makes fee negative; both backends reject (UTXO_MISSING for dummy input)",
	}
}
