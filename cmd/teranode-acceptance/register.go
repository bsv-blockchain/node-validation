// cmd/teranode-acceptance/register.go
package main

import (
	"github.com/bsv-blockchain/node-validation/internal/testrunner"
	"github.com/bsv-blockchain/node-validation/tests"
)

// registerTests is the single point where tests in the tests/ tree are
// registered with the suite. Keep entries alphabetised by ID.
func registerTests(suite *testrunner.Suite) {
	// Alphabetical by ID.
	suite.Register("NEW-NFR11", tests.RunNEWNFR11)
	suite.Register("NEW-NFR13", tests.RunNEWNFR13)
	suite.Register("OPS-3", tests.RunOPS3)
	suite.Register("PC-3", tests.RunPC3)
}
