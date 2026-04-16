package e2e_test

import (
	"testing"
)

func TestE2E_Basic_HelloWorld(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Hello World",
		Fixture:      "basic",
		Args:         []string{"-f", "hello_world.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Hello OCW World"},
		SkipNoDocker: true,
	})
}

func TestE2E_Basic_Sequence(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:              "Sequential Workflow",
		Fixture:           "basic",
		Args:              []string{"-f", "sequence.yaml"},
		WantExitCode:      0,
		WantStdoutOrdered: []string{"First step", "Second step", "Third step"},
		SkipNoDocker:      true,
	})
}

func TestE2E_Basic_Parallel(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Parallel Workflow",
		Fixture:      "basic",
		Args:         []string{"-f", "parallel.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Running task A", "Running task B", "Running task C"},
		SkipNoDocker: true,
	})
}
