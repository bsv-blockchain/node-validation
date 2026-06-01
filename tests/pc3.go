// Package tests — PC-3 implementation.
//
// Source plan §"Protocol Correctness Tests" → PC-3.
// Captures risk R2. Acceptance criteria from FR-2. Severity Critical.
//
// Objective:
//
//	Verify standard BSV transactions round-trip byte-identical through
//	Teranode, and Teranode-emitted blocks parse with a standard parser.
//
// Method:
//  1. Construct standard BSV transactions of three shapes (P2PKH,
//     P2MS multisig, OP_RETURN data carrier) using libsv/go-bt/v2.
//  2. Submit via Teranode RPC sendrawtransaction; record returned txid.
//  3. Fetch each tx back via Teranode REST /tx/{hash}; verify byte-exact
//     round-trip (matching txid).
//  4. Mine a block via svnode-1; wait for Teranode tip to advance.
//  5. Fetch the block; re-parse with libsv parser; verify all three test
//     transactions are in it.
//
// Acceptance criteria:
//   - All transactions round-trip with matching txid.
//   - All test blocks parse without error.
//
// Implementation notes:
//   - Scope is format-only; raw P2P packet capture is out of scope for SP5.
//   - The funder must have a UTXO ≥1.5 BSV; bootstrap if the balance is low.
//   - P2MS uses 2-of-3 with three deterministic dummy compressed pubkeys.
//   - Mining uses svnode-1's wallet via mineBlocks helper.
//   - Wait timeout: 60s for tip propagation (configurable via code).
package tests

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	bt "github.com/libsv/go-bt/v2"

	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/internal/txgen"
)

// builtTx records a submitted transaction for later block-inclusion checks.
type builtTx struct {
	shape    string
	expected [32]byte
	txid     string
}

func RunPC3(ctx context.Context, env *testrunner.Env) testrunner.Result {
	res := testrunner.Result{
		ID: "PC-3", Title: "Message Format and Wire Protocol Verification",
		Severity:              matrix.SeverityCritical,
		StartedAt:             env.Now(),
		SatisfiesRequirements: []string{"FR-2"},
		CapturedRisks:         []string{"R2"},
	}
	defer func() {
		res.Duration = env.Now().Sub(res.StartedAt)
	}()

	if env.Teranode == nil || env.Teranode.RPC == nil || env.Teranode.REST == nil ||
		env.SVNode == nil || env.SVNode.RPC == nil || env.TxGen == nil {
		return skipMissing(res, "Teranode RPC/REST, SVNode RPC, or TxGen not configured")
	}

	funder := env.TxGen
	builder := funder.Builder()

	// Bootstrap UTXO if needed.
	if funder.Balance() < 100_000_000 {
		if _, err := bootstrapConfirmed(ctx, env, 100_000_000); err != nil {
			return errorResult(res, fmt.Errorf("bootstrap: %w", err))
		}
		// Mine to confirm.
		if _, err := mineBlocks(ctx, env, 1); err != nil {
			return errorResult(res, fmt.Errorf("confirm bootstrap: %w", err))
		}
		time.Sleep(2 * time.Second) // brief settle for propagation
	}

	addrScript, err := txgen.P2PKHScript(funder.Address())
	if err != nil {
		return errorResult(res, fmt.Errorf("p2pkh script: %w", err))
	}

	// Three deterministic dummy compressed pubkeys for the P2MS shape.
	pubkeys := [][]byte{
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa1},
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa2},
		{0x02, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0xa3},
	}

	var txs []builtTx

	// Shape 1 — P2PKH.
	bres, err := builder.BuildP2PKH(txgen.BuildRequest{
		Outputs: []txgen.Output{{Script: addrScript, Satoshis: 1_000}},
		FeeRate: 500,
	})
	if err != nil {
		return errorResult(res, fmt.Errorf("build P2PKH: %w", err))
	}
	if err := submitAndConfirm(ctx, env, funder, bres, &txs, "P2PKH", &res); err != nil {
		return errorResult(res, err)
	}

	// Shape 2 — P2MS (output paying to a 2-of-3 multisig).
	bres2, err := builder.BuildP2MS(txgen.BuildRequest{Outputs: nil, FeeRate: 500}, 2, pubkeys, 5_000)
	if err != nil {
		return errorResult(res, fmt.Errorf("build P2MS: %w", err))
	}
	if err := submitAndConfirm(ctx, env, funder, bres2, &txs, "P2MS", &res); err != nil {
		return errorResult(res, err)
	}

	// Shape 3 — OP_RETURN.
	bres3, err := builder.BuildOpReturn(txgen.BuildRequest{Outputs: nil, FeeRate: 500}, []byte("PC-3 round-trip"))
	if err != nil {
		return errorResult(res, fmt.Errorf("build OP_RETURN: %w", err))
	}
	if err := submitAndConfirm(ctx, env, funder, bres3, &txs, "OP_RETURN", &res); err != nil {
		return errorResult(res, err)
	}

	// (4) Wait for the 3 submitted txs to propagate from Teranode into
	// svnode-1's mempool, THEN mine. Without this delay svnode-1 may
	// mine an empty block before the legacy P2P relay catches up.
	if err := waitForMempoolEntries(ctx, env.SVNode.RPC, []string{
		hex.EncodeToString(bres.TxID[:]),
		hex.EncodeToString(bres2.TxID[:]),
		hex.EncodeToString(bres3.TxID[:]),
	}, 30*time.Second); err != nil {
		res.AcceptanceChecks = append(res.AcceptanceChecks, testrunner.Check{
			Description: "All 3 test txs propagated to svnode-1 mempool",
			Required:    false,
			Pass:        false,
			Detail:      err.Error(),
		})
	} else {
		res.AcceptanceChecks = append(res.AcceptanceChecks, testrunner.Check{
			Description: "All 3 test txs propagated to svnode-1 mempool",
			Required:    false,
			Pass:        true,
			Detail:      "observed via getrawmempool",
		})
	}

	mined, err := mineBlocks(ctx, env, 1)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block mined via svnode-1",
		err == nil && len(mined) == 1,
		fmt.Sprintf("hashes=%v err=%v", mined, err),
	))
	if err != nil || len(mined) != 1 {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}
	blockHash := mined[0]

	// Wait for Teranode tip to advance.
	terr := waitForTeranodeTip(ctx, env.Teranode.RPC, blockHash, 60*time.Second)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Teranode tip reached mined block within 60s",
		terr == nil,
		fmt.Sprintf("tip=%s err=%v", blockHash, terr),
	))
	if terr != nil {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// (5) Fetch the block, parse, verify our test txs are present.
	blockBytes, err := env.Teranode.REST.GetBlockLegacyBytes(ctx, blockHash)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block bytes fetched via Teranode REST",
		err == nil && len(blockBytes) > 0,
		fmt.Sprintf("bytes=%d err=%v", len(blockBytes), err),
	))
	if err != nil {
		res.Status = deriveStatus(res.AcceptanceChecks)
		return res
	}

	// Parse with libsv. The block format may be standard Bitcoin (header + tx-count VarInt + txs)
	// or a Teranode-specific shape; if the standard parser fails, that's a finding.
	stdTxids, parseErr := parseStandardBlock(blockBytes)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		"Block parses with standard libsv parser",
		parseErr == nil && len(stdTxids) > 0,
		fmt.Sprintf("txs_in_block=%d err=%v", len(stdTxids), parseErr),
	))

	// Verify each of our txs is in the block.
	idSet := map[string]bool{}
	for _, id := range stdTxids {
		idSet[id] = true
	}
	for _, t := range txs {
		present := idSet[t.txid]
		shortID := t.txid
		if len(shortID) > 10 {
			shortID = shortID[:10]
		}
		res.AcceptanceChecks = append(res.AcceptanceChecks, required(
			fmt.Sprintf("Block contains %s test tx (%s…)", t.shape, shortID),
			present,
			fmt.Sprintf("present=%v", present),
		))
	}

	res.Status = deriveStatus(res.AcceptanceChecks)
	return res
}

