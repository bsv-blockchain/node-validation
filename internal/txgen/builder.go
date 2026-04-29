package txgen

import (
	"context"
	"encoding/hex"
	"fmt"

	bt "github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
)

// MinChainDepthFR7 is the acceptance threshold for NEW-FR7: chain depth ≥25
// must be constructable without error.
const MinChainDepthFR7 = 25

// Builder constructs and signs transactions on behalf of a Funder.
type Builder struct {
	funder *Funder
}

// Builder returns a Builder backed by the funder.
func (f *Funder) Builder() *Builder { return &Builder{funder: f} }

// BuildP2PKH constructs a transaction whose outputs are exactly req.Outputs
// (the caller already specified scripts), funded by the funder's UTXOs.
// A change output paying back to the funder's address is appended when
// the residual exceeds the dust threshold.
func (b *Builder) BuildP2PKH(req BuildRequest) (BuildResult, error) {
	target := uint64(0)
	for _, o := range req.Outputs {
		target += o.Satoshis
	}

	inputs, fee, change, err := b.funder.SelectInputs(target, req.Outputs, req.FeeRate)
	if err != nil {
		return BuildResult{}, err
	}

	tx := bt.NewTx()
	for _, in := range inputs {
		if err := tx.From(hex.EncodeToString(in.TxID[:]), in.Vout, hex.EncodeToString(in.Script), in.Satoshis); err != nil {
			return BuildResult{}, fmt.Errorf("tx.From: %w", err)
		}
	}
	for _, o := range req.Outputs {
		out := &bt.Output{Satoshis: o.Satoshis, LockingScript: btScript(o.Script)}
		tx.AddOutput(out)
	}

	var changeUTXO *UTXO
	if change > 0 {
		changeScript, err := P2PKHScript(b.funder.Address())
		if err != nil {
			return BuildResult{}, fmt.Errorf("change script: %w", err)
		}
		out := &bt.Output{Satoshis: change, LockingScript: btScript(changeScript)}
		tx.AddOutput(out)
		changeUTXO = &UTXO{
			Vout:     uint32(len(tx.Outputs) - 1),
			Satoshis: change,
			Script:   changeScript,
		}
	}

	// Sign every input using Getter (implements UnlockerGetter).
	ug := &unlocker.Getter{PrivateKey: b.funder.PrivateKey()}
	if err := tx.FillAllInputs(context.Background(), ug); err != nil {
		return BuildResult{}, fmt.Errorf("sign: %w", err)
	}

	txid := tx.TxIDBytes()
	var txidArr [32]byte
	copy(txidArr[:], txid)
	if changeUTXO != nil {
		changeUTXO.TxID = txidArr
	}

	_ = fee // already accounted in selection
	return BuildResult{
		TxID:   txidArr,
		HexTx:  tx.String(),
		Inputs: inputs,
		Change: changeUTXO,
	}, nil
}

// BuildOpReturn constructs a tx whose first output is OP_RETURN data;
// other outputs (if any) come from req.Outputs.
func (b *Builder) BuildOpReturn(req BuildRequest, dataPayload []byte) (BuildResult, error) {
	opReturnScript, err := OpReturnScript(dataPayload)
	if err != nil {
		return BuildResult{}, err
	}
	// OP_RETURN outputs carry 0 satoshis.
	outputs := append([]Output{{Script: opReturnScript, Satoshis: 0, Description: "op_return"}}, req.Outputs...)
	req2 := req
	req2.Outputs = outputs
	return b.BuildP2PKH(req2)
}

// BuildP2MS constructs a transaction whose first output is a bare
// m-of-n multisig output (paying TO the multisig script). Inputs
// remain P2PKH from the funder.
func (b *Builder) BuildP2MS(req BuildRequest, m int, pubkeys [][]byte, satoshisToMultisig uint64) (BuildResult, error) {
	msScript, err := MultisigScript(m, pubkeys)
	if err != nil {
		return BuildResult{}, err
	}
	outputs := append([]Output{{Script: msScript, Satoshis: satoshisToMultisig, Description: "p2ms"}}, req.Outputs...)
	req2 := req
	req2.Outputs = outputs
	return b.BuildP2PKH(req2)
}

// BuildP2SH constructs a transaction whose first output is a P2SH
// output committing to redeemScript. The redeemScript itself is not
// included in the transaction (it's revealed only at spend time).
func (b *Builder) BuildP2SH(req BuildRequest, redeemScript []byte, satoshisToP2SH uint64) (BuildResult, error) {
	p2sh, err := P2SHScript(redeemScript)
	if err != nil {
		return BuildResult{}, err
	}
	outputs := append([]Output{{Script: p2sh, Satoshis: satoshisToP2SH, Description: "p2sh"}}, req.Outputs...)
	req2 := req
	req2.Outputs = outputs
	return b.BuildP2PKH(req2)
}

// BuildChain constructs a chain of `depth` dependent P2PKH transactions:
// tx[0] spends a starting UTXO; tx[i+1] spends tx[i]'s change output.
// Each tx pays req.Outputs[0].Satoshis to req.Outputs[0].Script (typically
// a small dust-above payment to keep the chain alive).
func (b *Builder) BuildChain(req BuildRequest, depth int) ([]BuildResult, error) {
	if depth < 1 {
		return nil, fmt.Errorf("depth must be >=1, got %d", depth)
	}
	if len(req.Outputs) != 1 {
		return nil, fmt.Errorf("BuildChain requires exactly one output spec, got %d", len(req.Outputs))
	}
	out := make([]BuildResult, 0, depth)
	for i := 0; i < depth; i++ {
		res, err := b.BuildP2PKH(req)
		if err != nil {
			return nil, fmt.Errorf("chain depth %d: %w", i+1, err)
		}
		// Mark inputs spent and add change so the next iteration can spend it.
		b.funder.Confirm(res.Inputs, res.Change)
		out = append(out, res)
		if res.Change == nil {
			return nil, fmt.Errorf("chain truncated at depth %d: no change output to continue", i+1)
		}
	}
	return out, nil
}

// btScript wraps a raw script as a *bscript.Script.
func btScript(b []byte) *bscript.Script {
	s := bscript.Script(b)
	return &s
}
