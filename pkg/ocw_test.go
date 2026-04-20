package ocw

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockExecutor is a mock implementation of StepExecutor for testing
type MockExecutor struct {
	RunSteps   []Step
	BuildSteps []struct {
		Step        Step
		BuildConfig BuildConfig
	}
	Errors []error
}

// NewMockExecutor creates a new mock executor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		RunSteps: []Step{},
		BuildSteps: []struct {
			Step        Step
			BuildConfig BuildConfig
		}{},
		Errors: []error{},
	}
}

func (m *MockExecutor) ExecuteRunStep(ctx context.Context, step Step) error {
	m.RunSteps = append(m.RunSteps, step)
	if len(m.Errors) > len(m.RunSteps)+len(m.BuildSteps)-1 {
		return m.Errors[len(m.RunSteps)+len(m.BuildSteps)-1]
	}
	return nil
}

func (m *MockExecutor) ExecuteBuildStep(ctx context.Context, step Step, buildConfig BuildConfig) error {
	m.BuildSteps = append(m.BuildSteps, struct {
		Step        Step
		BuildConfig BuildConfig
	}{Step: step, BuildConfig: buildConfig})
	if len(m.Errors) > len(m.RunSteps)+len(m.BuildSteps)-1 {
		return m.Errors[len(m.RunSteps)+len(m.BuildSteps)-1]
	}
	return nil
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		expected    *OCW
	}{
		{
			name: "simple sequence workflow",
			content: `name: Hello World!
sequence:
  - name: hello
    image: alpine:latest
    cmd: echo "Hello World!"`,
			expectError: false,
			expected: &OCW{
				Name: "Hello World!",
				Sequence: []Step{
					{Name: "hello", Image: "alpine:latest", Cmd: "echo \"Hello World!\""},
				},
			},
		},
		{
			name: "parallel workflow",
			content: `name: Parallel workflow
parallel:
  - name: step a
    image: alpine
    cmd: echo "a"
  - name: step b
    image: alpine
    cmd: echo "b"`,
			expectError: false,
			expected: &OCW{
				Name: "Parallel workflow",
				Parallel: []Step{
					{Name: "step a", Image: "alpine", Cmd: "echo \"a\""},
					{Name: "step b", Image: "alpine", Cmd: "echo \"b\""},
				},
			},
		},
		{
			name: "workflow with environment variables",
			content: `name: Test workflow
env:
  DOCKER_USERNAME: testuser
  DOCKER_PASSWORD:
    secret: true
sequence:
  - name: build
    image: alpine
    cmd: echo "building..."`,
			expectError: false,
			expected: &OCW{
				Name: "Test workflow",
				Env: map[string]interface{}{
					"DOCKER_USERNAME": "testuser",
					"DOCKER_PASSWORD": map[string]interface{}{"secret": true},
				},
				Sequence: []Step{
					{Name: "build", Image: "alpine", Cmd: "echo \"building...\""},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := ParseBytes([]byte(tt.content))

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Name, ocw.Name)
			assert.Equal(t, len(tt.expected.Sequence), len(ocw.Sequence))
			assert.Equal(t, len(tt.expected.Parallel), len(ocw.Parallel))
		})
	}
}

func TestExtractSteps_SimpleSequence(t *testing.T) {
	content := `name: Simple Sequence
sequence:
  - name: step1
    image: alpine
    cmd: echo "step1"
  - name: step2
    image: alpine
    cmd: echo "step2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 2)
	assert.Equal(t, StepTypeRun, steps[0].Type)
	assert.Equal(t, "step1", steps[0].Step.Name)
	assert.Equal(t, StepTypeRun, steps[1].Type)
	assert.Equal(t, "step2", steps[1].Step.Name)
}

func TestExtractSteps_SimpleParallel(t *testing.T) {
	content := `name: Simple Parallel
parallel:
  - name: step1
    image: alpine
    cmd: echo "step1"
  - name: step2
    image: alpine
    cmd: echo "step2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 2)
	assert.Equal(t, StepTypeRun, steps[0].Type)
	assert.Equal(t, "step1", steps[0].Step.Name)
	assert.Equal(t, StepTypeRun, steps[1].Type)
	assert.Equal(t, "step2", steps[1].Step.Name)
}

