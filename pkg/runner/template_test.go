package runner

import (
	"os"
	"testing"
)

func TestTemplateContext_Interpolate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		setup    func(*TemplateContext)
		want     string
		wantErr  bool
	}{
		{
			name:     "plain string without templates",
			template: "hello world",
			setup:    func(tc *TemplateContext) {},
			want:     "hello world",
			wantErr:  false,
		},
		{
			name:     "step output reference",
			template: "{{ steps.build.image }}",
			setup: func(tc *TemplateContext) {
				tc.SetStepOutput("build", "image", "myapp:latest")
			},
			want:    "myapp:latest",
			wantErr: false,
		},
		{
			name:     "multiple step outputs in string",
			template: "Built {{ steps.build.image }} with tag {{ steps.build.tag }}",
			setup: func(tc *TemplateContext) {
				tc.SetStepOutput("build", "image", "myapp:v1.0")
				tc.SetStepOutput("build", "tag", "v1.0")
			},
			want:    "Built myapp:v1.0 with tag v1.0",
			wantErr: false,
		},
		{
			name:     "workflow name",
			template: "Running workflow: {{ workflow.name }}",
			setup: func(tc *TemplateContext) {
				tc.Workflow.Name = "MyWorkflow"
			},
			want:    "Running workflow: MyWorkflow",
			wantErr: false,
		},
		{
			name:     "workflow description",
			template: "{{ workflow.description }}",
			setup: func(tc *TemplateContext) {
				tc.Workflow.Description = "Test workflow"
			},
			want:    "Test workflow",
			wantErr: false,
		},
		{
			name:     "workflow id",
			template: "{{ workflow.id }}",
			setup: func(tc *TemplateContext) {
				tc.Workflow.ID = "test-wf"
			},
			want:    "test-wf",
			wantErr: false,
		},
		{
			name:     "job name",
			template: "{{ job.name }}",
			setup: func(tc *TemplateContext) {
				tc.Job.Name = "build-job"
			},
			want:    "build-job",
			wantErr: false,
		},
		{
			name:     "job description",
			template: "{{ job.description }}",
			setup: func(tc *TemplateContext) {
				tc.Job.Description = "Build the application"
			},
			want:    "Build the application",
			wantErr: false,
		},
		{
			name:     "job id",
			template: "{{ job.id }}",
			setup: func(tc *TemplateContext) {
				tc.Job.ID = "build"
			},
			want:    "build",
			wantErr: false,
		},
		{
			name:     "whitespace in template expression",
			template: "{{  steps.build.image  }}",
			setup: func(tc *TemplateContext) {
				tc.SetStepOutput("build", "image", "myapp:latest")
			},
			want:    "myapp:latest",
			wantErr: false,
		},
		{
			name:     "missing step output",
			template: "{{ steps.missing.output }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ steps.missing.output }}",
			wantErr:  true,
		},
		{
			name:     "invalid expression - too few parts",
			template: "{{ invalid }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ invalid }}",
			wantErr:  true,
		},
		{
			name:     "invalid step expression - missing output key",
			template: "{{ steps.build }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ steps.build }}",
			wantErr:  true,
		},
		{
			name:     "unknown workflow field",
			template: "{{ workflow.unknown }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ workflow.unknown }}",
			wantErr:  true,
		},
		{
			name:     "unknown job field",
			template: "{{ job.unknown }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ job.unknown }}",
			wantErr:  true,
		},
		{
			name:     "unknown namespace",
			template: "{{ unknown.field }}",
			setup:    func(tc *TemplateContext) {},
			want:     "{{ unknown.field }}",
			wantErr:  true,
		},
		{
			name:     "mixed valid and invalid templates",
			template: "{{ workflow.name }} - {{ invalid }}",
			setup: func(tc *TemplateContext) {
				tc.Workflow.Name = "MyWorkflow"
			},
			want:    "MyWorkflow - {{ invalid }}",
			wantErr: true,
		},
		{
			name:     "secrets namespace",
			template: "{{ secrets.MY_SECRET }}",
			setup: func(tc *TemplateContext) {
				os.Setenv("MY_SECRET", "secret-value")
			},
			want:    "secret-value",
			wantErr: false,
		},
		{
			name:     "inputs namespace",
			template: "{{ inputs.MY_INPUT }}",
			setup: func(tc *TemplateContext) {
				os.Setenv("MY_INPUT", "input-value")
			},
			want:    "input-value",
			wantErr: false,
		},
		{
			name:     "multiple different namespaces",
			template: "{{ workflow.name }}/{{ steps.build.image }}:{{ job.id }}",
			setup: func(tc *TemplateContext) {
				tc.Workflow.Name = "MyWorkflow"
				tc.Job.ID = "build"
				tc.SetStepOutput("build", "image", "myapp")
			},
			want:    "MyWorkflow/myapp:build",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTemplateContext()
			tt.setup(tc)

			got, err := tc.Interpolate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("Interpolate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Interpolate() = %q, want %q", got, tt.want)
			}

			// Clean up environment variables
			os.Unsetenv("MY_SECRET")
			os.Unsetenv("MY_INPUT")
		})
	}
}

