package steps

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// mockRuntime is a test double for ContainerRuntime
type mockRuntime struct {
	runCalled   bool
	buildCalled bool
	runOpts     workflow.RunOptions
	buildOpts   workflow.BuildOptions
	runResult   *workflow.RunResult
	buildResult *workflow.BuildResult
	runErr      error
	buildErr    error
}

func (m *mockRuntime) Run(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error) {
	m.runCalled = true
	m.runOpts = opts
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.runResult, nil
}

func (m *mockRuntime) Build(ctx context.Context, opts workflow.BuildOptions) (*workflow.BuildResult, error) {
	m.buildCalled = true
	m.buildOpts = opts
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return m.buildResult, nil
}

func (m *mockRuntime) Stop(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockRuntime) Logs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return nil, nil
}

func TestRunStep_Interface(t *testing.T) {
	step := &RunStep{
		original: &schema.Step{RunStep: &schema.RunStep{}},
		resolved: &schema.RunStep{
			StepBase: schema.StepBase{Name: "test-run", ID: "run-1"},
			Image:    "alpine:latest",
		},
	}

	assert.Equal(t, "run", step.Type())
	assert.Equal(t, "run-1", step.ID())
	assert.Equal(t, "test-run", step.Name())
	assert.NotNil(t, step.Original)
}

func TestRunStep_Execute(t *testing.T) {
	tests := []struct {
		name         string
		resolved     *schema.RunStep
		runtime      *mockRuntime
		wantOutputs  map[string]string
		wantService  bool
		wantErr      bool
		checkRunOpts func(*testing.T, workflow.RunOptions)
	}{
		{
			name: "basic container run",
			resolved: &schema.RunStep{
				StepBase: schema.StepBase{Name: "basic-run"},
				Image:    "alpine:latest",
				Cmd:      "echo hello",
			},
			runtime: &mockRuntime{
				runResult: &workflow.RunResult{
					ExitCode: 0,
					Outputs:  map[string]string{"output": "hello"},
				},
			},
			wantOutputs: map[string]string{"output": "hello"},
			wantService: false,
			checkRunOpts: func(t *testing.T, opts workflow.RunOptions) {
				assert.Equal(t, "alpine:latest", opts.Image)
				assert.Equal(t, "echo hello", opts.Cmd)
				assert.False(t, opts.Background)
			},
		},
		{
			name: "background service with ports",
			resolved: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "db-service", ID: "postgres"},
				Image:      "postgres:14",
				Background: true,
				Expose: &schema.Expose{
					Ports: []schema.ExposePort{
						{ContainerPort: 5432, HostPort: 5432, Protocol: "tcp"},
					},
				},
			},
			runtime: &mockRuntime{
				runResult: &workflow.RunResult{
					ContainerID: "pg-123",
					ExitCode:    0,
				},
			},
			wantOutputs: nil,
			wantService: true,
			checkRunOpts: func(t *testing.T, opts workflow.RunOptions) {
				assert.True(t, opts.Background)
				assert.Len(t, opts.Expose, 1)
				assert.Equal(t, 5432, opts.Expose[0].ContainerPort)
			},
		},
		{
			name: "container with env vars",
			resolved: &schema.RunStep{
				StepBase: schema.StepBase{
					Name: "env-test",
					Env:  map[string]schema.EnvVar{"VAR1": {Value: "value1"}},
				},
				Image: "busybox",
				RunEnv: &schema.StringMapOrSlice{
					Map: map[string]string{"VAR2": "value2"},
				},
			},
			runtime: &mockRuntime{
				runResult: &workflow.RunResult{ExitCode: 0},
			},
			checkRunOpts: func(t *testing.T, opts workflow.RunOptions) {
				assert.Equal(t, "value1", opts.Env["VAR1"])
				assert.Equal(t, "value2", opts.Env["VAR2"])
			},
		},
		{
			name: "container with healthcheck",
			resolved: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "healthcheck-test"},
				Image:      "nginx",
				Background: true,
				HealthCheck: &schema.HealthCheck{
					Cmd:         "curl -f http://localhost/ || exit 1",
					Interval:    "10s",
					Timeout:     "5s",
					Retries:     3,
					StartPeriod: "30s",
				},
			},
			runtime: &mockRuntime{
				runResult: &workflow.RunResult{ExitCode: 0},
			},
			wantService: true,
			checkRunOpts: func(t *testing.T, opts workflow.RunOptions) {
				require.NotNil(t, opts.HealthCheck)
				assert.Equal(t, "curl -f http://localhost/ || exit 1", opts.HealthCheck.Cmd)
				assert.Equal(t, "10s", opts.HealthCheck.Interval)
				assert.Equal(t, 3, opts.HealthCheck.Retries)
			},
		},
		{
			name: "container with resource limits",
			resolved: &schema.RunStep{
				StepBase: schema.StepBase{Name: "resource-test"},
				Image:    "heavy-app",
				CPUs:     &schema.NumberOrString{String: strPtr("2")},
				Memory:   "4g",
				GPUs:     &schema.NumberOrString{String: strPtr("all")},
			},
			runtime: &mockRuntime{
				runResult: &workflow.RunResult{ExitCode: 0},
			},
			checkRunOpts: func(t *testing.T, opts workflow.RunOptions) {
				assert.Equal(t, "2", opts.CPUs)
				assert.Equal(t, "4g", opts.Memory)
				assert.Equal(t, "all", opts.GPUs)
			},
		},
		{
			name: "runtime error",
			resolved: &schema.RunStep{
				StepBase: schema.StepBase{Name: "failing"},
				Image:    "broken",
			},
			runtime: &mockRuntime{
				runErr: errors.New("image not found"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &RunStep{
				original: &schema.Step{RunStep: tt.resolved},
				resolved: tt.resolved,
			}

			ctx := context.Background()
			stepCtx := &workflow.StepContext{
				Runtime: tt.runtime,
			}

			result, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.wantOutputs != nil {
					assert.Equal(t, tt.wantOutputs, result.Outputs)
				}

				if tt.wantService {
					assert.NotNil(t, result.Service)
				} else {
					assert.Nil(t, result.Service)
				}

				assert.True(t, tt.runtime.runCalled, "Runtime.Run should have been called")
				if tt.checkRunOpts != nil {
					tt.checkRunOpts(t, tt.runtime.runOpts)
				}
			}
		})
	}
}

