package tests

import (
	"testing"

	"github.com/bsv-blockchain/node-validation/internal/testrunner"
)

func TestDeriveStatus_allPass(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		ok("b", ""),
	}
	if got := deriveStatus(c); got != testrunner.StatusPass {
		t.Errorf("got %s want PASS", got)
	}
}

func TestDeriveStatus_anyRequiredFail(t *testing.T) {
	c := []testrunner.Check{
		ok("a", ""),
		fail("b", "boom"),
	}
	if got := deriveStatus(c); got != testrunner.StatusFail {
		t.Errorf("got %s want FAIL", got)
	}
}

func TestDeriveStatus_emptyIsError(t *testing.T) {
	if got := deriveStatus(nil); got != testrunner.StatusError {
		t.Errorf("got %s want ERROR", got)
	}
}

func TestClassifyRateLimit_429(t *testing.T) {
	if _, ok := classifyRateLimit(errFromString("HTTP 429 Too Many Requests")); !ok {
		t.Error("want classified as limit")
	}
}

func TestClassifyRateLimit_nilNotLimit(t *testing.T) {
	if _, ok := classifyRateLimit(nil); ok {
		t.Error("nil should not be a limit")
	}
}

// errFromString is a tiny helper so tests don't need to import errors.
type errString string

func (e errString) Error() string { return string(e) }

func errFromString(s string) error { return errString(s) }
