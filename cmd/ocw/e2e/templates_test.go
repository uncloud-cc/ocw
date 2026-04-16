package e2e_test

import (
	"testing"
)

func TestE2E_Template_WorkflowName(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Template Workflow Name",
		Fixture:      "templates",
		Args:         []string{"-f", "template_workflow.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Workflow name is Template Test Workflow"},
		SkipNoDocker: true,
	})
}

func TestE2E_Template_Env(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Template Env Variable",
		Fixture:      "templates",
		Args:         []string{"-f", "template_env.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Value is hello"},
		SkipNoDocker: true,
	})
}
