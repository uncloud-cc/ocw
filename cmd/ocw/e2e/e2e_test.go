package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var (
	binaryPath      string
	dockerAvailable bool
)

// TestMain builds the binary once before running tests
func TestMain(m *testing.M) {
	// Check if Docker is available
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		fmt.Println("Warning: Docker not available, some tests will be skipped")
		dockerAvailable = false
	} else {
		dockerAvailable = true
	}

	// Build binary to temp location
	tmpDir, err := os.MkdirTemp("", "ocw-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmpDir, "ocw")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../.")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// TestCase defines a single test case
type TestCase struct {
	Name              string            // Test name
	Fixture           string            // Path to fixture file relative to testdata/
	Args              []string          // CLI args (e.g., []string{"build"} for job name)
	Env               map[string]string // Environment variables to set
	WantExitCode      int               // Expected exit code
	WantStdout        []string          // Patterns that must appear in stdout
	WantStdoutNot     []string          // Patterns that must NOT appear in stdout
	WantStderr        []string          // Patterns that must appear in stderr
	WantStdoutOrdered []string          // Patterns that must appear in stdout in order
	Timeout           time.Duration     // Test timeout (default: 60s)
	SkipNoDocker      bool              // Skip if Docker not available
	InterruptAfter    time.Duration     // Send SIGINT after this duration (for background service tests)
}

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	if !dockerAvailable {
		t.Skip("Docker not available")
	}
}

// runTest executes a test case
func runTest(t *testing.T, tc TestCase) {
	t.Helper()

	if tc.SkipNoDocker {
		skipIfNoDocker(t)
	}

	// Set default timeout
	timeout := tc.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	// Create temp dir for test execution
	tmpDir, err := os.MkdirTemp("", "ocw-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy fixture files to temp dir
	if tc.Fixture != "" {
		fixtureDir := filepath.Join("testdata", tc.Fixture)
		if err := copyDir(fixtureDir, tmpDir); err != nil {
			t.Fatalf("Failed to copy fixture: %v", err)
		}
	}

	// Build command args
	args := tc.Args

	// Run CLI with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = tmpDir
	cmd.Env = buildEnv(tc.Env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// If InterruptAfter is set, send SIGINT after the specified duration
	if tc.InterruptAfter > 0 {
		go func() {
			time.Sleep(tc.InterruptAfter)
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGINT)
			}
		}()
	}

	err = cmd.Run()

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("Test timeout after %v", timeout)
		} else {
			t.Fatalf("Failed to run command: %v", err)
		}
	}

	// Assert exit code
	if exitCode != tc.WantExitCode {
		t.Errorf("Exit code = %d, want %d", exitCode, tc.WantExitCode)
		t.Logf("Stdout:\n%s", stdout.String())
		t.Logf("Stderr:\n%s", stderr.String())
	}

	// Assert stdout patterns
	stdoutStr := stdout.String()
	for _, pattern := range tc.WantStdout {
		if !strings.Contains(stdoutStr, pattern) {
			t.Errorf("Stdout missing expected pattern: %q", pattern)
			t.Logf("Stdout:\n%s", stdoutStr)
		}
	}

	// Assert stdout NOT patterns
	for _, pattern := range tc.WantStdoutNot {
		if strings.Contains(stdoutStr, pattern) {
			t.Errorf("Stdout contains unexpected pattern: %q", pattern)
			t.Logf("Stdout:\n%s", stdoutStr)
		}
	}

	// Assert stderr patterns
	stderrStr := stderr.String()
	for _, pattern := range tc.WantStderr {
		if !strings.Contains(stderrStr, pattern) {
			t.Errorf("Stderr missing expected pattern: %q", pattern)
			t.Logf("Stderr:\n%s", stderrStr)
		}
	}

	// Assert ordered stdout patterns
	if len(tc.WantStdoutOrdered) > 0 {
		lastIndex := -1
		for _, pattern := range tc.WantStdoutOrdered {
			index := strings.Index(stdoutStr, pattern)
			if index == -1 {
				t.Errorf("Stdout missing ordered pattern: %q", pattern)
				t.Logf("Stdout:\n%s", stdoutStr)
				break
			}
			if index <= lastIndex {
				t.Errorf("Pattern %q appears before previous pattern (index %d <= %d)", pattern, index, lastIndex)
				t.Logf("Stdout:\n%s", stdoutStr)
				break
			}
			lastIndex = index
		}
	}
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// buildEnv builds environment variables for the command
func buildEnv(env map[string]string) []string {
	// Start with current environment
	envList := os.Environ()

	// Add custom env vars
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}

	return envList
}
