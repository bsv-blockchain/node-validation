// cmd/teranode-acceptance/register.go
package main

import "github.com/bsv-blockchain/node-validation/internal/testrunner"

// registerTests is the single point where tests in the tests/ tree are
// registered with the suite. Empty in SP1; later sub-projects add lines
// like:
//
//	import "github.com/bsv-blockchain/node-validation/tests/pc1"
//	suite.Register("PC-1", pc1.Run)
//
// Keep entries alphabetised by ID.
func registerTests(suite *testrunner.Suite) {
	// Intentionally empty. Tests are added in SP5+.
	_ = suite
}
