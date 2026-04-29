package txgen

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
	"github.com/bsv-blockchain/node-validation/internal/txgen/testdata"
)

// scriptedRPC drives multiple Call() invocations with scripted responses.
type scriptedRPC struct {
	calls []struct {
		method string
		err    error
		resp   any
	}
	idx int
}

func (s *scriptedRPC) Call(_ context.Context, method string, _ []any, out any) error {
	if s.idx >= len(s.calls) {
		return errors.New("scriptedRPC exhausted")
	}
	step := s.calls[s.idx]
	s.idx++
	if step.method != "" && step.method != method {
		return errors.New("scriptedRPC method mismatch: want " + step.method + " got " + method)
	}
	if step.err != nil {
		return step.err
	}
	if step.resp == nil || out == nil {
		return nil
	}
	return jsonRoundTrip(step.resp, out)
}

func TestBootstrap_happyPath(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)

	expectedTxid := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	rpc := &scriptedRPC{calls: []struct {
		method string
		err    error
		resp   any
	}{
		{method: "sendtoaddress", resp: expectedTxid},
		{method: "getrawtransaction", resp: map[string]any{
			"hex": "01000000",
			"vout": []map[string]any{
				{
					"value": 1.5,
					"n":     0,
					"scriptPubKey": map[string]any{
						"hex":       "76a91400112233445566778899aabbccddeeff0011223388ac",
						"addresses": []string{f.Address()},
					},
				},
			},
		}},
	}}
	f.rpc = rpc

	utxo, err := f.Bootstrap(context.Background(), 150_000_000)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if utxo.Satoshis != 150_000_000 {
		t.Errorf("satoshis: %d", utxo.Satoshis)
	}
	if len(utxo.Script) == 0 {
		t.Error("script empty")
	}
	if got := f.Balance(); got != 150_000_000 {
		t.Errorf("balance after bootstrap: %d", got)
	}
}

func TestBootstrap_methodNotFound(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	rpc := &scriptedRPC{calls: []struct {
		method string
		err    error
		resp   any
	}{
		{method: "sendtoaddress", err: &jsonrpc.Error{Code: -32601, Message: "Method not found"}},
	}}
	f.rpc = rpc

	_, err := f.Bootstrap(context.Background(), 1_000)
	if !errors.Is(err, ErrNoWallet) {
		t.Errorf("want ErrNoWallet, got %v", err)
	}
	if !strings.Contains(err.Error(), "sendtoaddress") {
		t.Errorf("err message: %q", err.Error())
	}
}

func TestBootstrap_walletNotEnabled(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	rpc := &scriptedRPC{calls: []struct {
		method string
		err    error
		resp   any
	}{
		{method: "sendtoaddress", err: &jsonrpc.Error{Code: -28, Message: "Wallet not loaded"}},
	}}
	f.rpc = rpc

	_, err := f.Bootstrap(context.Background(), 1_000)
	if !errors.Is(err, ErrNoWallet) {
		t.Errorf("want ErrNoWallet, got %v", err)
	}
}

func TestBootstrap_noOutputToAddress(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	rpc := &scriptedRPC{calls: []struct {
		method string
		err    error
		resp   any
	}{
		{method: "sendtoaddress", resp: "0011223344556677889900aabbccddeeff00112233445566778899aabbccddeeff"},
		{method: "getrawtransaction", resp: map[string]any{
			"hex": "01000000",
			"vout": []map[string]any{
				{
					"value": 0.001,
					"n":     0,
					"scriptPubKey": map[string]any{
						"addresses": []string{"someone-else"},
					},
				},
			},
		}},
	}}
	f.rpc = rpc

	_, err := f.Bootstrap(context.Background(), 100_000)
	if err == nil || !strings.Contains(err.Error(), "no output paying to") {
		t.Errorf("want 'no output' error, got %v", err)
	}
}