// submitAndConfirm submits a built tx via Teranode RPC, fetches it back via REST,
// verifies the round-trip, and (on success) marks the funder's inputs spent and
// the change UTXO available. Appends acceptance checks to res.
func submitAndConfirm(
	ctx context.Context,
	env *testrunner.Env,
	funder *txgen.Funder,
	bres txgen.BuildResult,
	txs *[]builtTx,
	shape string,
	res *testrunner.Result,
) error {
	returnedTxid, err := env.Teranode.RPC.SendRawTransaction(ctx, bres.HexTx)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Teranode accepts %s tx via sendrawtransaction", shape),
		err == nil && returnedTxid != "",
		fmt.Sprintf("returned=%q err=%v", returnedTxid, err),
	))
	if err != nil {
		return nil // recorded as failed check, don't error the whole test
	}

	// Verify the returned txid matches the locally-computed one.
	expectedHex := hex.EncodeToString(bres.TxID[:])
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Returned %s txid matches locally-computed", shape),
		returnedTxid == expectedHex,
		fmt.Sprintf("returned=%s expected=%s", returnedTxid, expectedHex),
	))

	// Fetch back via REST.
	fetched, err := env.Teranode.REST.GetTxBytes(ctx, returnedTxid)
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("Teranode REST returns %s tx body", shape),
		err == nil && len(fetched) > 0,
		fmt.Sprintf("bytes=%d err=%v", len(fetched), err),
	))
	if err != nil {
		return nil
	}

	// Re-parse and verify the txid recomputes to the same value.
	parsed, err := bt.NewTxFromBytes(fetched)
	roundOK := err == nil && hex.EncodeToString(parsed.TxIDBytes()) == returnedTxid
	res.AcceptanceChecks = append(res.AcceptanceChecks, required(
		fmt.Sprintf("%s tx re-parses with matching txid", shape),
		roundOK,
		fmt.Sprintf("err=%v", err),
	))

	// Mine 1 block to confirm this tx on-chain so the next call (which may
	// spend the change UTXO via the shared funder) finds the parent in
	// storage rather than in the unconfirmed mempool. See helper.go on
	// confirmAndMine for why bare funder.Confirm is unsafe here.
	_ = confirmAndMine(ctx, env, returnedTxid, bres.Inputs, bres.Change)
	*txs = append(*txs, builtTx{shape: shape, expected: bres.TxID, txid: returnedTxid})
	return nil
}

