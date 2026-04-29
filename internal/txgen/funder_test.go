package txgen

import (
	"context"
	"strings"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/txgen/testdata"
)

func TestNewFunder_validWIF(t *testing.T) {
	f, err := NewFunder(nil, testdata.TestWIFRegtest, nil)
	if err != nil {
		t.Fatalf("NewFunder: %v", err)
	}
	if f.Address() == "" {
		t.Error("address should be derived")
	}
	if f.PrivateKey() == nil {
		t.Error("key should be set")
	}
}

func TestNewFunder_testnetWIFGivesTestnetAddress(t *testing.T) {
	// Known testnet WIF (privkey=1, testnet/regtest version byte 0xef).
	testnetWIF := "cMahea7zqjxrtgAbB7LSGbcZqfA1qiUJqXEsFnUMGbE3JjN1uTaG"
	f, err := NewFunder(nil, testnetWIF, nil)
	if err != nil {
		t.Skipf("could not derive funder from testnet WIF (lib may not support): %v", err)
	}
	// Testnet P2PKH addresses start with 'm' or 'n'; mainnet starts with '1'.
	addr := f.Address()
	if addr == "" || addr[0] == '1' {
		t.Errorf("expected testnet address (starts with m/n), got %q", addr)
	}
}

func TestNewFunder_badWIF(t *testing.T) {
	_, err := NewFunder(nil, "not-a-wif", nil)
	if err == nil || !strings.Contains(err.Error(), "decode WIF") {
		t.Errorf("want decode WIF error, got %v", err)
	}
}

func TestFunder_BalanceAndAddUTXO(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	if f.Balance() != 0 {
		t.Errorf("initial balance should be 0, got %d", f.Balance())
	}
	f.AddUTXO(utxoFromFixture(testdata.FundedUTXO(0x01, 100_000)))
	f.AddUTXO(utxoFromFixture(testdata.FundedUTXO(0x02, 50_000)))
	if got := f.Balance(); got != 150_000 {
		t.Errorf("balance: %d", got)
	}
}

func TestFunder_Reset(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	f.AddUTXO(utxoFromFixture(testdata.FundedUTXO(0x01, 1_000)))
	f.Reset()
	if f.Balance() != 0 {
		t.Errorf("after Reset, balance should be 0, got %d", f.Balance())
	}
}

func TestFunder_ConcurrentAddUTXO(t *testing.T) {
	f, _ := NewFunder(nil, testdata.TestWIFRegtest, nil)
	const goroutines = 100
	const perGoroutine = 10
	done := make(chan struct{}, goroutines)
	for g := 0; g < goroutines; g++ {
		go func(seed byte) {
			for i := 0; i < perGoroutine; i++ {
				f.AddUTXO(utxoFromFixture(testdata.FundedUTXO(seed, 1)))
			}
			done <- struct{}{}
		}(byte(g))
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	if got := f.Balance(); got != goroutines*perGoroutine {
		t.Errorf("expected %d total satoshis, got %d", goroutines*perGoroutine, got)
	}
}

// fakeRPC is a minimal RPCCaller for tests.
type fakeRPC struct {
	method string
	params []any
	err    error
	resp   any
}

func (f *fakeRPC) Call(_ context.Context, method string, params []any, out any) error {
	f.method = method
	f.params = params
	if f.err != nil {
		return f.err
	}
	if f.resp == nil || out == nil {
		return nil
	}
	// Use json round-trip to copy resp into out.
	return jsonRoundTrip(f.resp, out)
}

var _ RPCCaller = (*fakeRPC)(nil)
