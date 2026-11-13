package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPERF1SubmissionErrorRate(t *testing.T) {
	for _, tc := range []struct {
		name               string
		submitted, errored int
		pass               bool
	}{
		{"audit observation", 1784, 1216, false},
		{"one percent", 99, 1, true},
		{"above one percent", 98, 2, false},
		{"shortened run uses actual attempts", 9, 1, false},
		{"all failed", 0, 10, false},
		{"no attempts", 0, 0, false},
		{"all successful", 100, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := perf1ErrorRateCheck(100, tc.submitted, tc.errored)
			require.True(t, check.Required)
			require.Equal(t, tc.pass, check.Pass, check.Detail)
		})
	}
}
