package txgen

import (
	"errors"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/txgen/testdata"
)

func newFundedFunder(t *testing.T, amounts ...uint64) *Funder {
	t.Helper()
	f, err := NewFunder(nil, testdata.TestWIFRegtest, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range amounts {
		f.AddUTXO(utxoFromFixture(testdata.FundedUTXO(byte(i+1), a)))
	}
	return f
}

func TestSelectInputs_singleSufficient(t *testing.T) {
	f := newFundedFunder(t, 1_000_000)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 50_000}}
	inputs, fee, change, err := f.SelectInputs(50_000, outs, 1000)
	if err != nil {
		t.Fatalf("SelectInputs: %v", err)
	}
	if len(inputs) != 1 {
		t.Errorf("inputs: %d want 1", len(inputs))
	}
	if fee == 0 {
		t.Error("fee should be > 0")
	}
	if change != 1_000_000-50_000-fee {
		t.Errorf("change: %d", change)
	}
}

func TestSelectInputs_multipleNeeded(t *testing.T) {
	f := newFundedFunder(t, 30_000, 30_000, 30_000)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 70_000}}
	inputs, _, _, err := f.SelectInputs(70_000, outs, 500)
	if err != nil {
		t.Fatalf("SelectInputs: %v", err)
	}
	if len(inputs) < 2 {
		t.Errorf("want >=2 inputs, got %d", len(inputs))
	}
}

func TestSelectInputs_insufficient(t *testing.T) {
	f := newFundedFunder(t, 1_000)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 5_000}}
	_, _, _, err := f.SelectInputs(5_000, outs, 1000)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("want ErrInsufficientFunds, got %v", err)
	}
}

func TestSelectInputs_emptyFunder(t *testing.T) {
	f := newFundedFunder(t)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 1}}
	_, _, _, err := f.SelectInputs(1, outs, 1000)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("want ErrInsufficientFunds, got %v", err)
	}
}

func TestSelectInputs_dustChangeAbsorbed(t *testing.T) {
	// Pick a UTXO size such that change would be < 546.
	// Target 999_500 from a single 1_000_000 UTXO at low fee -> change ~= 500 - fee, dust.
	f := newFundedFunder(t, 1_000_000)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 999_500}}
	_, _, change, err := f.SelectInputs(999_500, outs, 100)
	if err != nil {
		t.Fatalf("SelectInputs: %v", err)
	}
	if change != 0 {
		t.Errorf("dust change should be absorbed, got %d", change)
	}
}

func TestSelectInputs_dustAbsorbed_feeIsActualResidual(t *testing.T) {
	f := newFundedFunder(t, 1_000_000)
	outs := []Output{{Script: make([]byte, 25), Satoshis: 999_500}}
	inputs, fee, change, err := f.SelectInputs(999_500, outs, 100)
	if err != nil {
		t.Fatal(err)
	}
	if change != 0 {
		t.Errorf("dust should be absorbed, got change=%d", change)
	}
	inputSum := uint64(0)
	for _, in := range inputs {
		inputSum += in.Satoshis
	}
	if fee != inputSum-999_500 {
		t.Errorf("fee should equal inputSum-target=%d, got %d", inputSum-999_500, fee)
	}
}

func TestMarkSpent_removesUTXOs(t *testing.T) {
	f := newFundedFunder(t, 1_000, 2_000, 3_000)
	utxos := f.SnapshotUTXOs()
	f.MarkSpent(utxos[:2])
	if got := f.Balance(); got != 3_000 {
		t.Errorf("balance after spend: %d", got)
	}
}

func TestConfirm_addsChange(t *testing.T) {
	f := newFundedFunder(t, 10_000)
	utxos := f.SnapshotUTXOs()
	change := UTXO{TxID: [32]byte{0xff}, Vout: 1, Satoshis: 5_000, Script: utxos[0].Script}
	f.Confirm(utxos, &change)
	if got := f.Balance(); got != 5_000 {
		t.Errorf("balance after Confirm: %d", got)
	}
}
