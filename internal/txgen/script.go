package txgen

import (
	"errors"
	"fmt"

	"github.com/libsv/go-bk/crypto"
	"github.com/libsv/go-bt/v2/bscript"
)

// opReturnMaxSize caps OP_RETURN payloads so tests stay bounded.
const opReturnMaxSize = 100 * 1024

// P2PKHScript returns the standard pay-to-pubkey-hash locking script
// for the given Bitcoin address.
func P2PKHScript(addr string) ([]byte, error) {
	a, err := bscript.NewAddressFromString(addr)
	if err != nil {
		return nil, fmt.Errorf("address %q: %w", addr, err)
	}
	s, err := bscript.NewP2PKHFromAddress(a.AddressString)
	if err != nil {
		return nil, fmt.Errorf("p2pkh script for %q: %w", addr, err)
	}
	return *s, nil
}

// OpReturnScript returns OP_FALSE OP_RETURN <push data>.
func OpReturnScript(data []byte) ([]byte, error) {
	if len(data) > opReturnMaxSize {
		return nil, fmt.Errorf("OP_RETURN payload %d > max %d", len(data), opReturnMaxSize)
	}
	s := &bscript.Script{}
	if err := s.AppendOpcodes(bscript.Op0, bscript.OpRETURN); err != nil {
		return nil, err
	}
	if err := s.AppendPushData(data); err != nil {
		return nil, err
	}
	return *s, nil
}

// MultisigScript returns the bare m-of-n multisig locking script:
// OP_M <pk1> ... <pkN> OP_N OP_CHECKMULTISIG.
func MultisigScript(m int, pubkeys [][]byte) ([]byte, error) {
	n := len(pubkeys)
	if m < 1 || m > n || n > 16 {
		return nil, fmt.Errorf("invalid multisig m-of-n: m=%d n=%d", m, n)
	}
	s := &bscript.Script{}
	mOp, err := opNumeric(m)
	if err != nil {
		return nil, err
	}
	if err := s.AppendOpcodes(mOp); err != nil {
		return nil, err
	}
	for _, pk := range pubkeys {
		if err := s.AppendPushData(pk); err != nil {
			return nil, err
		}
	}
	nOp, err := opNumeric(n)
	if err != nil {
		return nil, err
	}
	if err := s.AppendOpcodes(nOp, bscript.OpCHECKMULTISIG); err != nil {
		return nil, err
	}
	return *s, nil
}

// P2SHScript returns OP_HASH160 <hash160(redeem)> OP_EQUAL.
func P2SHScript(redeemScript []byte) ([]byte, error) {
	if len(redeemScript) == 0 {
		return nil, errors.New("redeem script empty")
	}
	s := &bscript.Script{}
	if err := s.AppendOpcodes(bscript.OpHASH160); err != nil {
		return nil, err
	}
	hash := crypto.Hash160(redeemScript)
	if err := s.AppendPushData(hash); err != nil {
		return nil, err
	}
	if err := s.AppendOpcodes(bscript.OpEQUAL); err != nil {
		return nil, err
	}
	return *s, nil
}

// opNumeric maps 1..16 to the corresponding OP_1..OP_16 byte.
func opNumeric(n int) (byte, error) {
	if n < 1 || n > 16 {
		return 0, fmt.Errorf("opNumeric: %d out of range", n)
	}
	return bscript.Op1 - 1 + byte(n), nil
}