func TestExtractSteps_NestedSequence(t *testing.T) {
	content := `name: Nested Sequence
sequence:
  - name: outer1
    image: alpine
    cmd: echo "outer1"
  - name: nested-sequence
    sequence:
      - name: inner1
        image: alpine
        cmd: echo "inner1"
      - name: inner2
        image: alpine
        cmd: echo "inner2"
  - name: outer2
    image: alpine
    cmd: echo "outer2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 4)
	assert.Equal(t, "outer1", steps[0].Step.Name)
	assert.Equal(t, StepTypeRun, steps[0].Type)
	assert.Equal(t, "inner1", steps[1].Step.Name)
	assert.Equal(t, StepTypeRun, steps[1].Type)
	assert.NotNil(t, steps[1].Parent) // parent is the nested-sequence container
	assert.Equal(t, "nested-sequence", steps[1].Parent.Step.Name)
	assert.Equal(t, "inner2", steps[2].Step.Name)
	assert.Equal(t, "outer2", steps[3].Step.Name)
}

func TestExtractSteps_NestedParallel(t *testing.T) {
	content := `name: Nested Parallel
sequence:
  - name: outer
    image: alpine
    cmd: echo "outer"
  - name: nested-parallel
    parallel:
      - name: inner1
        image: alpine
        cmd: echo "inner1"
      - name: inner2
        image: alpine
        cmd: echo "inner2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 3)
	assert.Equal(t, "outer", steps[0].Step.Name)
	assert.Equal(t, "inner1", steps[1].Step.Name)
	assert.Equal(t, "inner2", steps[2].Step.Name)
}

func TestExtractSteps_BuildStep(t *testing.T) {
	content := `name: Build Workflow
sequence:
  - name: build step
    build:
      image: myapp:latest
      dockerfile: Dockerfile
      context: .
      tags:
        - latest
        - v1.0.0`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 1)
	assert.Equal(t, StepTypeBuild, steps[0].Type)
	assert.Equal(t, "build step", steps[0].Step.Name)
	require.NotNil(t, steps[0].Step.Build)
	assert.Equal(t, "myapp:latest", steps[0].Step.Build.Image)
	assert.Equal(t, "Dockerfile", steps[0].Step.Build.Dockerfile)
	assert.Equal(t, ".", steps[0].Step.Build.Context)
	assert.Equal(t, []string{"latest", "v1.0.0"}, steps[0].Step.Build.Tags)
}

func TestExtractSteps_Jobs(t *testing.T) {
	content := `name: Jobs Workflow
jobs:
  test:
    name: Test Job
    parallel:
      - name: unit tests
        image: node:20
        cmd: npm test
      - name: lint
        image: node:20
        cmd: npm run lint

sequence:
  - name: run tests
    job: test`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 2)
	assert.Equal(t, "unit tests", steps[0].Step.Name)
	assert.Equal(t, StepTypeRun, steps[0].Type)
	assert.Equal(t, "lint", steps[1].Step.Name)
}

func TestExtractSteps_Switch(t *testing.T) {
	content := `name: Switch Workflow
switch:
  expression: "{{ env.BRANCH }}"
  cases:
    - value: main
      steps:
        - name: deploy prod
          image: alpine
          cmd: echo "deploying to prod"
    - value: dev
      steps:
        - name: deploy dev
          image: alpine
          cmd: echo "deploying to dev"
  default:
    - name: deploy staging
      image: alpine
      cmd: echo "deploying to staging"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	require.Len(t, steps, 3)
	assert.Equal(t, "deploy prod", steps[0].Step.Name)
	assert.Equal(t, "deploy dev", steps[1].Step.Name)
	assert.Equal(t, "deploy staging", steps[2].Step.Name)
}

func TestExecute_SimpleSequence(t *testing.T) {
	content := `name: Test Workflow
sequence:
  - name: step1
    image: alpine
    cmd: echo "step1"
  - name: step2
    image: alpine
    cmd: echo "step2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	executor := NewMockExecutor()
	workflow := NewWorkflow(ocw, executor)

	ctx := context.Background()
	err = workflow.Execute(ctx)
	require.NoError(t, err)

	assert.Len(t, executor.RunSteps, 2)
	assert.Equal(t, "step1", executor.RunSteps[0].Name)
	assert.Equal(t, "step2", executor.RunSteps[1].Name)
	assert.Len(t, executor.BuildSteps, 0)
}

