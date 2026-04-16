package e2e_test

import (
	"testing"
)

func TestE2E_Nested_SequenceParallel(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Nested Sequence with Parallel",
		Fixture:      "nested",
		Args:         []string{"-f", "nested.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Setting up", "Running unit tests", "Running integration tests", "All tests completed"},
		SkipNoDocker: true,
	})
}
