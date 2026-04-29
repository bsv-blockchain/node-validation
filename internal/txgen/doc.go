// Package txgen produces signed BSV transactions for the acceptance
// test suite. The Funder owns wallet state (WIF, derived key, address,
// UTXO set) and bootstraps via SV Node sendtoaddress. The Builder
// constructs and signs transactions of any common script shape (P2PKH,
// P2MS multi-sig, P2SH, OP_RETURN data carrier).
//
// SP4 is unit-tests-only. Live broadcast happens in SP5+ via the
// teranode and svnode RPC clients.
package txgen
