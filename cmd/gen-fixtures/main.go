// Command gen-fixtures generates the SP8 PC-2 and IBD-2 test fixtures
// from deterministic constructions. Output is committed YAML in
// tests/testdata/. The generator is reproducible — running twice
// produces byte-identical output. CI gate: `make verify` runs it and
// `git diff --exit-code`.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	out := flag.String("out", "tests/testdata", "output directory for fixture YAML files")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	pc2Fixtures := generatePC2Fixtures()
	ibd2Fixtures := generateIBD2Fixtures()

	if err := writeYAML(filepath.Join(*out, "historical_scripts.yaml"), pc2Fixtures); err != nil {
		fmt.Fprintf(os.Stderr, "write pc2: %v\n", err)
		os.Exit(1)
	}
	if err := writeYAML(filepath.Join(*out, "historical_utxos.yaml"), ibd2Fixtures); err != nil {
		fmt.Fprintf(os.Stderr, "write ibd2: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d PC-2 fixtures and %d IBD-2 fixtures to %s/\n",
		len(pc2Fixtures), len(ibd2Fixtures), *out)
}

// fixture is one entry in a historical_*.yaml file.
type fixture struct {
	ID               string `yaml:"id"`
	Category         string `yaml:"category"`
	Description      string `yaml:"description"`
	HexTx            string `yaml:"hex_tx"`
	ExpectedValid    bool   `yaml:"expected_valid"`
	ExpectedCategory string `yaml:"expected_category"`
	Provenance       string `yaml:"provenance"`
	Notes            string `yaml:"notes,omitempty"`
}

func writeYAML(path string, fxs []fixture) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(fxs); err != nil {
		return err
	}
	return enc.Close()
}
