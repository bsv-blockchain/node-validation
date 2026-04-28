package testrunner

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/node-validation/config"
	"github.com/bsv-blockchain/node-validation/internal/matrix"
)

func TestNewEnv_defaults(t *testing.T) {
	cfg := config.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	env := NewEnv(cfg, logger, matrix.Load(), nil)
	if env.Now == nil {
		t.Fatal("Now must default")
	}
	if d := time.Since(env.Now()); d > time.Second {
		t.Errorf("default Now seems wrong: %v", d)
	}
	if len(env.Manifest.Entries) != 58 {
		t.Errorf("manifest not populated: %d entries", len(env.Manifest.Entries))
	}
}
