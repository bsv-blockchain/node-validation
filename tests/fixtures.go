// Package tests — fixture loader for PC-2 / IBD-2 tests.
package tests

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Fixture is one entry in a historical_*.yaml file.
type Fixture struct {
	ID               string `yaml:"id"`
	Category         string `yaml:"category"`
	Description      string `yaml:"description"`
	HexTx            string `yaml:"hex_tx"`
	ExpectedValid    bool   `yaml:"expected_valid"`
	ExpectedCategory string `yaml:"expected_category"` // matches compare.RejectionCategory
	Provenance       string `yaml:"provenance"`
	Notes            string `yaml:"notes,omitempty"`
}

// LoadFixtures reads and parses a historical_*.yaml file.
func LoadFixtures(path string) ([]Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load fixtures %s: %w", path, err)
	}
	var fxs []Fixture
	if err := yaml.Unmarshal(b, &fxs); err != nil {
		return nil, fmt.Errorf("parse fixtures %s: %w", path, err)
	}
	return fxs, nil
}