func TestNewRunStep(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.Step
		ctx     *workflow.StepContext
		wantErr bool
	}{
		{
			name: "valid run step",
			step: &schema.Step{
				RunStep: &schema.RunStep{
					StepBase: schema.StepBase{Name: "test", ID: "step-1"},
					Image:    "alpine",
				},
			},
			ctx:     &workflow.StepContext{},
			wantErr: false,
		},
		{
			name:    "nil run step",
			step:    &schema.Step{RunStep: nil},
			ctx:     &workflow.StepContext{},
			wantErr: true,
		},
		{
			name:    "nil step",
			step:    nil,
			ctx:     &workflow.StepContext{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runStep, err := NewRunStep(tt.step, tt.ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, runStep)
			} else {
				require.NoError(t, err)
				require.NotNil(t, runStep)
				assert.Equal(t, "run", runStep.Type())
			}
		})
	}
}

func TestRunStep_WithVolumes(t *testing.T) {
	resolved := &schema.RunStep{
		StepBase: schema.StepBase{
			Name: "volume-test",
		},
		Image: "app:latest",
	}

	step := &RunStep{
		original: &schema.Step{RunStep: resolved},
		resolved: resolved,
	}

	runtime := &mockRuntime{
		runResult: &workflow.RunResult{ExitCode: 0},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Runtime: runtime,
	}

	_, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)

	// Volumes should be handled if defined (currently empty since test step has no volumes)
	assert.Empty(t, runtime.runOpts.Volumes)
}

func TestRunStep_WithWorkdir(t *testing.T) {
	resolved := &schema.RunStep{
		StepBase: schema.StepBase{Name: "workdir-test"},
		Image:    "busybox",
		Workdir:  "/app/src",
	}

	step := &RunStep{
		original: &schema.Step{RunStep: resolved},
		resolved: resolved,
	}

	runtime := &mockRuntime{
		runResult: &workflow.RunResult{ExitCode: 0},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Runtime: runtime,
	}

	_, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)

	assert.Equal(t, "/app/src", runtime.runOpts.Workdir)
}

func strPtr(s string) *string {
	return &s
}
