package e2e_test

import (
	"testing"
)

func TestE2E_Validation_ValidateOnly(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Validate Only",
		Fixture:       "basic",
		Args:          []string{"-f", "hello_world.yaml", "--validate"},
		WantExitCode:  0,
		WantStdoutNot: []string{"Hello OCW World"}, // Should not run
		SkipNoDocker:  false,
	})
}

func TestE2E_Validation_InvalidSchema(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Invalid Schema",
		Fixture:      "errors",
		Args:         []string{"-f", "invalid_schema.yaml"},
		WantExitCode: 1,
		WantStderr:   []string{"failed to parse"},
		SkipNoDocker: false,
	})
}

func TestE2E_Validation_InvalidYAML(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Invalid YAML",
		Fixture:      "errors",
		Args:         []string{"-f", "invalid_yaml.yaml"},
		WantExitCode: 1,
		WantStderr:   []string{"failed to parse"},
		SkipNoDocker: false,
	})
}

func TestE2E_Validation_FileNotFound(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "File Not Found",
		Fixture:      "basic",
		Args:         []string{"-f", "nonexistent.yaml"},
		WantExitCode: 1,
		SkipNoDocker: false,
	})
}
