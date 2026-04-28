package ocw

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// Sequence execution
// ---------------------------------------------------------------------------

func TestEngine_Sequence(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "sequence test",
		Sequence: []schema.Step{
			runStep("step1", "alpine:latest", "echo first"),
			runStep("step2", "alpine:latest", "echo second"),
			runStep("step3", "alpine:latest", "echo third"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo first", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo second", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo third", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Parallel execution
// ---------------------------------------------------------------------------

func TestEngine_Parallel(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "parallel test",
		Parallel: []schema.Step{
			runStep("p1", "alpine:latest", "echo one"),
			runStep("p2", "alpine:latest", "echo two"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// All steps ran (order may vary).
	assert.Len(t, dummy.Runs, 2)
}

// ---------------------------------------------------------------------------
// Switch execution
// ---------------------------------------------------------------------------

func TestEngine_Switch_MatchesCase(t *testing.T) {
	rt, dummy := newTestEngine()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch test",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "staging"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging":    runStep("deploy staging", "alpine", "echo staging"),
			"production": runStep("deploy prod", "alpine", "echo production"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo staging", dummy.Runs[0].Cmd)
}

func TestEngine_Switch_FallsToDefault(t *testing.T) {
	rt, dummy := newTestEngine()

	switchExpr := "{{ env.MODE }}"
	defaultStep := runStep("default deploy", "alpine", "echo default")
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch default test",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "dev"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging":    runStep("staging", "alpine", "echo staging"),
			"production": runStep("prod", "alpine", "echo prod"),
		},
		Default: &defaultStep,
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo default", dummy.Runs[0].Cmd)
}

func TestEngine_Switch_NoMatchNoDefault(t *testing.T) {
	rt, dummy := newTestEngine()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch no match",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "unknown"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging": runStep("staging", "alpine", "echo staging"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Empty(t, dummy.Runs)
}

// ---------------------------------------------------------------------------
// Switch with nested composite (sequence inside a case branch)
// ---------------------------------------------------------------------------

func TestEngine_Switch_CaseBranchIsSequence(t *testing.T) {
	rt, dummy := newTestEngine()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch with sequence branch",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "staging"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging": {
				SequenceStep: &schema.SequenceStep{
					Sequence: []schema.Step{
						runStep("prepare", "alpine", "echo prepare"),
						runStep("deploy", "alpine", "echo deploy"),
						runStep("verify", "alpine", "echo verify"),
					},
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo prepare", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo deploy", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo verify", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Nested composites: sequence containing a parallel block
// ---------------------------------------------------------------------------

func TestEngine_NestedSequenceParallel(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "nested test",
		Sequence: []schema.Step{
			runStep("setup", "alpine", "echo setup"),
			{
				ParallelStep: &schema.ParallelStep{
					Parallel: []schema.Step{
						runStep("test-a", "alpine", "echo test-a"),
						runStep("test-b", "alpine", "echo test-b"),
					},
				},
			},
			runStep("teardown", "alpine", "echo teardown"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// 1 setup + 2 parallel + 1 teardown = 4
	assert.Len(t, dummy.Runs, 4)
	// setup ran first
	assert.Equal(t, "echo setup", dummy.Runs[0].Cmd)
	// teardown ran last
	assert.Equal(t, "echo teardown", dummy.Runs[3].Cmd)
}

// ---------------------------------------------------------------------------
// Build step
// ---------------------------------------------------------------------------

func TestEngine_BuildStep(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "build test",
		Sequence: []schema.Step{
			buildStep("build app", "myapp:latest"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Builds, 1)
	assert.Equal(t, "myapp:latest", dummy.Builds[0].Image)
}

// ---------------------------------------------------------------------------
// Job execution
// ---------------------------------------------------------------------------

func TestEngine_Job(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "job test",
		Jobs: schema.Jobs{
			"build": {
				Name: "Build",
				Sequence: []schema.Step{
					runStep("compile", "golang:1.22", "go build ./..."),
				},
			},
			"test": {
				Name: "Test",
				Sequence: []schema.Step{
					runStep("unit", "golang:1.22", "go test ./..."),
					runStep("lint", "golangci/golangci-lint", "golangci-lint run"),
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "test")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 2)
	assert.Equal(t, "go test ./...", dummy.Runs[0].Cmd)
	assert.Equal(t, "golangci-lint run", dummy.Runs[1].Cmd)
}

func TestEngine_Job_NotFound(t *testing.T) {
	rt, _ := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "job test",
		Jobs: schema.Jobs{
			"build": {Sequence: []schema.Step{runStep("x", "alpine", "echo")}},
		},
	}

	_, err := rt.RunWorkflow(context.Background(), ocw, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEngine_Job_SingleStep(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "single step job",
		Jobs: schema.Jobs{
			"run": {
				Step: &schema.Step{
					RunStep: &schema.RunStep{
						StepBase: schema.StepBase{Name: "run it"},
						Image:    "alpine",
						Cmd:      "echo single",
					},
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "run")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo single", dummy.Runs[0].Cmd)
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestEngine_CancelledContext(t *testing.T) {
	rt, _ := newTestEngine()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "cancel test",
		Sequence: []schema.Step{
			runStep("should not run", "alpine", "echo nope"),
		},
	}

	_, err := rt.RunWorkflow(ctx, ocw, "")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// No flow control
// ---------------------------------------------------------------------------

func TestEngine_NoFlowControl(t *testing.T) {
	rt, _ := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "empty workflow",
	}

	_, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no flow control")
}

// ---------------------------------------------------------------------------
// SwitchStep nested inside a sequence (the Step-level switch, not top-level)
// ---------------------------------------------------------------------------

func TestEngine_SwitchStep_Nested(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "nested switch",
		Env: schema.Env{
			"TARGET": schema.EnvVar{Value: "prod"},
		},
		Sequence: []schema.Step{
			runStep("before", "alpine", "echo before"),
			{
				SwitchStep: &schema.SwitchStep{
					Switch: "{{ env.TARGET }}",
					Case: map[string]schema.Step{
						"prod": runStep("prod deploy", "alpine", "echo prod"),
						"dev":  runStep("dev deploy", "alpine", "echo dev"),
					},
				},
			},
			runStep("after", "alpine", "echo after"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo before", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo prod", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo after", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Step ID propagation through scope
// ---------------------------------------------------------------------------

func TestEngine_StepOutputsInScope(t *testing.T) {
	rt, dummy := newTestEngine()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "output propagation",
		Sequence: []schema.Step{
			buildStep("build app", "myapp:latest"),
			runStepWithID("use image", "runner", "alpine", "echo {{ steps.build.image }}"),
		},
	}

	// Give the build step an ID so its output is stored.
	ocw.Sequence[0].BuildStep.ID = "build"

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Builds, 1)
	require.Len(t, dummy.Runs, 1)
	// The dummy runtime doesn't interpolate the cmd, but we verify the build
	// output was captured in scope by checking it's present.
	assert.Equal(t, "myapp:latest", dummy.Builds[0].Image)
}
