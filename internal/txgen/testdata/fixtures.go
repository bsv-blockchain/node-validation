// Package testdata provides deterministic fixtures for txgen tests.
// THESE WIFS ARE PUBLIC TEST FIXTURES — DO NOT use them on a live network
// holding real funds.
package testdata

// TestWIFRegtest is a well-known test-fixture WIF (private key = 1, mainnet
// compressed). Used only by txgen unit tests. Public; never used to hold funds.
const TestWIFRegtest = "KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn"

// UTXO holds the raw fields for a synthetic UTXO fixture.
// Callers in the txgen package construct txgen.UTXO from these fields.
type UTXO struct {
	TxID     [32]byte
	Vout     uint32
	Satoshis uint64
	Script   []byte
}

// FundedUTXO returns a synthetic UTXO with the given satoshis.
// txid is deterministic from the seed.
func FundedUTXO(seed byte, satoshis uint64) UTXO {
	var id [32]byte
	for i := range id {
		id[i] = seed
	}
	return UTXO{
		TxID:     id,
		Vout:     0,
		Satoshis: satoshis,
		// P2PKH script for an arbitrary address — the funder's key won't
		// validate, but tests that inject this UTXO and then try to build
		// transactions spending it will use the funder's own address script.
		Script: []byte{0x76, 0xa9, 0x14 /* 20 zero bytes */, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0xac},
	}
}
