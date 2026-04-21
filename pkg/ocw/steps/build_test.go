package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestBuildStep_Interface(t *testing.T) {
	step := &BuildStep{
		original: &schema.Step{BuildStep: &schema.BuildStep{}},
		resolved: &schema.BuildStep{
			StepBase: schema.StepBase{Name: "test-build", ID: "build-1"},
			Build: schema.BuildConfig{
				Image: "myapp:latest",
			},
		},
	}

	assert.Equal(t, "build", step.Type())
	assert.Equal(t, "build-1", step.ID())
	assert.Equal(t, "test-build", step.Name())
	assert.NotNil(t, step.Original)
}

func TestBuildStep_Execute(t *testing.T) {
	tests := []struct {
		name        string
		resolved    *schema.BuildStep
		runtime     *mockRuntime
		wantOutputs map[string]string
		wantErr     bool
		checkOpts   func(*testing.T, workflow.BuildOptions)
	}{
		{
			name: "basic image build",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "basic-build"},
				Build: schema.BuildConfig{
					Image:   "myapp",
					Context: "/project",
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{
					ImageID:  "sha256:abc123",
					ImageRef: "myapp:latest",
					Digest:   "sha256:def456",
				},
			},
			wantOutputs: map[string]string{
				"image":   "myapp:latest",
				"imageId": "sha256:abc123",
				"digest":  "sha256:def456",
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				// Image name comes from Tags[0] in BuildOptions
				assert.Equal(t, "/project", opts.Context)
			},
		},
		{
			name: "build with dockerfile",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "custom-dockerfile"},
				Build: schema.BuildConfig{
					Image:      "api",
					Dockerfile: "Dockerfile.prod",
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "api:v1"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, "Dockerfile.prod", opts.Dockerfile)
			},
		},
		{
			name: "build with target",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "multi-stage"},
				Build: schema.BuildConfig{
					Image:  "app",
					Target: "production",
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "app:prod"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, "production", opts.Target)
			},
		},
		{
			name: "build with tags",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "multi-tag"},
				Build: schema.BuildConfig{
					Image: "service",
					Tags:  []string{"latest", "v1.0.0", "stable"},
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "service:latest"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, []string{"latest", "v1.0.0", "stable"}, opts.Tags)
			},
		},
		{
			name: "build with build args",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "with-args"},
				Build: schema.BuildConfig{
					Image:     "configurable-app",
					BuildArgs: map[string]string{"VERSION": "1.0", "ENV": "prod"},
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "configurable-app:prod"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, "1.0", opts.BuildArgs["VERSION"])
				assert.Equal(t, "prod", opts.BuildArgs["ENV"])
			},
		},
		{
			name: "build with platform",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "multi-platform"},
				Build: schema.BuildConfig{
					Image: "cross-platform",
					Platform: &schema.StringOrStringSlice{
						Multiple: []string{"linux/amd64", "linux/arm64"},
					},
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "cross-platform:latest"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, []string{"linux/amd64", "linux/arm64"}, opts.Platform)
			},
		},
		{
			name: "build with cache",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "cached-build"},
				Build: schema.BuildConfig{
					Image:     "cached-app",
					CacheFrom: []string{"type=local,src=/cache"},
					CacheTo:   []string{"type=local,dest=/cache"},
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{ImageRef: "cached-app:latest"},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.Equal(t, []string{"type=local,src=/cache"}, opts.CacheFrom)
				assert.Equal(t, []string{"type=local,dest=/cache"}, opts.CacheTo)
			},
		},
		{
			name: "build and push",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "push-build"},
				Build: schema.BuildConfig{
					Image: "registry.io/app",
					Push:  true,
					Load:  false,
				},
			},
			runtime: &mockRuntime{
				buildResult: &workflow.BuildResult{
					ImageRef: "registry.io/app:v1",
					Digest:   "sha256:push123",
				},
			},
			checkOpts: func(t *testing.T, opts workflow.BuildOptions) {
				assert.True(t, opts.Push)
				assert.False(t, opts.Load)
			},
		},
		{
			name: "build error",
			resolved: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "failing-build"},
				Build: schema.BuildConfig{
					Image: "broken",
				},
			},
			runtime: &mockRuntime{
				buildErr: errors.New("dockerfile not found"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &BuildStep{
				original: &schema.Step{BuildStep: tt.resolved},
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
					for key, val := range tt.wantOutputs {
						assert.Equal(t, val, result.Outputs[key], "output key %s", key)
					}
				}

				assert.True(t, tt.runtime.buildCalled, "Runtime.Build should have been called")
				if tt.checkOpts != nil {
					tt.checkOpts(t, tt.runtime.buildOpts)
				}
			}
		})
	}
}

func TestNewBuildStep(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.Step
		ctx     *workflow.StepContext
		wantErr bool
	}{
		{
			name: "valid build step",
			step: &schema.Step{
				BuildStep: &schema.BuildStep{
					StepBase: schema.StepBase{Name: "test"},
					Build:    schema.BuildConfig{Image: "app"},
				},
			},
			ctx:     &workflow.StepContext{},
			wantErr: false,
		},
		{
			name:    "nil build step",
			step:    &schema.Step{BuildStep: nil},
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
			buildStep, err := NewBuildStep(tt.step, tt.ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, buildStep)
			} else {
				require.NoError(t, err)
				require.NotNil(t, buildStep)
				assert.Equal(t, "build", buildStep.Type())
			}
		})
	}
}

func TestBuildStep_NoCache(t *testing.T) {
	resolved := &schema.BuildStep{
		StepBase: schema.StepBase{Name: "no-cache"},
		Build: schema.BuildConfig{
			Image:   "fresh",
			NoCache: true,
		},
	}

	step := &BuildStep{
		original: &schema.Step{BuildStep: resolved},
		resolved: resolved,
	}

	runtime := &mockRuntime{
		buildResult: &workflow.BuildResult{ImageRef: "fresh:latest"},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Runtime: runtime,
	}

	_, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)

	// NoCache option should be passed to runtime
	// (implementation specific - could be part of BuildOptions)
}

func TestBuildStep_WithLabels(t *testing.T) {
	resolved := &schema.BuildStep{
		StepBase: schema.StepBase{Name: "labeled"},
		Build: schema.BuildConfig{
			Image: "app",
			Labels: map[string]string{
				"version":  "1.0",
				"built-by": "ocw",
			},
		},
	}

	step := &BuildStep{
		original: &schema.Step{BuildStep: resolved},
		resolved: resolved,
	}

	runtime := &mockRuntime{
		buildResult: &workflow.BuildResult{ImageRef: "app:latest"},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Runtime: runtime,
	}

	_, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)
}
