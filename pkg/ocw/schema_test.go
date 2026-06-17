package ocw

import (
	"testing"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestGetFlowType(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *schema.OCW
		expected string
	}{
		{
			name: "parallel flow",
			ocw: &schema.OCW{
				Parallel: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "parallel",
		},
		{
			name: "sequence flow",
			ocw: &schema.OCW{
				Sequence: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "sequence",
		},
		{
			name: "switch flow",
			ocw: &schema.OCW{
				Switch: "{{ env.MODE }}",
			},
			expected: "switch",
		},
		{
			name:     "no flow",
			ocw:      &schema.OCW{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFlowType(tt.ocw)
			if result != tt.expected {
				t.Errorf("GetFlowType() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestHasDirectFlow(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *schema.OCW
		expected bool
	}{
		{
			name: "has parallel flow",
			ocw: &schema.OCW{
				Parallel: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: true,
		},
		{
			name:     "no flow",
			ocw:      &schema.OCW{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasDirectFlow(tt.ocw)
			if result != tt.expected {
				t.Errorf("HasDirectFlow() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestHasJobs(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *schema.OCW
		expected bool
	}{
		{
			name: "has jobs",
			ocw: &schema.OCW{
				Jobs: schema.Jobs{
					"build": schema.Job{
						Name: "Build",
					},
				},
			},
			expected: true,
		},
		{
			name:     "no jobs",
			ocw:      &schema.OCW{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasJobs(tt.ocw)
			if result != tt.expected {
				t.Errorf("HasJobs() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestGetJob(t *testing.T) {
	ocw := &schema.OCW{
		Jobs: schema.Jobs{
			"build": schema.Job{
				Name: "Build Job",
			},
			"test": schema.Job{
				Name: "Test Job",
			},
		},
	}

	tests := []struct {
		name    string
		jobName string
		wantNil bool
		check   func(*testing.T, *schema.Job)
	}{
		{
			name:    "existing job",
			jobName: "build",
			wantNil: false,
			check: func(t *testing.T, j *schema.Job) {
				if j.Name != "Build Job" {
					t.Errorf("expected job name 'Build Job', got %q", j.Name)
				}
			},
		},
		{
			name:    "another existing job",
			jobName: "test",
			wantNil: false,
			check: func(t *testing.T, j *schema.Job) {
				if j.Name != "Test Job" {
					t.Errorf("expected job name 'Test Job', got %q", j.Name)
				}
			},
		},
		{
			name:    "non-existent job",
			jobName: "deploy",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := GetJob(ocw, tt.jobName)
			if (job == nil) != tt.wantNil {
				t.Errorf("GetJob() returned nil=%v; want nil=%v", job == nil, tt.wantNil)
				return
			}
			if !tt.wantNil {
				tt.check(t, job)
			}
		})
	}
}

func TestGetJobNames(t *testing.T) {
	ocw := &schema.OCW{
		Jobs: schema.Jobs{
			"build":  schema.Job{Name: "Build"},
			"test":   schema.Job{Name: "Test"},
			"deploy": schema.Job{Name: "Deploy"},
		},
	}

	names := GetJobNames(ocw)
	if len(names) != 3 {
		t.Errorf("GetJobNames() returned %d names; want 3", len(names))
	}

	// Check that all job names are present (order doesn't matter for maps)
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range []string{"build", "test", "deploy"} {
		if !nameMap[expected] {
			t.Errorf("GetJobNames() missing %q", expected)
		}
	}
}

func TestGetSteps(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *schema.OCW
		expected int
	}{
		{
			name: "parallel steps",
			ocw: &schema.OCW{
				Parallel: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "nginx"}},
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step2"}, Image: "alpine"}},
				},
			},
			expected: 2,
		},
		{
			name: "sequence steps",
			ocw: &schema.OCW{
				Sequence: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "nginx"}},
				},
			},
			expected: 1,
		},
		{
			name:     "no steps",
			ocw:      &schema.OCW{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := GetSteps(tt.ocw)
			if len(steps) != tt.expected {
				t.Errorf("GetSteps() returned %d steps; want %d", len(steps), tt.expected)
			}
		})
	}
}
