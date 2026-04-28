package overrides

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_validFile(t *testing.T) {
	o, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if o.Reviewer == "" {
		t.Error("reviewer empty")
	}
	if !o.ReviewedAt.Equal(time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)) {
		t.Errorf("reviewed_at: %v", o.ReviewedAt)
	}
	if got, ok := o.Overrides["IBD-1"]; !ok || got.Decision != DecisionPass {
		t.Errorf("IBD-1 missing or wrong: %+v", got)
	}
}

func TestLoad_emptyPathIsEmpty(t *testing.T) {
	o, err := Load("")
	if err != nil {
		t.Fatalf("Load(empty): %v", err)
	}
	if len(o.Overrides) != 0 {
		t.Error("expected empty overrides")
	}
}

func TestLoad_rejectsMissingArtefacts(t *testing.T) {
	_, err := Load("testdata/missing-artefacts.yaml")
	if err == nil || !strings.Contains(err.Error(), "artefacts") {
		t.Errorf("want artefacts error, got %v", err)
	}
}
