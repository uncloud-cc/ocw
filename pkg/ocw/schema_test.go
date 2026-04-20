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
				Switch: stringPtr("{{ env.MODE }}"),
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

func TestGetJobFlowType(t *testing.T) {
	tests := []struct {
		name     string
		job      *schema.Job
		expected string
	}{
		{
			name: "parallel flow",
			job: &schema.Job{
				Parallel: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "parallel",
		},
		{
			name: "sequence flow",
			job: &schema.Job{
				Sequence: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"}},
				},
			},
			expected: "sequence",
		},
		{
			name: "switch flow",
			job: &schema.Job{
				Switch: stringPtr("{{ env.MODE }}"),
			},
			expected: "switch",
		},
		{
			name: "single step",
			job: &schema.Job{
				Step: &schema.Step{
					RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test"}, Image: "nginx"},
				},
			},
			expected: "step",
		},
		{
			name:     "no flow",
			job:      &schema.Job{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetJobFlowType(tt.job)
			if result != tt.expected {
				t.Errorf("GetJobFlowType() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestGetJobSteps(t *testing.T) {
	tests := []struct {
		name     string
		job      *schema.Job
		expected int
	}{
		{
			name: "parallel steps",
			job: &schema.Job{
				Parallel: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "nginx"}},
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step2"}, Image: "alpine"}},
				},
			},
			expected: 2,
		},
		{
			name: "sequence steps",
			job: &schema.Job{
				Sequence: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "nginx"}},
				},
			},
			expected: 1,
		},
		{
			name: "single step",
			job: &schema.Job{
				Step: &schema.Step{
					RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "nginx"},
				},
			},
			expected: 1,
		},
		{
			name:     "no steps",
			job:      &schema.Job{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := GetJobSteps(tt.job)
			if len(steps) != tt.expected {
				t.Errorf("GetJobSteps() returned %d steps; want %d", len(steps), tt.expected)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func TestWatchIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected bool
	}{
		{
			name:     "nil watch",
			watch:    nil,
			expected: false,
		},
		{
			name: "bool true",
			watch: &schema.Watch{
				Bool: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "bool false",
			watch: &schema.Watch{
				Bool: boolPtr(false),
			},
			expected: false,
		},
		{
			name: "string glob",
			watch: &schema.Watch{
				String: stringPtr("src/**/*.go"),
			},
			expected: true,
		},
		{
			name: "strings array",
			watch: &schema.Watch{
				Strings: []string{"src/**/*.go"},
			},
			expected: true,
		},
		{
			name: "config object",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					Files: []string{"src/**/*.go"},
				},
			},
			expected: true,
		},
		{
			name:     "empty watch",
			watch:    &schema.Watch{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchIsEnabled(tt.watch)
			if result != tt.expected {
				t.Errorf("WatchIsEnabled() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestWatchGetFiles(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected []string
	}{
		{
			name:     "nil watch",
			watch:    nil,
			expected: nil,
		},
		{
			name: "bool true - no files",
			watch: &schema.Watch{
				Bool: boolPtr(true),
			},
			expected: nil,
		},
		{
			name: "single string",
			watch: &schema.Watch{
				String: stringPtr("src/**/*.go"),
			},
			expected: []string{"src/**/*.go"},
		},
		{
			name: "strings array",
			watch: &schema.Watch{
				Strings: []string{"src/**/*.go", "pkg/**/*.go"},
			},
			expected: []string{"src/**/*.go", "pkg/**/*.go"},
		},
		{
			name: "config with files",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					Files: []string{"src/**/*.go", "pkg/**/*.go"},
				},
			},
			expected: []string{"src/**/*.go", "pkg/**/*.go"},
		},
		{
			name: "config without files",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchGetFiles(tt.watch)
			if len(result) != len(tt.expected) {
				t.Errorf("WatchGetFiles() returned %d files; want %d", len(result), len(tt.expected))
				return
			}
			for i, file := range tt.expected {
				if result[i] != file {
					t.Errorf("WatchGetFiles()[%d] = %q; want %q", i, result[i], file)
				}
			}
		})
	}
}

func TestWatchGetMode(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected schema.WatchMode
	}{
		{
			name:     "nil watch - default",
			watch:    nil,
			expected: schema.WatchModeRebuildReload,
		},
		{
			name: "bool true - default",
			watch: &schema.Watch{
				Bool: boolPtr(true),
			},
			expected: schema.WatchModeRebuildReload,
		},
		{
			name: "config with rebuild-reload mode",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					Mode: schema.WatchModeRebuildReload,
				},
			},
			expected: schema.WatchModeRebuildReload,
		},
		{
			name: "config with reload mode",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					Mode: schema.WatchModeReload,
				},
			},
			expected: schema.WatchModeReload,
		},
		{
			name: "config without mode - default",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{},
			},
			expected: schema.WatchModeRebuildReload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchGetMode(tt.watch)
			if result != tt.expected {
				t.Errorf("WatchGetMode() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestWatchShouldUseGitIgnore(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected bool
	}{
		{
			name:     "nil watch - default true",
			watch:    nil,
			expected: true,
		},
		{
			name: "bool true - default true",
			watch: &schema.Watch{
				Bool: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "config with true",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					UseGitIgnore: boolPtr(true),
				},
			},
			expected: true,
		},
		{
			name: "config with false",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					UseGitIgnore: boolPtr(false),
				},
			},
			expected: false,
		},
		{
			name: "config without setting - default true",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchShouldUseGitIgnore(tt.watch)
			if result != tt.expected {
				t.Errorf("WatchShouldUseGitIgnore() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestWatchShouldUseDockerIgnore(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected bool
	}{
		{
			name:     "nil watch - default true",
			watch:    nil,
			expected: true,
		},
		{
			name: "config with true",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					UseDockerIgnore: boolPtr(true),
				},
			},
			expected: true,
		},
		{
			name: "config with false",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					UseDockerIgnore: boolPtr(false),
				},
			},
			expected: false,
		},
		{
			name: "config without setting - default true",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchShouldUseDockerIgnore(tt.watch)
			if result != tt.expected {
				t.Errorf("WatchShouldUseDockerIgnore() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestWatchGetIgnorePatterns(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected []string
	}{
		{
			name:     "nil watch",
			watch:    nil,
			expected: nil,
		},
		{
			name: "bool watch - no patterns",
			watch: &schema.Watch{
				Bool: boolPtr(true),
			},
			expected: nil,
		},
		{
			name: "config with ignore patterns",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{
					Ignore: []string{"**/*_test.go", "*.tmp"},
				},
			},
			expected: []string{"**/*_test.go", "*.tmp"},
		},
		{
			name: "config without ignore patterns",
			watch: &schema.Watch{
				Config: &schema.WatchConfig{},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WatchGetIgnorePatterns(tt.watch)
			if len(result) != len(tt.expected) {
				t.Errorf("WatchGetIgnorePatterns() returned %d patterns; want %d", len(result), len(tt.expected))
				return
			}
			for i, pattern := range tt.expected {
				if result[i] != pattern {
					t.Errorf("WatchGetIgnorePatterns()[%d] = %q; want %q", i, result[i], pattern)
				}
			}
		})
	}
}
