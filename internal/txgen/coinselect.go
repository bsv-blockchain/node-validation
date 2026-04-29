package txgen

import "fmt"

// SelectInputs picks UTXOs to cover (target + estimated fee) under the
// greedy first-fit policy. Outputs is the intended output set so the
// fee can be estimated; SelectInputs assumes the caller will add a
// change output if necessary.
//
// Returns the chosen inputs, the fee paid, and the change satoshis
// (zero when change would be dust).
func (f *Funder) SelectInputs(target uint64, outputs []Output, satPerKB uint64) (inputs []UTXO, fee uint64, change uint64, err error) {
	utxos := f.snapshotUTXOs()
	if len(utxos) == 0 {
		return nil, 0, 0, fmt.Errorf("%w: no UTXOs", ErrInsufficientFunds)
	}

	var (
		acc    uint64
		picked []UTXO
	)
	for i := range utxos {
		picked = append(picked, utxos[i])
		acc += utxos[i].Satoshis
		// Estimate size assuming we may emit a change output.
		size := EstimateSize(len(picked), append(outputs, Output{Script: make([]byte, 25)}))
		fee := ComputeFee(size, satPerKB)
		if acc >= target+fee {
			change := acc - target - fee
			if change < dustThresholdSats {
				// Re-estimate without change.
				sizeNoChange := EstimateSize(len(picked), outputs)
				feeNoChange := ComputeFee(sizeNoChange, satPerKB)
				if acc >= target+feeNoChange {
					return picked, feeNoChange, 0, nil
				}
				continue
			}
			return picked, fee, change, nil
		}
	}
	return nil, 0, 0, fmt.Errorf("%w: have %d, need ~%d", ErrInsufficientFunds, acc, target)
}

// MarkSpent removes the given UTXOs from the funder's set. Used by
// Confirm after broadcast.
func (f *Funder) MarkSpent(spent []UTXO) {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	out := f.state.utxos[:0]
	skip := make(map[[32]byte]map[uint32]bool, len(spent))
	for _, s := range spent {
		if skip[s.TxID] == nil {
			skip[s.TxID] = map[uint32]bool{}
		}
		skip[s.TxID][s.Vout] = true
	}
	for _, u := range f.state.utxos {
		if skip[u.TxID][u.Vout] {
			continue
		}
		out = append(out, u)
	}
	f.state.utxos = out
}

// Confirm marks inputs spent and (optionally) adds the change UTXO.
// Tests call this after a successful broadcast.
func (f *Funder) Confirm(spent []UTXO, change *UTXO) {
	f.MarkSpent(spent)
	if change != nil {
		f.AddUTXO(*change)
	}
}
