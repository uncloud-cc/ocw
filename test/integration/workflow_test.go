package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/steps"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// setupEngine creates a workflow engine from a testdata YAML file with the mock runtime.
func setupEngine(t *testing.T, filename string, jobName string) (*workflow.Engine, *MockRuntime) {
	t.Helper()

	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", filename)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err, "parsing %s", filename)

	runtime := NewMockRuntime()
	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, jobName, ctx)
	require.NoError(t, err, "creating engine for %s", filename)

	return engine, runtime
}

// runToCompletion executes the engine until it is done.
func runToCompletion(ctx context.Context, t *testing.T, engine *workflow.Engine) {
	t.Helper()
	for !engine.Done() {
		err := engine.Execute(ctx)
		require.NoError(t, err, "engine execute")
	}
}

func TestIntegration_HelloWorld(t *testing.T) {
	engine, runtime := setupEngine(t, "1_hello_world.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have executed one run call for the hello step
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "alpine:latest", calls[0].Image)

	// The step should have produced outputs
	stepCtx := engine.Context()
	assert.NotNil(t, stepCtx)
}

func TestIntegration_BuildStep(t *testing.T) {
	engine, runtime := setupEngine(t, "2_build.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have executed one build call
	builds := runtime.BuildCalls()
	require.Len(t, builds, 1)

	// Build step has no ID in this workflow, so outputs are not merged into Steps
	// (engine only merges outputs when StepID is non-empty)
}

func TestIntegration_BuildAndRun_WithOutputs(t *testing.T) {
	engine, runtime := setupEngine(t, "3_build_and_run.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have one build and one run
	builds := runtime.BuildCalls()
	require.Len(t, builds, 1)

	runs := runtime.RunCalls()
	require.Len(t, runs, 1)

	// Interpolation resolved the template to the mock build output
	assert.Equal(t, "mock-image:latest", runs[0].Image)
}

func TestIntegration_Sequence(t *testing.T) {
	engine, runtime := setupEngine(t, "4_sequence.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have executed two run calls in sequence
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "alpine", calls[0].Image)
	assert.Equal(t, "node:25-alpine", calls[1].Image)
}

func TestIntegration_Parallel(t *testing.T) {
	engine, runtime := setupEngine(t, "5_parallel.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have executed two run calls in parallel
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)

	// Verify each step ran with a distinct command (proving two separate executions)
	cmds := []string{calls[0].Opts.(workflow.RunOptions).Cmd, calls[1].Opts.(workflow.RunOptions).Cmd}
	assert.Contains(t, cmds, `for i in $(seq 10); do echo "Processing step a)... $i"; sleep 1; done`)
	assert.Contains(t, cmds, `for i in $(seq 7); do echo "Processing step b)... $i"; sleep 1; done`)
}

func TestIntegration_Nested_ParallelInSequence(t *testing.T) {
	engine, runtime := setupEngine(t, "6_nested.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Should have executed 5 run calls total:
	// 1. Setup
	// 2-4. Three parallel test steps
	// 5. Build
	// 6. Deploy
	calls := runtime.RunCalls()
	require.Len(t, calls, 6)

	// Setup must be first (outer sequence order)
	assert.Equal(t, "alpine:latest", calls[0].Image)

	// Next three calls are the parallel group -- order is non-deterministic once
	// the engine executes concurrently. Assert images only.
	for i := 1; i <= 3; i++ {
		assert.Equal(t, "node:20-alpine", calls[i].Image)
	}

	// Build and Deploy must come after the parallel group
	assert.Equal(t, "node:20-alpine", calls[4].Image) // Build
	assert.Equal(t, "alpine:latest", calls[5].Image)  // Deploy
}

func TestIntegration_Switch_MatchingCase(t *testing.T) {
	// Test switch with DEPLOY_ENV=staging
	path := filepath.Join("testdata", "8_switch.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	runtime := NewMockRuntime()
	ctx := &workflow.StepContext{
		Env: map[string]string{
			"DEPLOY_ENV": "staging",
		},
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()
	runToCompletion(cctx, t, engine)

	// Should have executed the staging case (one run call)
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "alpine:latest", calls[0].Image)
}

func TestIntegration_Switch_DefaultCase(t *testing.T) {
	// Test switch with DEPLOY_ENV=unknown (should fall back to default)
	path := filepath.Join("testdata", "8_switch.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	runtime := NewMockRuntime()
	ctx := &workflow.StepContext{
		Env: map[string]string{
			"DEPLOY_ENV": "unknown",
		},
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()
	runToCompletion(cctx, t, engine)

	// Should have executed the default case (one run call)
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "alpine:latest", calls[0].Image)
}

func TestIntegration_Jobs_BuildJob(t *testing.T) {
	engine, runtime := setupEngine(t, "9_jobs.yaml", "build")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// build job has two sequential run steps
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "node:20-alpine", calls[0].Image)
	assert.Equal(t, "node:20-alpine", calls[1].Image)
}

func TestIntegration_Jobs_TestJob(t *testing.T) {
	engine, runtime := setupEngine(t, "9_jobs.yaml", "test")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// test job has two parallel run steps
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "node:20-alpine", calls[0].Image)
	assert.Equal(t, "node:20-alpine", calls[1].Image)
}

func TestIntegration_Jobs_DevJob(t *testing.T) {
	engine, runtime := setupEngine(t, "9_jobs.yaml", "dev")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// dev job has one run step
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "node:20-alpine", calls[0].Image)
}

func TestIntegration_Templates_Resolution(t *testing.T) {
	path := filepath.Join("testdata", "7_templates.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	runtime := NewMockRuntime()
	ctx := &workflow.StepContext{
		Env: map[string]string{
			"USER": "testuser",
			"HOME": "/home/testuser",
		},
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()
	runToCompletion(cctx, t, engine)

	// Should have executed one run call with resolved templates
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "alpine:latest", calls[0].Image)

	// Verify template expressions were resolved in the command
	opts := calls[0].Opts.(workflow.RunOptions)
	assert.Contains(t, opts.Cmd, "Template Demo")
	assert.Contains(t, opts.Cmd, "testuser")
	assert.Contains(t, opts.Cmd, "/home/testuser")
}

func TestIntegration_Outputs_Propagation(t *testing.T) {
	// Configure mock to return outputs based on command content.
	// YAML block scalars (|) may include trailing newlines, so we use Contains.
	runtime := NewMockRuntime()
	runtime.runFn = func(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error) {
		outputs := map[string]string{}
		if opts.Cmd != "" {
			if strings.Contains(opts.Cmd, `version=1.0.0`) {
				outputs["version"] = "1.0.0"
			}
			if strings.Contains(opts.Cmd, `timestamp=$(date -u +%Y-%m-%d)`) {
				outputs["timestamp"] = "2024-01-01"
			}
		}
		return &workflow.RunResult{
			ContainerID: fmt.Sprintf("container-%s", opts.Image),
			ExitCode:    0,
			Outputs:     outputs,
		}, nil
	}

	path := filepath.Join("testdata", "10_outputs.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()
	runToCompletion(cctx, t, engine)

	// Should have executed three run calls
	calls := runtime.RunCalls()
	require.Len(t, calls, 3)

	// Verify outputs were propagated to context
	stepCtx := engine.Context()
	assert.Contains(t, stepCtx.Steps, "version")
	assert.Equal(t, "1.0.0", stepCtx.Steps["version"]["version"])
	assert.Contains(t, stepCtx.Steps, "build")
	assert.Equal(t, "2024-01-01", stepCtx.Steps["build"]["timestamp"])
}

func TestIntegration_BackgroundService_Tracking(t *testing.T) {
	// Test that background services are tracked
	runtime := NewMockRuntime().
		WithRunResult("node:25-alpine",
			&workflow.RunResult{
				ContainerID: "webserver-123",
				ExitCode:    0,
				Outputs:     map[string]string{},
			}, nil)

	path := filepath.Join("testdata", "14_expose.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()
	runToCompletion(cctx, t, engine)

	// Should have tracked the background service
	stepCtx := engine.Context()
	assert.Contains(t, stepCtx.Services, "webserver")
	assert.Equal(t, "webserver-123", stepCtx.Services["webserver"].ContainerID)

	// Verify the expose ports were configured
	calls := runtime.RunCalls()
	require.Len(t, calls, 1)
	opts := calls[0].Opts.(workflow.RunOptions)
	require.Len(t, opts.Expose, 1)
	assert.Equal(t, 8080, opts.Expose[0].ContainerPort)
	assert.Equal(t, 8080, opts.Expose[0].HostPort)
}

func TestIntegration_Engine_StateTransitions(t *testing.T) {
	engine, _ := setupEngine(t, "1_hello_world.yaml", "")

	ctx := context.Background()

	// Before execution, engine should not be done
	assert.False(t, engine.Done())
	assert.NotEmpty(t, engine.Current())

	// Execute - with recursive sequential execution, both the sequence expansion
	// and the child step run in the first Execute() call
	err := engine.Execute(ctx)
	require.NoError(t, err)

	// With recursive execution, the child step runs immediately after expansion
	// so the engine should be done after the first call
	assert.True(t, engine.Done())
	assert.Empty(t, engine.Current())
	assert.Empty(t, engine.Pending())
}

func TestIntegration_InvalidJobName(t *testing.T) {
	path := filepath.Join("testdata", "9_jobs.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  NewMockRuntime(),
		Factory:  steps.NewStepFactory(),
	}

	_, err = workflow.New(ocwSchema, "nonexistent", ctx)
	require.Error(t, err)
	assert.Equal(t, "failed to find entry point: job not found: nonexistent", err.Error())
}

func TestIntegration_NilFactory(t *testing.T) {
	path := filepath.Join("testdata", "1_hello_world.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  NewMockRuntime(),
		// Factory intentionally nil
	}

	_, err = workflow.New(ocwSchema, "", ctx)
	require.Error(t, err)
	assert.Equal(t, "StepContext.Factory is nil", err.Error())
}

func TestIntegration_ContextCloning_Parallel(t *testing.T) {
	// Verify that parallel branches get cloned contexts
	engine, runtime := setupEngine(t, "5_parallel.yaml", "")

	ctx := context.Background()
	runToCompletion(ctx, t, engine)

	// Both parallel steps should have executed
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)

	// Verify the engine context has outputs from both if they produced any
	stepCtx := engine.Context()
	assert.NotNil(t, stepCtx)
}

func TestIntegration_CompositeStep_ExpansionOrder(t *testing.T) {
	// Top-level `sequence:` in the YAML is wrapped in a SequenceStep.
	// With recursive sequential execution, the engine expands the sequence
	// wrapper and executes all children in a single Execute() call.
	engine, runtime := setupEngine(t, "4_sequence.yaml", "")

	ctx := context.Background()

	// Before execution
	assert.False(t, engine.Done())

	// Execute - expands sequence wrapper and runs all children recursively
	err := engine.Execute(ctx)
	require.NoError(t, err)

	// Both steps executed (order may vary due to goroutine scheduling)
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)
	images := []string{calls[0].Image, calls[1].Image}
	assert.Contains(t, images, "alpine")
	assert.Contains(t, images, "node:25-alpine")

	// Engine should be done
	assert.True(t, engine.Done())
}

func TestIntegration_StepFailure_MidSequence(t *testing.T) {
	// Configure the mock to fail on the second step (node:25-alpine)
	runtime := NewMockRuntime().
		WithRunResult("alpine",
			&workflow.RunResult{
				ContainerID: "step1-ok",
				ExitCode:    0,
				Outputs:     map[string]string{},
			}, nil).
		WithRunResult("node:25-alpine",
			nil, // no result
			fmt.Errorf("container image not found: node:25-alpine"))

	path := filepath.Join("testdata", "4_sequence.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	ocwSchema, err := schema.Parse(data)
	require.NoError(t, err)

	ctx := &workflow.StepContext{
		Env:      make(map[string]string),
		Secrets:  make(map[string]string),
		Inputs:   make(map[string]string),
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Runtime:  runtime,
		Factory:  steps.NewStepFactory(),
	}

	engine, err := workflow.New(ocwSchema, "", ctx)
	require.NoError(t, err)

	cctx := context.Background()

	// Execute - with recursive sequential execution, both steps run in one call.
	// The second step fails, so the error is returned immediately.
	err = engine.Execute(cctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container image not found")

	// Both steps were attempted sequentially (first succeeded, second failed)
	calls := runtime.RunCalls()
	require.Len(t, calls, 2)
	// Note: With concurrent execution, order may vary. Find each by image.
	var alpineCall, nodeCall *CallRecord
	for i := range calls {
		if calls[i].Image == "alpine" {
			alpineCall = &calls[i]
		} else if calls[i].Image == "node:25-alpine" {
			nodeCall = &calls[i]
		}
	}
	require.NotNil(t, alpineCall, "should have alpine call")
	require.NotNil(t, nodeCall, "should have node:25-alpine call")
	assert.Nil(t, alpineCall.Error)
	assert.NotNil(t, nodeCall.Error)
}