func TestTemplateContext_InterpolateMap(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		setup   func(*TemplateContext)
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "nil map",
			input:   nil,
			setup:   func(tc *TemplateContext) {},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty map",
			input:   map[string]string{},
			setup:   func(tc *TemplateContext) {},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "map with plain strings",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			setup: func(tc *TemplateContext) {},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "map with templates",
			input: map[string]string{
				"IMAGE": "{{ steps.build.image }}",
				"TAG":   "{{ steps.build.tag }}",
			},
			setup: func(tc *TemplateContext) {
				tc.SetStepOutput("build", "image", "myapp")
				tc.SetStepOutput("build", "tag", "v1.0")
			},
			want: map[string]string{
				"IMAGE": "myapp",
				"TAG":   "v1.0",
			},
			wantErr: false,
		},
		{
			name: "map with invalid template",
			input: map[string]string{
				"VALID":   "{{ workflow.name }}",
				"INVALID": "{{ invalid }}",
			},
			setup: func(tc *TemplateContext) {
				tc.Workflow.Name = "MyWorkflow"
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTemplateContext()
			tt.setup(tc)

			got, err := tc.InterpolateMap(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InterpolateMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("InterpolateMap() got %d items, want %d", len(got), len(tt.want))
					return
				}
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("InterpolateMap()[%q] = %q, want %q", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestTemplateContext_InterpolateSlice(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		setup   func(*TemplateContext)
		want    []string
		wantErr bool
	}{
		{
			name:    "nil slice",
			input:   nil,
			setup:   func(tc *TemplateContext) {},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty slice",
			input:   []string{},
			setup:   func(tc *TemplateContext) {},
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "slice with plain strings",
			input:   []string{"value1", "value2", "value3"},
			setup:   func(tc *TemplateContext) {},
			want:    []string{"value1", "value2", "value3"},
			wantErr: false,
		},
		{
			name:  "slice with templates",
			input: []string{"{{ steps.build.image }}", "{{ steps.build.tag }}"},
			setup: func(tc *TemplateContext) {
				tc.SetStepOutput("build", "image", "myapp")
				tc.SetStepOutput("build", "tag", "v1.0")
			},
			want:    []string{"myapp", "v1.0"},
			wantErr: false,
		},
		{
			name:  "slice with invalid template",
			input: []string{"{{ workflow.name }}", "{{ invalid }}"},
			setup: func(tc *TemplateContext) {
				tc.Workflow.Name = "MyWorkflow"
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTemplateContext()
			tt.setup(tc)

			got, err := tc.InterpolateSlice(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InterpolateSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("InterpolateSlice() got %d items, want %d", len(got), len(tt.want))
					return
				}
				for i, v := range tt.want {
					if got[i] != v {
						t.Errorf("InterpolateSlice()[%d] = %q, want %q", i, got[i], v)
					}
				}
			}
		})
	}
}

func TestHasTemplates(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "plain string",
			input: "hello world",
			want:  false,
		},
		{
			name:  "string with template",
			input: "{{ steps.build.image }}",
			want:  true,
		},
		{
			name:  "string with multiple templates",
			input: "{{ workflow.name }} - {{ job.name }}",
			want:  true,
		},
		{
			name:  "string with partial braces",
			input: "{ not a template }",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "template at start",
			input: "{{ workflow.name }} suffix",
			want:  true,
		},
		{
			name:  "template at end",
			input: "prefix {{ workflow.name }}",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasTemplates(tt.input); got != tt.want {
				t.Errorf("HasTemplates(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTemplateContext_GetStepOutput(t *testing.T) {
	tc := NewTemplateContext()
	tc.SetStepOutput("build", "image", "myapp:latest")

	tests := []struct {
		name   string
		stepID string
		key    string
		want   string
		wantOk bool
	}{
		{
			name:   "existing output",
			stepID: "build",
			key:    "image",
			want:   "myapp:latest",
			wantOk: true,
		},
		{
			name:   "non-existent step",
			stepID: "missing",
			key:    "image",
			want:   "",
			wantOk: false,
		},
		{
			name:   "non-existent key",
			stepID: "build",
			key:    "missing",
			want:   "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tc.GetStepOutput(tt.stepID, tt.key)
			if got != tt.want {
				t.Errorf("GetStepOutput() value = %q, want %q", got, tt.want)
			}
			if ok != tt.wantOk {
				t.Errorf("GetStepOutput() ok = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestTemplateContext_SetStepOutput(t *testing.T) {
	tc := NewTemplateContext()

	// Test setting output for new step
	tc.SetStepOutput("build", "image", "myapp:v1")
	if value, ok := tc.GetStepOutput("build", "image"); !ok || value != "myapp:v1" {
		t.Errorf("SetStepOutput() failed to set new output")
	}

	// Test overwriting existing output
	tc.SetStepOutput("build", "image", "myapp:v2")
	if value, ok := tc.GetStepOutput("build", "image"); !ok || value != "myapp:v2" {
		t.Errorf("SetStepOutput() failed to overwrite output")
	}

	// Test setting multiple outputs for same step
	tc.SetStepOutput("build", "tag", "v2")
	if value, ok := tc.GetStepOutput("build", "tag"); !ok || value != "v2" {
		t.Errorf("SetStepOutput() failed to set additional output")
	}

	// Verify first output still exists
	if value, ok := tc.GetStepOutput("build", "image"); !ok || value != "myapp:v2" {
		t.Errorf("SetStepOutput() corrupted existing output")
	}
}
