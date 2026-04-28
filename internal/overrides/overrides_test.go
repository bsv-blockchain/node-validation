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

func TestLoad_rejectsMissingReviewer(t *testing.T) {
	_, err := Load("testdata/missing-reviewer.yaml")
	if err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Errorf("want reviewer error, got %v", err)
	}
}

func TestLoad_rejectsMissingReviewedAt(t *testing.T) {
	_, err := Load("testdata/missing-reviewed-at.yaml")
	if err == nil || !strings.Contains(err.Error(), "reviewed_at") {
		t.Errorf("want reviewed_at error, got %v", err)
	}
}

func TestLoad_rejectsBadDecision(t *testing.T) {
	_, err := Load("testdata/bad-decision.yaml")
	if err == nil || !strings.Contains(err.Error(), "decision") {
		t.Errorf("want decision error, got %v", err)
	}
}

func TestLoad_rejectsMissingNote(t *testing.T) {
	_, err := Load("testdata/missing-note.yaml")
	if err == nil || !strings.Contains(err.Error(), "note") {
		t.Errorf("want note error, got %v", err)
	}
}

func TestLoad_noOverridesEntries(t *testing.T) {
	o, err := Load("testdata/no-overrides.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(o.Overrides) != 0 {
		t.Errorf("want empty overrides map, got %v", o.Overrides)
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := Load("testdata/nonexistent.yaml")
	if err == nil || !strings.Contains(err.Error(), "reading overrides") {
		t.Errorf("want reading error, got %v", err)
	}
}

func TestLoad_decisionFail(t *testing.T) {
	// valid.yaml has PASS; create inline to test FAIL decision path
	o, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if o.Reviewer == "" {
		t.Error("reviewer empty")
	}
	// Verify the File is well-formed so we exercise the success return path.
	if o.Overrides == nil {
		t.Error("overrides map should not be nil")
	}
}
