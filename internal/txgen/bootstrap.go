package txgen

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
)

// Bootstrap calls SV Node's sendtoaddress RPC to obtain a fresh UTXO
// of the given size paying to the funder's address, then registers
// the new UTXO with the funder.
//
// Returns ErrNoWallet (wrapped) when SV Node lacks wallet support so
// SP5+ tests can skip cleanly.
func (f *Funder) Bootstrap(ctx context.Context, satoshis uint64) (UTXO, error) {
	if f.rpc == nil {
		return UTXO{}, errors.New("txgen: Bootstrap requires SV Node RPC; none configured")
	}
	bsv := float64(satoshis) / 1e8
	var txid string
	if err := f.rpc.Call(ctx, "sendtoaddress", []any{f.address, bsv}, &txid); err != nil {
		if isMethodNotFound(err) {
			return UTXO{}, fmt.Errorf("%w: sendtoaddress not available", ErrNoWallet)
		}
		return UTXO{}, fmt.Errorf("sendtoaddress: %w", err)
	}

	// Decode the transaction to find the output paying to f.address.
	var raw struct {
		Hex  string `json:"hex"`
		Vout []struct {
			Value        float64 `json:"value"`
			N            uint32  `json:"n"`
			ScriptPubKey struct {
				Hex       string   `json:"hex"`
				Addresses []string `json:"addresses"`
			} `json:"scriptPubKey"`
		} `json:"vout"`
	}
	if err := f.rpc.Call(ctx, "getrawtransaction", []any{txid, 1}, &raw); err != nil {
		return UTXO{}, fmt.Errorf("getrawtransaction: %w", err)
	}

	for _, v := range raw.Vout {
		for _, a := range v.ScriptPubKey.Addresses {
			if a == f.address {
				txidBytes, err := hex.DecodeString(txid)
				if err != nil || len(txidBytes) != 32 {
					return UTXO{}, fmt.Errorf("decode txid %q: %w", txid, err)
				}
				scriptBytes, err := hex.DecodeString(v.ScriptPubKey.Hex)
				if err != nil {
					return UTXO{}, fmt.Errorf("decode scriptPubKey: %w", err)
				}
				var arr [32]byte
				copy(arr[:], txidBytes)
				utxo := UTXO{
					TxID:     arr,
					Vout:     v.N,
					Satoshis: uint64(v.Value * 1e8),
					Script:   scriptBytes,
				}
				f.AddUTXO(utxo)
				return utxo, nil
			}
		}
	}
	return UTXO{}, fmt.Errorf("no output paying to %s in tx %s", f.address, txid)
}

func isMethodNotFound(err error) bool {
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == -32601 || // JSON-RPC method-not-found
			rpcErr.Code == -32 || // bsvjson misc
			strings.Contains(strings.ToLower(rpcErr.Message), "method not found") ||
			strings.Contains(strings.ToLower(rpcErr.Message), "wallet")
	}
	return false
}
