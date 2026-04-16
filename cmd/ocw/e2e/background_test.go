package e2e_test

import (
	"testing"
	"time"
)

func TestE2E_Background_SimpleService(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:           "Background Service",
		Fixture:        "background",
		Args:           []string{"-f", "background.yaml"},
		WantExitCode:   0,
		WantStdout:     []string{"PONG", "completed successfully", "Background services running"},
		InterruptAfter: 5 * time.Second,
		Timeout:        10 * time.Second,
		SkipNoDocker:   true,
	})
}

func TestE2E_Background_Needs(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:           "Background Service with Needs",
		Fixture:        "background",
		Args:           []string{"-f", "needs.yaml"},
		WantExitCode:   0,
		WantStdout:     []string{"PONG", "completed successfully", "Background services running"},
		InterruptAfter: 5 * time.Second,
		Timeout:        10 * time.Second,
		SkipNoDocker:   true,
	})
}
