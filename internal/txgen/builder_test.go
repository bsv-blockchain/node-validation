package txgen

import (
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/txgen/testdata"
	bt "github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

func TestBuildP2PKH_hexParses(t *testing.T) {
	f := newFundedFunder(t, 100_000_000)
	// Use the funder's address as destination.
	dest, err := P2PKHScript(f.Address())
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.Builder().BuildP2PKH(BuildRequest{
		Outputs: []Output{{Script: dest, Satoshis: 50_000_000}},
		FeeRate: 500,
	})
	if err != nil {
		t.Fatalf("BuildP2PKH: %v", err)
	}
	if res.HexTx == "" {
		t.Fatal("hex tx empty")
	}
	parsed, err := bt.NewTxFromString(res.HexTx)
	if err != nil {
		t.Fatalf("parse hex: %v", err)
	}
	if len(parsed.Inputs) != 1 {
		t.Errorf("inputs: %d", len(parsed.Inputs))
	}
	if len(parsed.Outputs) < 1 {
		t.Errorf("outputs: %d", len(parsed.Outputs))
	}
	// Verify the txid in the result matches the parsed tx.
	pid := parsed.TxIDBytes()
	if hex.EncodeToString(pid) != hex.EncodeToString(res.TxID[:]) {
		t.Errorf("txid mismatch")
	}
}

func TestBuildP2PKH_changeOutputCreated(t *testing.T) {
	f := newFundedFunder(t, 100_000_000)
	dest, _ := P2PKHScript(f.Address())
	res, err := f.Builder().BuildP2PKH(BuildRequest{
		Outputs: []Output{{Script: dest, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Change == nil {
		t.Fatal("change should exist")
	}
	if res.Change.Satoshis < 99_990_000 {
		t.Errorf("change satoshis: %d", res.Change.Satoshis)
	}
}

func TestBuildOpReturn_dataIncluded(t *testing.T) {
	f := newFundedFunder(t, 100_000_000)
	res, err := f.Builder().BuildOpReturn(BuildRequest{
		Outputs: nil,
		FeeRate: 500,
	}, []byte("acceptance-test-payload"))
	if err != nil {
		t.Fatalf("BuildOpReturn: %v", err)
	}
	parsed, _ := bt.NewTxFromString(res.HexTx)
	// First output should be 0 satoshis OP_RETURN.
	if parsed.Outputs[0].Satoshis != 0 {
		t.Errorf("opreturn satoshis: %d", parsed.Outputs[0].Satoshis)
	}
}

func TestBuildP2PKH_insufficientFunds(t *testing.T) {
	f := newFundedFunder(t, 1_000)
	dest, _ := P2PKHScript(f.Address())
	_, err := f.Builder().BuildP2PKH(BuildRequest{
		Outputs: []Output{{Script: dest, Satoshis: 1_000_000}},
		FeeRate: 500,
	})
	if err == nil {
		t.Fatal("want ErrInsufficientFunds")
	}
}

// TestBuildP2PKH_realKeySigns verifies that the signer actually populates
// the unlocking script when the UTXO's locking script matches the funder's key.
func TestBuildP2PKH_realKeySigns(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	// Construct a UTXO whose locking script is exactly the funder's P2PKH script.
	addrScript, err := P2PKHScript(f.Address())
	if err != nil {
		t.Fatal(err)
	}
	f.AddUTXO(UTXO{
		TxID:     [32]byte{0xab},
		Vout:     0,
		Satoshis: 100_000,
		Script:   addrScript,
	})
	res, err := f.Builder().BuildP2PKH(BuildRequest{
		Outputs: []Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		t.Fatalf("BuildP2PKH: %v", err)
	}
	parsed, err := bt.NewTxFromString(res.HexTx)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Inputs[0].UnlockingScript == nil || len(*parsed.Inputs[0].UnlockingScript) == 0 {
		t.Error("unlocking script should be populated by signer")
	}
}

func TestBuildP2MS_outputCorrectShape(t *testing.T) {
	f := newFundedFunder(t, 100_000_000)
	pks := [][]byte{make([]byte, 33), make([]byte, 33), make([]byte, 33)}
	for i, pk := range pks {
		pk[0] = 0x02
		pk[32] = byte(i + 1)
	}
	res, err := f.Builder().BuildP2MS(BuildRequest{
		Outputs: nil,
		FeeRate: 500,
	}, 2, pks, 10_000)
	if err != nil {
		t.Fatalf("BuildP2MS: %v", err)
	}
	parsed, _ := bt.NewTxFromString(res.HexTx)
	if parsed.Outputs[0].Satoshis != 10_000 {
		t.Errorf("p2ms out satoshis: %d", parsed.Outputs[0].Satoshis)
	}
	// Locking script should end with OP_CHECKMULTISIG.
	ls := *parsed.Outputs[0].LockingScript
	if ls[len(ls)-1] != bscript.OpCHECKMULTISIG {
		t.Errorf("p2ms script tail: %x", ls[len(ls)-1])
	}
}

func TestBuildP2SH_outputCorrectShape(t *testing.T) {
	f := newFundedFunder(t, 100_000_000)
	redeem := []byte{0x51} // OP_1
	res, err := f.Builder().BuildP2SH(BuildRequest{
		Outputs: nil,
		FeeRate: 500,
	}, redeem, 5_000)
	if err != nil {
		t.Fatalf("BuildP2SH: %v", err)
	}
	parsed, _ := bt.NewTxFromString(res.HexTx)
	ls := *parsed.Outputs[0].LockingScript
	if ls[0] != bscript.OpHASH160 {
		t.Errorf("p2sh head: %x", ls[0])
	}
	if ls[len(ls)-1] != bscript.OpEQUAL {
		t.Errorf("p2sh tail: %x", ls[len(ls)-1])
	}
}

func TestBuildChain_depth5(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	addrScript, _ := P2PKHScript(f.Address())
	// Single big UTXO that the chain consumes incrementally.
	f.AddUTXO(UTXO{
		TxID:     [32]byte{0xcd},
		Vout:     0,
		Satoshis: 100_000_000,
		Script:   addrScript,
	})
	results, err := f.Builder().BuildChain(BuildRequest{
		Outputs: []Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	}, 5)
	if err != nil {
		t.Fatalf("BuildChain: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results: %d want 5", len(results))
	}
	// Each tx in the chain should spend the previous tx's change.
	for i := 1; i < len(results); i++ {
		prev := results[i-1]
		this := results[i]
		if len(this.Inputs) == 0 {
			t.Errorf("chain[%d]: no inputs", i)
			continue
		}
		// The first input should reference the previous tx's change vout.
		if this.Inputs[0].TxID != prev.TxID {
			t.Errorf("chain[%d]: first input txid %x not from prev %x", i, this.Inputs[0].TxID, prev.TxID)
		}
	}
}

func TestBuildChain_depth25(t *testing.T) {
	// FR-7: chains of depth >=25 must work.
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	addrScript, _ := P2PKHScript(f.Address())
	f.AddUTXO(UTXO{
		TxID:     [32]byte{0xef},
		Vout:     0,
		Satoshis: 1_000_000_000, // 10 BSV — comfortable headroom
		Script:   addrScript,
	})
	results, err := f.Builder().BuildChain(BuildRequest{
		Outputs: []Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	}, 25)
	if err != nil {
		t.Fatalf("BuildChain depth=25: %v", err)
	}
	if len(results) != 25 {
		t.Errorf("results: %d want 25", len(results))
	}
}

func TestBuildChain_depthZeroRejected(t *testing.T) {
	f := newFundedFunder(t, 1_000_000)
	addrScript, _ := P2PKHScript(f.Address())
	if _, err := f.Builder().BuildChain(BuildRequest{
		Outputs: []Output{{Script: addrScript, Satoshis: 1}},
		FeeRate: 500,
	}, 0); err == nil {
		t.Error("depth=0 should error")
	}
}
