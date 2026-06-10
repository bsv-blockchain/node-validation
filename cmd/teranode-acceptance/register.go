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
	suite.Register("CLIENT-1", tests.RunCLIENT1)
	suite.Register("CLIENT-2", tests.RunCLIENT2)
	suite.Register("CLIENT-3", tests.RunCLIENT3)
	suite.Register("IBD-2", tests.RunIBD2)
	suite.Register("INTER-1", tests.RunINTER1)
	suite.Register("INTER-2", tests.RunINTER2)
	suite.Register("NEW-FR10", tests.RunNEWFR10)
	suite.Register("NEW-FR11", tests.RunNEWFR11)
	suite.Register("NEW-FR7", tests.RunNEWFR7)
	// NEW-FR8 retired: FR-8 (fee estimation) is covered externally by
	// Arcade / Arc, not the Teranode RPC. Marked RESOLVED_EXTERNAL in the
	// manifest and intentionally not registered. See tests/new_fr8.go.
	suite.Register("NEW-FR9", tests.RunNEWFR9)
	suite.Register("NEW-NFR11", tests.RunNEWNFR11)
	suite.Register("NEW-NFR13", tests.RunNEWNFR13)
	suite.Register("NEW-NFR7", tests.RunNEWNFR7)
	suite.Register("OPS-3", tests.RunOPS3)
	suite.Register("PC-1", tests.RunPC1)
	suite.Register("PC-2", tests.RunPC2)
	suite.Register("PC-3", tests.RunPC3)
	suite.Register("PERF-1", tests.RunPERF1)
}
