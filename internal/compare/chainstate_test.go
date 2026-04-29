package compare

import (
	"errors"
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
)

func TestCategorizeTeranode_table(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want RejectionCategory
	}{
		{"nil → ACCEPTED", nil, CategoryAccepted},
		{"utxo spent (70)", &jsonrpc.Error{Code: 70, Message: "utxo already spent"}, CategoryUTXOSpent},
		{"tx conflicting (36)", &jsonrpc.Error{Code: 36, Message: "tx is conflicting"}, CategoryConflicting},
		{"invalid DS (32)", &jsonrpc.Error{Code: 32, Message: "invalid double-spend"}, CategoryConflicting},
		{"-22 malformed", &jsonrpc.Error{Code: -22, Message: "TX decode failed"}, CategoryMalformed},
		{"-25 missing inputs", &jsonrpc.Error{Code: -25, Message: ""}, CategoryUTXOMissing},
		{"-26 dust", &jsonrpc.Error{Code: -26, Message: "dust output"}, CategoryDustOutput},
		{"-26 fee", &jsonrpc.Error{Code: -26, Message: "min mining fee not met"}, CategoryFeeTooLow},
		{"-26 script", &jsonrpc.Error{Code: -26, Message: "mandatory-script-verify-flag-failed"}, CategoryScriptFailure},
		{"-26 already spent", &jsonrpc.Error{Code: -26, Message: "utxo already spent"}, CategoryUTXOSpent},
		{"-26 malformed tx-size", &jsonrpc.Error{Code: -26, Message: "tx-size too large"}, CategoryMalformed},
		{"-26 unknown msg", &jsonrpc.Error{Code: -26, Message: "some weird message"}, CategoryUnknown},
		{"unknown code", &jsonrpc.Error{Code: -99, Message: "unknown"}, CategoryRPCError},
		{"non-RPC error", errors.New("network failure"), CategoryRPCError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CategorizeTeranode(c.err); got != c.want {
				t.Errorf("got %s want %s", got, c.want)
			}
		})
	}
}

func TestCategorizeSVNode_table(t *testing.T) {
	cases := []struct {
		err  error
		want RejectionCategory
	}{
		{nil, CategoryAccepted},
		{&jsonrpc.Error{Code: -25, Message: "Missing inputs"}, CategoryUTXOMissing},
		{&jsonrpc.Error{Code: -27, Message: "transaction already in block chain"}, CategoryConflicting},
		{&jsonrpc.Error{Code: -26, Message: "256: txn-mempool-conflict"}, CategoryConflicting},
		{&jsonrpc.Error{Code: -22, Message: "TX decode failed"}, CategoryMalformed},
		{&jsonrpc.Error{Code: -26, Message: "non-mandatory-script-verify-flag"}, CategoryNonStandard},
		{&jsonrpc.Error{Code: -26, Message: "evalscript failed"}, CategoryScriptFailure},
		{&jsonrpc.Error{Code: -26, Message: "min relay fee not met"}, CategoryFeeTooLow},
		{&jsonrpc.Error{Code: -26, Message: "bad-txns-inputs-missingorspent"}, CategoryUTXOMissing},
		{&jsonrpc.Error{Code: -26, Message: "exceeds max size"}, CategoryMalformed},
		{errors.New("timeout"), CategoryRPCError},
	}
	for _, c := range cases {
		if got := CategorizeSVNode(c.err); got != c.want {
			t.Errorf("err=%v: got %s want %s", c.err, got, c.want)
		}
	}
}

func TestCompareCategories(t *testing.T) {
	matched, _, _ := CompareCategories(nil, nil)
	if !matched {
		t.Error("nil/nil should match")
	}
	// Teranode error code 70 → UTXO_SPENT (confirmed spend).
	// SVNode -26 + "double-spend detected" → CONFLICTING (mempool conflict).
	// These are distinct categories: UTXO_SPENT means an input was already
	// consumed in a confirmed block; CONFLICTING means two unconfirmed txs
	// racing for the same UTXO. They must NOT match.
	matched, tc, sc := CompareCategories(
		&jsonrpc.Error{Code: 70, Message: "utxo already spent"},
		&jsonrpc.Error{Code: -26, Message: "double-spend detected"},
	)
	if tc != CategoryUTXOSpent || sc != CategoryConflicting {
		t.Errorf("expected (UTXO_SPENT, CONFLICTING), got (%s, %s)", tc, sc)
	}
	if matched {
		t.Errorf("should NOT match: %s vs %s", tc, sc)
	}
}
