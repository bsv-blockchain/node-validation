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

func TestRegisterTests_SP1IsEmpty(t *testing.T) {
	cfg := config.Config{TestTimeout: time.Minute}
	env := testrunner.NewEnv(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), matrix.Load(), nil)
	suite := testrunner.NewSuite(env)
	registerTests(suite)
	results := suite.Run(testContext(t))
	if len(results) != 0 {
		t.Fatalf("SP1 should register zero tests, got %d", len(results))
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
