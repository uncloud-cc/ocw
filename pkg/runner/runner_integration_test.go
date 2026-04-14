//go:build integration

package runner

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestRunner_SimpleWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - id: hello
    image: alpine:latest
    run: echo "Hello, World!"
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tmpDir := t.TempDir()
	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_JobExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
schemaVersion: v1
name: test-workflow
jobs:
  test:
    sequence:
      - id: test_step
        image: alpine:latest
        run: echo "Testing job execution"
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tmpDir := t.TempDir()
	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.RunJob(ctx, ocw, "test"); err != nil {
		t.Errorf("RunJob() error = %v", err)
	}
}

func TestRunner_ParallelSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
schemaVersion: v1
name: test-workflow
parallel:
  - image: alpine:latest
    run: echo "Parallel task 1"
  - image: alpine:latest
    run: echo "Parallel task 2"
  - image: alpine:latest
    run: echo "Parallel task 3"
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tmpDir := t.TempDir()
	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_SequenceSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - image: alpine:latest
    run: echo "Step 1"
  - image: alpine:latest
    run: echo "Step 2"
  - image: alpine:latest
    run: echo "Step 3"
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tmpDir := t.TempDir()
	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_VolumeMount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - image: alpine:latest
    run: cat /data/test.txt
    volumes:
      - ./:/data
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_EnvironmentVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
schemaVersion: v1
name: test-workflow
env:
  TEST_VAR: test_value
sequence:
  - image: alpine:latest
    run: 'echo "TEST_VAR=$TEST_VAR"'
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tmpDir := t.TempDir()
	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_BuildStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a temp directory with a Dockerfile
	tmpDir := t.TempDir()
	dockerfile := `FROM alpine:latest
RUN echo "Built successfully" > /build-marker
`
	if err := os.WriteFile(tmpDir+"/Dockerfile", []byte(dockerfile), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - id: build
    build:
      image: test-image:latest
      context: .
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Note: Built images are left for manual cleanup with: docker rmi test-image:latest
}

func TestRunner_StepOutputs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - id: output_step
    image: alpine:latest
    run: 'echo "version=1.0.0" >> $OCW_OUTPUT'
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}

	// Check if output was captured
	if val, ok := r.templateCtx.GetStepOutput("output_step", "version"); !ok || val != "1.0.0" {
		t.Errorf("Expected output 'version=1.0.0', got '%s'", val)
	}
}

func TestRunner_TemplateInterpolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	yaml := `
schemaVersion: v1
name: test-workflow
sequence:
  - id: first
    image: alpine:latest
    run: 'echo "value=hello" >> $OCW_OUTPUT'
  - image: alpine:latest
    run: 'echo "The value is: {{ steps.first.outputs.value }}"'
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	r := NewRunner(tmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}
