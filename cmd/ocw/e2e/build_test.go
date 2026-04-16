package e2e_test

import (
	"testing"
	"time"
)

func TestE2E_Build_Simple(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Simple Build",
		Fixture:      "build",
		Args:         []string{"-f", "build_simple.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Built: test-simple-build"},
		Timeout:      120 * time.Second,
		SkipNoDocker: true,
	})
}

func TestE2E_Build_BuildAndRun(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Build And Run",
		Fixture:      "build",
		Args:         []string{"-f", "build_and_run.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Built: test-build-and-run", "Running built container"},
		Timeout:      120 * time.Second,
		SkipNoDocker: true,
	})
}
