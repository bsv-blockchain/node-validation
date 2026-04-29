package txgen

// Per-input size estimates in bytes. Conservative.
const (
	sizeP2PKHInput        = 148 // 1-byte script length + 71-byte sig + 1-byte sep + 33-byte pubkey + 36-byte outpoint + 4-byte sequence
	sizePerOutputOverhead = 9   // 8-byte value + 1-byte VarInt assuming script ≤ 252 bytes
	sizeTxOverhead        = 10  // 4-byte version + 1-byte input count + 1-byte output count + 4-byte locktime
)

// EstimateSize returns a conservative serialized size in bytes for a
// transaction with the given P2PKH input count and the given outputs.
func EstimateSize(numInputs int, outputs []Output) int {
	size := sizeTxOverhead + numInputs*sizeP2PKHInput
	for _, o := range outputs {
		size += sizePerOutputOverhead + len(o.Script)
		// If script length crosses the VarInt 1-byte boundary (≥253), add 2 bytes.
		if len(o.Script) >= 253 {
			size += 2
		}
	}
	return size
}

// ComputeFee returns the fee in satoshis for a given size and rate (sat/kB).
// Rounds up so we never underpay.
func ComputeFee(sizeBytes int, satPerKB uint64) uint64 {
	// fee = ceil(sizeBytes * satPerKB / 1000)
	prod := uint64(sizeBytes) * satPerKB
	return (prod + 999) / 1000
}
