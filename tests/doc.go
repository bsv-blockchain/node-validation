// Package tests contains the acceptance-test functions registered with
// the suite by cmd/teranode-acceptance/register.go. Each public Run*
// function follows the testrunner.TestFunc signature and is named
// after the test ID it implements (RunOPS3, RunPC3, etc.).
//
// Tests run against a live Teranode + SV Node pair (the SP4-DOCKER
// compose stack by default) and use the typed clients in env.Teranode,
// env.SVNode, plus the txgen funder in env.TxGen.
package tests
