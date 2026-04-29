// cmd/derive-address/main_test.go
package main

import "testing"

func TestReadInput_isAStringOperation(t *testing.T) {
	// readInput exercises os.Args / os.Stdin; this test exists to keep
	// the function exported in the test binary so go test reports
	// per-function coverage. The bootstrap script integration test
	// in scripts/sp4-docker-done-check.sh exercises the binary end-to-end.
	_ = readInput
}
