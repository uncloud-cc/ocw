package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestJob_GetFlowType(t *testing.T) {
	tests := []struct {
		name     string
		job      *Job
		expected string
	}{
		{
			name: "parallel flow",
			job: &Job{
				Parallel: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "parallel",
		},
		{
			name: "sequence flow",
			job: &Job{
				Sequence: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "sequence",
		},
		{
			name: "switch flow",
			job: &Job{
				Switch: stringPtr("{{ env.MODE }}"),
			},
			expected: "switch",
		},
		{
			name: "single step",
			job: &Job{
				Step: &Step{
					RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"},
				},
			},
			expected: "step",
		},
		{
			name:     "no flow",
			job:      &Job{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.job.GetFlowType()
			if result != tt.expected {
				t.Errorf("GetFlowType() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestJob_GetSteps(t *testing.T) {
	tests := []struct {
		name     string
		job      *Job
		expected int
	}{
		{
			name: "parallel steps",
			job: &Job{
				Parallel: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "step1"}, Image: "nginx"}},
					{RunStep: &RunStep{StepBase: StepBase{Name: "step2"}, Image: "alpine"}},
				},
			},
			expected: 2,
		},
		{
			name: "sequence steps",
			job: &Job{
				Sequence: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "step1"}, Image: "nginx"}},
				},
			},
			expected: 1,
		},
		{
			name: "single step",
			job: &Job{
				Step: &Step{
					RunStep: &RunStep{StepBase: StepBase{Name: "step1"}, Image: "nginx"},
				},
			},
			expected: 1,
		},
		{
			name:     "no steps",
			job:      &Job{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := tt.job.GetSteps()
			if len(steps) != tt.expected {
				t.Errorf("GetSteps() returned %d steps; want %d", len(steps), tt.expected)
			}
		})
	}
}

func TestJob_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *Job)
		wantErr bool
	}{
		{
			name: "job with sequence",
			yaml: `name: Build
sequence:
  - name: build
    image: node`,
			check: func(t *testing.T, j *Job) {
				if j.Name != "Build" {
					t.Errorf("expected name 'Build', got %q", j.Name)
				}
				if len(j.Sequence) != 1 {
					t.Errorf("expected 1 sequence step, got %d", len(j.Sequence))
				}
			},
			wantErr: false,
		},
		{
			name: "job with parallel",
			yaml: `name: Test
parallel:
  - name: test1
    image: node
  - name: test2
    image: python`,
			check: func(t *testing.T, j *Job) {
				if len(j.Parallel) != 2 {
					t.Errorf("expected 2 parallel steps, got %d", len(j.Parallel))
				}
			},
			wantErr: false,
		},
		{
			name: "job as single run step",
			yaml: `name: Simple
image: nginx`,
			check: func(t *testing.T, j *Job) {
				if j.Step == nil {
					t.Fatal("expected Step to be set")
				}
				if j.Step.RunStep == nil {
					t.Fatal("expected RunStep to be set")
				}
				if j.Step.RunStep.Image != "nginx" {
					t.Errorf("expected image 'nginx', got %q", j.Step.RunStep.Image)
				}
			},
			wantErr: false,
		},
		{
			name: "job as single build step",
			yaml: `name: Build
build:
  image: myapp:latest`,
			check: func(t *testing.T, j *Job) {
				if j.Step == nil {
					t.Fatal("expected Step to be set")
				}
				if j.Step.BuildStep == nil {
					t.Fatal("expected BuildStep to be set")
				}
				if j.Step.BuildStep.Build.Image != "myapp:latest" {
					t.Errorf("expected image 'myapp:latest', got %q", j.Step.BuildStep.Build.Image)
				}
			},
			wantErr: false,
		},
		{
			name: "job with outputs",
			yaml: `name: Build
sequence:
  - name: build
    image: node
outputs:
  image: "{{ steps.build.image }}"`,
			check: func(t *testing.T, j *Job) {
				if len(j.Outputs) != 1 {
					t.Errorf("expected 1 output, got %d", len(j.Outputs))
				}
				if j.Outputs["image"] != "{{ steps.build.image }}" {
					t.Errorf("unexpected output value: %q", j.Outputs["image"])
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var job Job
			err := yaml.Unmarshal([]byte(tt.yaml), &job)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				tt.check(t, &job)
			}
		})
	}
}
