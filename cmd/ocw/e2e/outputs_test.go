package e2e_test

import (
	"testing"
)

func TestE2E_Outputs_StepOutputs(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Step Outputs",
		Fixture:      "outputs",
		Args:         []string{"-f", "outputs.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Generated version", "Version is 1.0.0"},
		SkipNoDocker: true,
	})
}

func TestE2E_Outputs_OutputPassing(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Output Passing",
		Fixture:      "outputs",
		Args:         []string{"-f", "output_passing.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Generated version", "final_version: 2.0.0"},
		SkipNoDocker: true,
	})
}
