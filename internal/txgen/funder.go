package txgen

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/wif"
	"github.com/libsv/go-bt/v2/bscript"
)

// Funder holds wallet state for the txgen package.
type Funder struct {
	rpc     RPCCaller
	wif     string
	key     *bec.PrivateKey
	address string
	state   *keyAndUTXOs
	logger  *slog.Logger
}

// RPCCaller is the minimal interface Funder needs for Bootstrap.
// In production this is *svnode.RPCClient; tests inject a fake.
type RPCCaller interface {
	Call(ctx context.Context, method string, params []any, out any) error
}

// NewFunder constructs a Funder. wifStr is the funding wallet's WIF;
// rpc may be nil if the test will only inject UTXOs directly.
func NewFunder(rpc RPCCaller, wifStr string, logger *slog.Logger) (*Funder, error) {
	w, err := wif.DecodeWIF(wifStr)
	if err != nil {
		return nil, fmt.Errorf("txgen: decode WIF: %w", err)
	}
	addr, err := bscript.NewAddressFromPublicKey(w.PrivKey.PubKey(), true)
	if err != nil {
		return nil, fmt.Errorf("txgen: derive address: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Funder{
		rpc:     rpc,
		wif:     wifStr,
		key:     w.PrivKey,
		address: addr.AddressString,
		state:   &keyAndUTXOs{key: w.PrivKey},
		logger:  logger,
	}, nil
}

// Address returns the P2PKH address the WIF controls.
func (f *Funder) Address() string { return f.address }

// PrivateKey returns the underlying secp256k1 key. Used by Builder.
func (f *Funder) PrivateKey() *bec.PrivateKey { return f.key }

// Balance returns the sum of all UTXOs the funder currently knows about.
func (f *Funder) Balance() uint64 {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	var total uint64
	for _, u := range f.state.utxos {
		total += u.Satoshis
	}
	return total
}

// AddUTXO injects a UTXO into the funder. Used by tests and by
// Confirm/Bootstrap.
func (f *Funder) AddUTXO(u UTXO) {
	f.state.mu.Lock()
	f.state.utxos = append(f.state.utxos, u)
	f.state.mu.Unlock()
}

// Reset clears all UTXOs. For test cleanup only.
func (f *Funder) Reset() {
	f.state.mu.Lock()
	f.state.utxos = nil
	f.state.mu.Unlock()
}

// snapshotUTXOs returns a copy of the UTXO list under the lock. Used by
// SelectInputs and tests.
func (f *Funder) snapshotUTXOs() []UTXO {
	f.state.mu.Lock()
	defer f.state.mu.Unlock()
	out := make([]UTXO, len(f.state.utxos))
	copy(out, f.state.utxos)
	return out
}
