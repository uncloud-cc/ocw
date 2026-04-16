package main

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
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
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

// ============================================================================
// Basic Execution Tests
// ============================================================================

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

// ============================================================================
// Jobs Tests
// ============================================================================

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

// ============================================================================
// Switch Tests
// ============================================================================

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

// ============================================================================
// Build Tests
// ============================================================================

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

// ============================================================================
// Nested Workflows Tests
// ============================================================================

func TestE2E_Nested_SequenceParallel(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Nested Sequence with Parallel",
		Fixture:      "nested",
		Args:         []string{"-f", "nested.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Setting up", "Running unit tests", "Running integration tests", "All tests completed"},
		SkipNoDocker: true,
	})
}

// ============================================================================
// Environment & Secrets Tests
// ============================================================================

func TestE2E_Env_Variables(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Environment Variables",
		Fixture:      "env",
		Args:         []string{"-f", "env.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Port: 5432", "User: testuser"},
		SkipNoDocker: true,
	})
}

func TestE2E_Env_SecretsMasked(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Secrets Masked",
		Fixture:       "env",
		Args:          []string{"-f", "secrets.yaml"},
		WantExitCode:  0,
		WantStdout:    []string{"User: testuser"},
		WantStdoutNot: []string{"secretpass123", "api-key-456"},
		SkipNoDocker:  true,
	})
}

func TestE2E_Env_SecretsShown(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Secrets Shown",
		Fixture:      "env",
		Args:         []string{"-f", "secrets.yaml", "--show-secrets"},
		WantExitCode: 0,
		WantStdout:   []string{"User: testuser", "Password: secretpass123", "API Key: api-key-456"},
		SkipNoDocker: true,
	})
}

// ============================================================================
// Template Tests
// ============================================================================

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

// ============================================================================
// Outputs Tests
// ============================================================================

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

// ============================================================================
// Background Services Tests
// ============================================================================

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

// ============================================================================
// Validation & Error Tests
// ============================================================================

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

// ============================================================================
// CLI Flags Tests
// ============================================================================

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
