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
	suite.Register("CLIENT-2", tests.RunCLIENT2)
	suite.Register("INTER-2", tests.RunINTER2)
	suite.Register("NEW-FR10", tests.RunNEWFR10)
	suite.Register("NEW-FR11", tests.RunNEWFR11)
	suite.Register("NEW-FR7", tests.RunNEWFR7)
	suite.Register("NEW-FR8", tests.RunNEWFR8)
	suite.Register("NEW-FR9", tests.RunNEWFR9)
	suite.Register("NEW-NFR11", tests.RunNEWNFR11)
	suite.Register("NEW-NFR13", tests.RunNEWNFR13)
	suite.Register("NEW-NFR7", tests.RunNEWNFR7)
	suite.Register("OPS-3", tests.RunOPS3)
	suite.Register("PC-3", tests.RunPC3)
}
