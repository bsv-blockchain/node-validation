// Package overrides loads and validates the reviewer-overrides YAML
// supplied via --reviewer-overrides. An override marks a manifest entry
// (typically a DOCUMENTATION_REVIEW or CONTRACTUAL row) as PASS or FAIL
// based on out-of-band evidence the runner cannot itself check.
package overrides

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Decision is the reviewer's verdict for a single manifest entry.
type Decision string

const (
	DecisionPass Decision = "PASS"
	DecisionFail Decision = "FAIL"
)

// Override is one row in the overrides YAML.
type Override struct {
	Decision  Decision `yaml:"decision"`
	Artefacts []string `yaml:"artefacts"`
	Note      string   `yaml:"note"`
}

// File is the parsed overrides document.
type File struct {
	Reviewer   string              `yaml:"reviewer"`
	ReviewedAt time.Time           `yaml:"reviewed_at"`
	Overrides  map[string]Override `yaml:"overrides"`
}

// Load reads and validates an overrides YAML. An empty path returns an
// empty File with no error — overrides are optional.
func Load(path string) (File, error) {
	if path == "" {
		return File{Overrides: map[string]Override{}}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("reading overrides: %w", err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parsing overrides: %w", err)
	}

	var errs []string
	if f.Reviewer == "" {
		errs = append(errs, "reviewer field is required")
	}
	if f.ReviewedAt.IsZero() {
		errs = append(errs, "reviewed_at field is required")
	}
	for id, o := range f.Overrides {
		if o.Decision != DecisionPass && o.Decision != DecisionFail {
			errs = append(errs, fmt.Sprintf("%s: decision must be PASS or FAIL, got %q", id, o.Decision))
		}
		if len(o.Artefacts) == 0 {
			errs = append(errs, fmt.Sprintf("%s: artefacts required", id))
		}
		if strings.TrimSpace(o.Note) == "" {
			errs = append(errs, fmt.Sprintf("%s: note required", id))
		}
	}

	if len(errs) > 0 {
		return File{}, fmt.Errorf("invalid overrides:\n  - %s", strings.Join(errs, "\n  - "))
	}
	if f.Overrides == nil {
		f.Overrides = map[string]Override{}
	}
	return f, nil
}