func TestExecute_WithBuildStep(t *testing.T) {
	content := `name: Build Workflow
sequence:
  - name: build step
    build:
      image: myapp:latest
      dockerfile: Dockerfile`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	executor := NewMockExecutor()
	workflow := NewWorkflow(ocw, executor)

	ctx := context.Background()
	err = workflow.Execute(ctx)
	require.NoError(t, err)

	assert.Len(t, executor.RunSteps, 0)
	assert.Len(t, executor.BuildSteps, 1)
	assert.Equal(t, "myapp:latest", executor.BuildSteps[0].BuildConfig.Image)
}

func TestExecute_MixedSteps(t *testing.T) {
	content := `name: Mixed Workflow
sequence:
  - name: setup
    image: alpine
    cmd: echo "setup"
  - name: build
    build:
      image: myapp:latest
      dockerfile: Dockerfile
  - name: test
    image: alpine
    cmd: echo "testing"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	executor := NewMockExecutor()
	workflow := NewWorkflow(ocw, executor)

	ctx := context.Background()
	err = workflow.Execute(ctx)
	require.NoError(t, err)

	assert.Len(t, executor.RunSteps, 2)
	assert.Len(t, executor.BuildSteps, 1)
	assert.Equal(t, "setup", executor.RunSteps[0].Name)
	assert.Equal(t, "test", executor.RunSteps[1].Name)
}

func TestExecute_ErrorHandling(t *testing.T) {
	content := `name: Error Workflow
sequence:
  - name: step1
    image: alpine
    cmd: echo "step1"
  - name: step2
    image: alpine
    cmd: echo "step2"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	executor := NewMockExecutor()
	executor.Errors = []error{nil, fmt.Errorf("step2 failed")}
	workflow := NewWorkflow(ocw, executor)

	ctx := context.Background()
	err = workflow.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step2 failed")
	assert.Len(t, executor.RunSteps, 2)
}

func TestStepTypeString(t *testing.T) {
	assert.Equal(t, StepType("run"), StepTypeRun)
	assert.Equal(t, StepType("build"), StepTypeBuild)
	assert.Equal(t, StepType("parallel"), StepTypeParallel)
	assert.Equal(t, StepType("sequence"), StepTypeSequence)
	assert.Equal(t, StepType("switch"), StepTypeSwitch)
	assert.Equal(t, StepType("job"), StepTypeJob)
}

func TestWorkflow_Name(t *testing.T) {
	content := `name: My Test Workflow
sequence:
  - name: step1
    image: alpine
    cmd: echo "step1"`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	assert.Equal(t, "My Test Workflow", workflow.Name())

	// Test default name
	ocw.Name = ""
	assert.Equal(t, "Unnamed Workflow", workflow.Name())
}

func TestExtractSteps_ComplexNested(t *testing.T) {
	content := `name: Complex Workflow
sequence:
  - name: setup
    image: alpine
    cmd: echo "setup"
  - name: parallel-tests
    parallel:
      - name: unit tests
        image: node:20
        cmd: npm run unit
      - name: integration tests
        sequence:
          - name: start db
            image: postgres:15
            cmd: postgres
            background: true
          - name: run integration
            image: node:20
            cmd: npm run integration
  - name: build
    build:
      image: myapp:latest
      dockerfile: Dockerfile`

	ocw, err := ParseBytes([]byte(content))
	require.NoError(t, err)

	workflow := NewWorkflow(ocw, nil)
	steps := workflow.ExtractSteps()

	// Should extract: setup, unit tests, start db, run integration, build
	require.Len(t, steps, 5)

	assert.Equal(t, "setup", steps[0].Step.Name)
	assert.Equal(t, StepTypeRun, steps[0].Type)

	assert.Equal(t, "unit tests", steps[1].Step.Name)
	assert.Equal(t, StepTypeRun, steps[1].Type)

	assert.Equal(t, "start db", steps[2].Step.Name)
	assert.Equal(t, StepTypeRun, steps[2].Type)
	assert.True(t, steps[2].Step.Background)

	assert.Equal(t, "run integration", steps[3].Step.Name)
	assert.Equal(t, StepTypeRun, steps[3].Type)

	assert.Equal(t, "build", steps[4].Step.Name)
	assert.Equal(t, StepTypeBuild, steps[4].Type)
}

func TestParseBytes_InvalidYAML(t *testing.T) {
	invalidContent := `name: Invalid
sequence:
  - name: test
    image: alpine
    cmd: [unclosed bracket`

	_, err := ParseBytes([]byte(invalidContent))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing workflow")
}
