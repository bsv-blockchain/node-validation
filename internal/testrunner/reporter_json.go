// internal/testrunner/reporter_json.go
package testrunner

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteJSON serialises a ReportModel to disk as pretty-printed JSON
// with deterministic key order (the struct field order).
func WriteJSON(path string, m ReportModel) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
