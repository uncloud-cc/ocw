package e2e_test

import (
	"testing"
)

func TestE2E_Jobs_RunSpecificJob(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Run Specific Job",
		Fixture:       "jobs",
		Args:          []string{"-f", "jobs.yaml", "build"},
		WantExitCode:  0,
		WantStdout:    []string{"Building..."},
		WantStdoutNot: []string{"Testing...", "Deploying..."},
		SkipNoDocker:  true,
	})
}

func TestE2E_Jobs_JobNotFound(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Job Not Found",
		Fixture:      "jobs",
		Args:         []string{"-f", "jobs.yaml", "nonexistent"},
		WantExitCode: 1,
		WantStderr:   []string{"job", "not found"},
		SkipNoDocker: true,
	})
}

func TestE2E_Jobs_ListJobs(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "List Jobs",
		Fixture:      "jobs",
		Args:         []string{"-f", "jobs.yaml"},
		WantExitCode: 1,
		WantStdout:   []string{"build", "test", "deploy"},
		SkipNoDocker: true,
	})
}
