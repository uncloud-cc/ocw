package e2e_test

import (
	"testing"
)

func TestE2E_CLI_VersionFlag(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Version Flag",
		Fixture:      "",
		Args:         []string{"--version"},
		WantExitCode: 0,
		WantStdout:   []string{"ocw version"},
		SkipNoDocker: false,
	})
}

func TestE2E_CLI_HelpFlag(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Help Flag",
		Fixture:      "",
		Args:         []string{"--help"},
		WantExitCode: 0,
		WantStderr:   []string{"Usage:", "Options:"},
		SkipNoDocker: false,
	})
}

func TestE2E_CLI_VerboseOutput(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Verbose Output",
		Fixture:      "basic",
		Args:         []string{"-f", "hello_world.yaml", "--verbose"},
		WantExitCode: 0,
		WantStdout:   []string{"Hello OCW World"},
		SkipNoDocker: true,
	})
}
