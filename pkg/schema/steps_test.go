package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestStep_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *Step)
		wantErr bool
	}{
		{
			name: "single run step",
			yaml: "step:\n  name: test\n  image: nginx",
			check: func(t *testing.T, s *Step) {
				if s.RunStep == nil {
					t.Error("expected RunStep to be set")
					return
				}
				if s.RunStep.Name != "test" {
					t.Errorf("expected name 'test', got %q", s.RunStep.Name)
				}
			},
			wantErr: false,
		},
		{
			name: "parallel step",
			yaml: "step:\n  parallel:\n    - name: test1\n      image: nginx\n    - name: test2\n      image: alpine",
			check: func(t *testing.T, s *Step) {
				if s.ParallelStep == nil {
					t.Error("expected ParallelStep to be set")
					return
				}
				if len(s.ParallelStep.Parallel) != 2 {
					t.Errorf("expected 2 steps, got %d", len(s.ParallelStep.Parallel))
					return
				}
				if s.ParallelStep.Parallel[0].RunStep == nil || s.ParallelStep.Parallel[0].RunStep.Name != "test1" {
					t.Error("expected first step name to be 'test1'")
				}
				if s.ParallelStep.Parallel[1].RunStep == nil || s.ParallelStep.Parallel[1].RunStep.Name != "test2" {
					t.Error("expected second step name to be 'test2'")
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Step *Step `yaml:"step"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Step != nil {
				tt.check(t, obj.Step)
			}
		})
	}
}

func TestStep_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		step     *Step
		expected string
	}{
		{
			name: "run step",
			step: &Step{
				RunStep: &RunStep{
					Image: "alpine",
					Cmd:   "echo hello",
				},
			},
			expected: "value:\n  name: \"\"\n  image: alpine\n  cmd: echo hello\n",
		},
		{
			name: "parallel step",
			step: &Step{
				ParallelStep: &ParallelStep{
					Parallel: []Step{
						{
							RunStep: &RunStep{
								Image: "alpine",
								Cmd:   "echo step1",
							},
						},
						{
							RunStep: &RunStep{
								Image: "alpine",
								Cmd:   "echo step2",
							},
						},
					},
				},
			},
			expected: "value:\n  parallel:\n  - name: \"\"\n    image: alpine\n    cmd: echo step1\n  - name: \"\"\n    image: alpine\n    cmd: echo step2\n",
		},
		{
			name:     "nil value",
			step:     nil,
			expected: "value: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *Step `yaml:"value"`
			}{
				Value: tt.step,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}