// parseStandardBlock parses a serialized BSV block and returns the list of
// transaction IDs in order. Uses bt.NewTxFromStream-style iteration since
// libsv/go-bt/v2 does not export a top-level "Block" parser; we read header
// + VarInt + repeated bt.NewTxFromStream.
//
// Accepts two input shapes:
//  1. Bare block: [80-byte header][VarInt tx_count][txs…]
//  2. Legacy P2P wire frame: [4-byte network magic][4-byte size][80-byte
//     header][VarInt tx_count][txs…] — what Teranode's
//     /api/v1/block_legacy/{hash} REST endpoint returns.
func parseStandardBlock(blockBytes []byte) ([]string, error) {
	if len(blockBytes) < 81 {
		return nil, fmt.Errorf("block too short: %d bytes", len(blockBytes))
	}
	// Detect P2P wire frame by checking for known network magic values at
	// offset 0.
	//
	// Empirically, Teranode's /api/v1/block_legacy/{hash} REST endpoint
	// returns blocks prefixed with the Bitcoin legacy (BTC) mainnet magic
	// 0xf9beb4d9 (on-wire: f9 be b4 d9) regardless of the running network.
	// The BSV-specific magic values are retained for completeness.
	//
	// Magic bytes are written as uint32 little-endian; on-wire byte order:
	//   BTC mainnet / Teranode 0xd9b4bef9 → f9 be b4 d9  ← actual Teranode REST emission
	//   BSV mainnet            0xe8f3e1e3 → e3 e1 f3 e8
	//   BSV regtest            0xfabfb5da → da b5 bf fa
	//   BSV testnet            0xf4f3e5f4 → f4 e5 f3 f4
	//   BSV teratestnet        0x0c09010d → 0d 01 09 0c
	headerStart := 0
	if len(blockBytes) >= 88 {
		first4 := blockBytes[:4]
		switch {
		case first4[0] == 0xf9 && first4[1] == 0xbe && first4[2] == 0xb4 && first4[3] == 0xd9, // BTC mainnet (actual Teranode REST emission)
			first4[0] == 0xe3 && first4[1] == 0xe1 && first4[2] == 0xf3 && first4[3] == 0xe8, // BSV mainnet
			first4[0] == 0xda && first4[1] == 0xb5 && first4[2] == 0xbf && first4[3] == 0xfa, // BSV regtest
			first4[0] == 0xf4 && first4[1] == 0xe5 && first4[2] == 0xf3 && first4[3] == 0xf4, // BSV testnet
			first4[0] == 0x0d && first4[1] == 0x01 && first4[2] == 0x09 && first4[3] == 0x0c: // BSV teratestnet
			headerStart = 8 // skip 4 magic + 4 size
		}
	}
	if len(blockBytes) < headerStart+81 {
		return nil, fmt.Errorf("block too short after %d-byte preamble: %d bytes", headerStart, len(blockBytes))
	}
	// Skip preamble (if any) and the 80-byte header.
	body := blockBytes[headerStart+80:]
	// Read VarInt for tx count.
	count, n, err := readVarInt(body)
	if err != nil {
		return nil, fmt.Errorf("read tx count: %w", err)
	}
	// Sanity cap: a regtest block with > 10M txs is implausible and
	// indicates a parse error or non-standard block format.
	if count > 10_000_000 {
		return nil, fmt.Errorf("tx count %d implausibly large; block format may differ", count)
	}
	body = body[n:]
	out := make([]string, 0, count)
	for i := uint64(0); i < count; i++ {
		tx, used, err := bt.NewTxFromStream(body)
		if err != nil {
			return nil, fmt.Errorf("parse tx %d: %w", i, err)
		}
		out = append(out, hex.EncodeToString(tx.TxIDBytes()))
		body = body[used:]
	}
	return out, nil
}

// readVarInt decodes a Bitcoin-style VarInt and returns the value and bytes consumed.
func readVarInt(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("empty input")
	}
	switch b[0] {
	case 0xfd:
		if len(b) < 3 {
			return 0, 0, fmt.Errorf("truncated 0xfd varint")
		}
		return uint64(b[1]) | uint64(b[2])<<8, 3, nil
	case 0xfe:
		if len(b) < 5 {
			return 0, 0, fmt.Errorf("truncated 0xfe varint")
		}
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24, 5, nil
	case 0xff:
		if len(b) < 9 {
			return 0, 0, fmt.Errorf("truncated 0xff varint")
		}
		v := uint64(0)
		for i := 0; i < 8; i++ {
			v |= uint64(b[1+i]) << (8 * i)
		}
		return v, 9, nil
	default:
		return uint64(b[0]), 1, nil
	}
}
