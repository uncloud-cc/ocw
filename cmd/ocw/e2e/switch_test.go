package e2e_test

import (
	"testing"
)

func TestE2E_Switch_CaseMatch(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Switch Case Match",
		Fixture:       "switch",
		Args:          []string{"-f", "switch.yaml"},
		Env:           map[string]string{"ENVIRONMENT": "staging"},
		WantExitCode:  0,
		WantStdout:    []string{"Deploying to staging"},
		WantStdoutNot: []string{"production", "development"},
		SkipNoDocker:  true,
	})
}

func TestE2E_Switch_DefaultCase(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Switch Default Case",
		Fixture:       "switch",
		Args:          []string{"-f", "switch.yaml"},
		Env:           map[string]string{"ENVIRONMENT": "unknown"},
		WantExitCode:  0,
		WantStdout:    []string{"Deploying to development"},
		WantStdoutNot: []string{"staging", "production"},
		SkipNoDocker:  true,
	})
}

func TestE2E_Switch_NoMatch(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Switch No Match No Default",
		Fixture:      "switch",
		Args:         []string{"-f", "switch_no_default.yaml"},
		Env:          map[string]string{"ENVIRONMENT": "unknown"},
		WantExitCode: 0,
		SkipNoDocker: true,
	})
}
