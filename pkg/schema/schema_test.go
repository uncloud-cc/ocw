package schema

import (
	"testing"
)

func TestOCW_GetFlowType(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *OCW
		expected string
	}{
		{
			name: "parallel flow",
			ocw: &OCW{
				Parallel: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "parallel",
		},
		{
			name: "sequence flow",
			ocw: &OCW{
				Sequence: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "sequence",
		},
		{
			name: "switch flow",
			ocw: &OCW{
				Switch: stringPtr("{{ env.MODE }}"),
			},
			expected: "switch",
		},
		{
			name:     "no flow",
			ocw:      &OCW{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ocw.GetFlowType()
			if result != tt.expected {
				t.Errorf("GetFlowType() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestOCW_HasDirectFlow(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *OCW
		expected bool
	}{
		{
			name: "has parallel flow",
			ocw: &OCW{
				Parallel: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: true,
		},
		{
			name:     "no flow",
			ocw:      &OCW{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ocw.HasDirectFlow()
			if result != tt.expected {
				t.Errorf("HasDirectFlow() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestOCW_HasJobs(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *OCW
		expected bool
	}{
		{
			name: "has jobs",
			ocw: &OCW{
				Jobs: Jobs{
					"build": Job{
						Name: "Build",
					},
				},
			},
			expected: true,
		},
		{
			name:     "no jobs",
			ocw:      &OCW{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ocw.HasJobs()
			if result != tt.expected {
				t.Errorf("HasJobs() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestOCW_GetJob(t *testing.T) {
	ocw := &OCW{
		Jobs: Jobs{
			"build": Job{
				Name: "Build Job",
			},
			"test": Job{
				Name: "Test Job",
			},
		},
	}

	tests := []struct {
		name    string
		jobName string
		wantNil bool
		check   func(*testing.T, *Job)
	}{
		{
			name:    "existing job",
			jobName: "build",
			wantNil: false,
			check: func(t *testing.T, j *Job) {
				if j.Name != "Build Job" {
					t.Errorf("expected job name 'Build Job', got %q", j.Name)
				}
			},
		},
		{
			name:    "another existing job",
			jobName: "test",
			wantNil: false,
			check: func(t *testing.T, j *Job) {
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
			job := ocw.GetJob(tt.jobName)
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

func TestOCW_GetJobNames(t *testing.T) {
	ocw := &OCW{
		Jobs: Jobs{
			"build":  Job{Name: "Build"},
			"test":   Job{Name: "Test"},
			"deploy": Job{Name: "Deploy"},
		},
	}

	names := ocw.GetJobNames()
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

func TestOCW_GetSteps(t *testing.T) {
	tests := []struct {
		name     string
		ocw      *OCW
		expected int
	}{
		{
			name: "parallel steps",
			ocw: &OCW{
				Parallel: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "step1"}, Image: "nginx"}},
					{RunStep: &RunStep{StepBase: StepBase{Name: "step2"}, Image: "alpine"}},
				},
			},
			expected: 2,
		},
		{
			name: "sequence steps",
			ocw: &OCW{
				Sequence: []Step{
					{RunStep: &RunStep{StepBase: StepBase{Name: "step1"}, Image: "nginx"}},
				},
			},
			expected: 1,
		},
		{
			name:     "no steps",
			ocw:      &OCW{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := tt.ocw.GetSteps()
			if len(steps) != tt.expected {
				t.Errorf("GetSteps() returned %d steps; want %d", len(steps), tt.expected)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *OCW)
		wantErr bool
	}{
		{
			name: "valid minimal workflow",
			yaml: `schemaVersion: "0.1.0"
name: Test Workflow
sequence:
  - name: test
    image: nginx`,
			check: func(t *testing.T, ocw *OCW) {
				if ocw.Name != "Test Workflow" {
					t.Errorf("expected name 'Test Workflow', got %q", ocw.Name)
				}
				if len(ocw.Sequence) != 1 {
					t.Errorf("expected 1 sequence step, got %d", len(ocw.Sequence))
				}
			},
			wantErr: false,
		},
		{
			name: "workflow with jobs",
			yaml: `schemaVersion: "0.1.0"
name: Test Workflow
jobs:
  build:
    name: Build Job
    sequence:
      - name: build
        image: node`,
			check: func(t *testing.T, ocw *OCW) {
				if len(ocw.Jobs) != 1 {
					t.Errorf("expected 1 job, got %d", len(ocw.Jobs))
				}
				job, ok := ocw.Jobs["build"]
				if !ok {
					t.Fatal("expected 'build' job to exist")
				}
				if job.Name != "Build Job" {
					t.Errorf("expected job name 'Build Job', got %q", job.Name)
				}
			},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			yaml:    `invalid: yaml: syntax`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ocw != nil {
				tt.check(t, ocw)
			}
		})
	}
}

func TestOCW_Marshal(t *testing.T) {
	ocw := &OCW{
		SchemaVersion: "0.1.0",
		Name:          "Test",
		Sequence: []Step{
			{RunStep: &RunStep{StepBase: StepBase{Name: "test"}, Image: "nginx"}},
		},
	}

	data, err := ocw.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Verify we can parse it back
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() after Marshal() error = %v", err)
	}

	if parsed.Name != ocw.Name {
		t.Errorf("Round-trip failed: name = %q; want %q", parsed.Name, ocw.Name)
	}
}
