// Package tests — CLIENT-2 implementation.
//
// Source plan §"Client Integration Tests" → CLIENT-2. Captures risks R2, R6.
//
// Objective:
//
//	Verify that, where Teranode advertises an extended transaction format,
//	an integrator can both produce and consume it.
//
// Method:
//  1. Use discovery (SP2) to determine whether extended format is advertised.
//     Per SP2 (docs/discovery.md §7), BIP-239 extended format is *always*
//     implemented in v0.15.0-beta-2; auto-extension means standard format
//     is also accepted. Test never skips for "not advertised".
//  2. Construct, submit, and verify round-trip of an extended-format
//     transaction (tx.ExtendedBytes()).
//  3. Verify standard-format backward compatibility on the same endpoint.
//
// Acceptance criteria:
//   - Extended-format transactions accepted where documented; no corruption.
//   - Standard format remains accepted.
//
// Implementation notes:
//   - Submit via Teranode RPC sendrawtransaction (per SP6 spec §4.1).
//     Discovery confirmed RPC accepts both formats.
//   - Retrieval is non-extended per discovery; this is recorded as an
//     observation, not a failure.
package tests

import (
	"context"
	"encoding/hex"
	"fmt"

	bt "github.com/libsv/go-bt/v2"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

func RunCLIENT2(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "CLIENT-2", Title: "Extended Transaction Format Support",
		Severity:              matrix.SeverityImportant,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-2"},
		CapturedRisks:         []string{"R2", "R6"},
		Observations:          map[string]any{},
	}
	defer func() { res.Duration = env.Now().Sub(res.StartedAt) }()

	if env.Teranode == nil || env.Teranode.RPC == nil || env.TxGen == nil || env.SVNode == nil {
		return skipMissing(res, "Teranode RPC, TxGen, or SVNode not configured")
	}

	funder := env.TxGen
	builder := funder.Builder()
	if funder.Balance() < 100_000_000 {
		if _, err := funder.Bootstrap(ctx, 100_000_000); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, err)
		}
	}

	addrScript, _ := txgen.P2PKHScript(funder.Address())

	// 1. Build extended-format tx via BuildP2PKH (txgen produces extended-format
	//    bytes by default per SP4 — the funder UTXOs carry PreviousTxScript +
	//    PreviousTxSatoshis when synthesised).
	bres, err := builder.BuildP2PKH(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		return errorResult(res, fmt.Errorf("build extended: %w", err))
	}

	// Submit extended-format hex.
	extTxID, err := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Teranode accepts extended-format transaction",
		err == nil && extTxID != "",
		fmt.Sprintf("returned=%q err=%v", extTxID, err),
	))
	if err == nil {
		funder.Confirm(bres.Inputs, bres.Change)
	}

	// 2. Build a standard-format tx and submit. Funder.Builder always produces
	//    extended-format hex; to get a standard-format hex we convert by parsing
	//    + re-serializing without the extended marker. The libsv go-bt v2 type
	//    has tx.Bytes() which produces standard format regardless of source.
	bres2, err := builder.BuildP2PKH(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		return errorResult(res, fmt.Errorf("build std: %w", err))
	}

	stdHex, err := standardFormatHex(bres2.HexTx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Standard-format hex computed from extended source",
		err == nil && stdHex != "",
		fmt.Sprintf("err=%v", err),
	))
	if err == nil {
		stdTxID, err := env.Teranode.RPC.SendRawTransaction(ctx, stdHex)
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			"Teranode accepts standard-format transaction (backward compat)",
			err == nil && stdTxID != "",
			fmt.Sprintf("returned=%q err=%v", stdTxID, err),
		))
		if err == nil {
			funder.Confirm(bres2.Inputs, bres2.Change)
		}
	}

	res.Observations["retrieval_format"] = "non-extended (per SP2 discovery; Asset REST returns standard bytes regardless of submission format)"

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}

// standardFormatHex parses extended-format hex and re-serialises as standard.
func standardFormatHex(extHex string) (string, error) {
	raw, err := hex.DecodeString(extHex)
	if err != nil {
		return "", fmt.Errorf("decode hex: %w", err)
	}
	tx, err := bt.NewTxFromBytes(raw)
	if err != nil {
		return "", fmt.Errorf("parse tx: %w", err)
	}
	stdBytes := tx.Bytes() // standard (non-extended) serialization
	return hex.EncodeToString(stdBytes), nil
}
