// internal/compare/chainstate.go
package compare

import (
	"errors"
	"strings"

	"github.com/bsv-blockchain/node-validation/internal/jsonrpc"
)

type RejectionCategory string

const (
	CategoryAccepted      RejectionCategory = "ACCEPTED"
	CategoryUTXOSpent     RejectionCategory = "UTXO_SPENT"
	CategoryUTXOMissing   RejectionCategory = "UTXO_MISSING"
	CategoryScriptFailure RejectionCategory = "SCRIPT_FAILURE"
	CategoryFeeTooLow     RejectionCategory = "FEE_TOO_LOW"
	CategoryDustOutput    RejectionCategory = "DUST_OUTPUT"
	CategoryNonStandard   RejectionCategory = "NON_STANDARD"
	CategoryConflicting   RejectionCategory = "CONFLICTING"
	CategoryMalformed     RejectionCategory = "MALFORMED"
	CategoryRPCError      RejectionCategory = "RPC_ERROR"
	CategoryUnknown       RejectionCategory = "UNKNOWN"
)

// Teranode error codes per docs/discovery.md (errors/error.pb.go).
const (
	teranodeErrUTXOSpent     = 70
	teranodeErrTxConflicting = 36
	teranodeErrTxInvalidDS   = 32
)

// CategorizeTeranode maps a Teranode RPC error to a canonical category.
// Returns CategoryAccepted when err is nil.
func CategorizeTeranode(err error) RejectionCategory {
	if err == nil {
		return CategoryAccepted
	}
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		switch rpcErr.Code {
		case teranodeErrUTXOSpent:
			return CategoryUTXOSpent
		case teranodeErrTxConflicting, teranodeErrTxInvalidDS:
			return CategoryConflicting
		}
		// Bitcoin-style codes (-26 = rejected, -25 = missing inputs, -22 = decode failure).
		switch rpcErr.Code {
		case -22:
			return CategoryMalformed
		case -25:
			return CategoryUTXOMissing
		case -26:
			return categorizeRejectionMessage(rpcErr.Message)
		}
	}
	return CategoryRPCError
}

// CategorizeSVNode maps an SV Node RPC error to the same canonical category.
// SV Node uses the same JSON-RPC error code surface (codes -22, -25, -26 etc.)
// but lacks the Teranode-specific 32/36/70 codes.
func CategorizeSVNode(err error) RejectionCategory {
	if err == nil {
		return CategoryAccepted
	}
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		switch rpcErr.Code {
		case -22:
			return CategoryMalformed
		case -25:
			return CategoryUTXOMissing
		case -26:
			return categorizeRejectionMessage(rpcErr.Message)
		case -27:
			return CategoryConflicting // tx already in chain / already in mempool
		}
	}
	return CategoryRPCError
}

// categorizeRejectionMessage parses the substring of a generic -26 reject
// message ("dust", "min relay fee not met", "missing-inputs", "non-mandatory-script-verify-flag").
func categorizeRejectionMessage(msg string) RejectionCategory {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "double-spend") || strings.Contains(m, "conflict"):
		return CategoryConflicting
	case strings.Contains(m, "dust"):
		return CategoryDustOutput
	case strings.Contains(m, "fee") && (strings.Contains(m, "low") || strings.Contains(m, "min relay") || strings.Contains(m, "min mining")):
		return CategoryFeeTooLow
	case strings.Contains(m, "missing-inputs") || strings.Contains(m, "utxo not found") || strings.Contains(m, "bad-txns-inputs-missingorspent"):
		return CategoryUTXOMissing
	case strings.Contains(m, "already spent") || strings.Contains(m, "utxo already spent"):
		return CategoryUTXOSpent
	// Check non-standard/non-mandatory before the broader script/verify-flag check
	// because "non-mandatory-script-verify-flag" would otherwise match both.
	case strings.Contains(m, "non-standard") || strings.Contains(m, "non-mandatory"):
		return CategoryNonStandard
	case strings.Contains(m, "script") || strings.Contains(m, "verify-flag") || strings.Contains(m, "evalscript"):
		return CategoryScriptFailure
	case strings.Contains(m, "tx-size") || strings.Contains(m, "exceeds") || strings.Contains(m, "decode"):
		return CategoryMalformed
	}
	return CategoryUnknown
}

// CompareCategories returns matched=true iff both backends produced the
// same canonical category.
func CompareCategories(teranodeErr, svnodeErr error) (matched bool, teranodeCat, svnodeCat RejectionCategory) {
	teranodeCat = CategorizeTeranode(teranodeErr)
	svnodeCat = CategorizeSVNode(svnodeErr)
	matched = teranodeCat == svnodeCat
	return
}
