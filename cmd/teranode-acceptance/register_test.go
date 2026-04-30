// cmd/teranode-acceptance/register_test.go
package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func TestRegisterTests_SP8RegistersSixteen(t *testing.T) {
	cfg := config.Config{TestTimeout: time.Minute}
	env := testrunner.NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
	suite := testrunner.NewSuite(env)
	registerTests(suite)
	results := suite.Run(testContext(t))
	if len(results) != 16 {
		t.Fatalf("expected 16 results, got %d", len(results))
	}
	wantIDs := map[string]bool{
		"CLIENT-1": false, "CLIENT-2": false, "CLIENT-3": false,
		"IBD-2": false, "INTER-2": false,
		"NEW-FR7": false, "NEW-FR8": false, "NEW-FR9": false,
		"NEW-FR10": false, "NEW-FR11": false,
		"NEW-NFR7": false, "NEW-NFR11": false, "NEW-NFR13": false,
		"OPS-3": false, "PC-2": false, "PC-3": false,
	}
	for _, r := range results {
		if _, ok := wantIDs[r.ID]; ok {
			wantIDs[r.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing result for %s", id)
		}
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
