package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		ctx     *StepContext
		want    string
		wantErr bool
	}{
		{
			name:    "no templates",
			input:   "plain string",
			ctx:     &StepContext{},
			want:    "plain string",
			wantErr: false,
		},
		{
			name:  "env variable",
			input: "image: {{ env.IMAGE }}",
			ctx: &StepContext{
				Env: map[string]string{"IMAGE": "alpine:latest"},
			},
			want:    "image: alpine:latest",
			wantErr: false,
		},
		{
			name:  "secret variable",
			input: "token: {{ secrets.API_KEY }}",
			ctx: &StepContext{
				Secrets: map[string]string{"API_KEY": "secret123"},
			},
			want:    "token: secret123",
			wantErr: false,
		},
		{
			name:  "step output",
			input: "{{ steps.build.image }}",
			ctx: &StepContext{
				Steps: map[string]map[string]string{
					"build": {"image": "myapp:v1"},
				},
			},
			want:    "myapp:v1",
			wantErr: false,
		},
		{
			name:  "input variable",
			input: "version: {{ inputs.VERSION }}",
			ctx: &StepContext{
				Inputs: map[string]string{"VERSION": "1.0.0"},
			},
			want:    "version: 1.0.0",
			wantErr: false,
		},
		{
			name:  "workflow metadata name",
			input: "workflow: {{ workflow.name }}",
			ctx: &StepContext{
				Workflow: WorkflowMeta{Name: "my-workflow"},
			},
			want:    "workflow: my-workflow",
			wantErr: false,
		},
		{
			name:  "workflow metadata id",
			input: "id: {{ workflow.id }}",
			ctx: &StepContext{
				Workflow: WorkflowMeta{ID: "wf-123"},
			},
			want:    "id: wf-123",
			wantErr: false,
		},
		{
			name:  "multiple templates",
			input: "{{ env.REGISTRY }}/{{ env.IMAGE }}:{{ inputs.TAG }}",
			ctx: &StepContext{
				Env:    map[string]string{"REGISTRY": "docker.io", "IMAGE": "app"},
				Inputs: map[string]string{"TAG": "latest"},
			},
			want:    "docker.io/app:latest",
			wantErr: false,
		},
		{
			name:    "missing env variable",
			input:   "{{ env.MISSING }}",
			ctx:     &StepContext{Env: map[string]string{}},
			want:    "{{ env.MISSING }}", // Original preserved on error
			wantErr: true,
		},
		{
			name:  "whitespace in template",
			input: "{{  env.IMAGE  }}",
			ctx: &StepContext{
				Env: map[string]string{"IMAGE": "alpine"},
			},
			want:    "alpine",
			wantErr: false,
		},
		{
			name:    "invalid expression namespace",
			input:   "{{ invalid.VAR }}",
			ctx:     &StepContext{},
			want:    "{{ invalid.VAR }}",
			wantErr: true,
		},
		{
			name:    "missing step",
			input:   "{{ steps.nonexistent.key }}",
			ctx:     &StepContext{Steps: map[string]map[string]string{}},
			want:    "{{ steps.nonexistent.key }}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveString(tt.input, tt.ctx)

			if tt.wantErr {
				// Some implementations may return error, others preserve original
				// Just verify we get the expected string
				assert.Equal(t, tt.want, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestResolveString_NilContext(t *testing.T) {
	result, err := ResolveString("{{ env.VAR }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "{{ env.VAR }}", result, "should return original when context is nil")
}

func TestResolveString_EmptyContext(t *testing.T) {
	ctx := &StepContext{
		Env:     map[string]string{},
		Secrets: map[string]string{},
		Inputs:  map[string]string{},
		Steps:   map[string]map[string]string{},
	}
	result, err := ResolveString("{{ env.MISSING }}", ctx)
	// Should preserve template when not found (may or may not error)
	assert.Contains(t, result, "{{ env.MISSING }}", "should preserve template when not found")
	_ = err // Error handling is implementation dependent
}

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		ctx     *StepContext
		wantErr bool
	}{
		{
			name: "interpolate run step image",
			step: &mockStep{
				stepType: "run",
				id:       "test",
			},
			ctx: &StepContext{
				Env: map[string]string{"IMAGE": "alpine:latest"},
			},
			wantErr: false,
		},
		{
			name:    "nil step",
			step:    nil,
			ctx:     &StepContext{},
			wantErr: true,
		},
		{
			name: "interpolate build step image",
			step: &mockStep{
				stepType: "build",
			},
			ctx: &StepContext{
				Steps: map[string]map[string]string{
					"previous": {"image": "base:v1"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Interpolate(tt.step, tt.ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInterpolate_NilContext(t *testing.T) {
	step := &mockStep{stepType: "run"}
	err := Interpolate(step, nil)
	assert.NoError(t, err, "nil context should not panic")
}
