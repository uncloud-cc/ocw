package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestStepOrSteps_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *StepOrSteps)
		wantErr bool
	}{
		{
			name: "single step",
			yaml: "step:\n  name: test\n  image: nginx",
			check: func(t *testing.T, s *StepOrSteps) {
				if s.Single == nil {
					t.Error("expected Single to be set")
					return
				}
				if s.Single.RunStep == nil {
					t.Error("expected RunStep to be set")
					return
				}
				if s.Single.RunStep.Name != "test" {
					t.Errorf("expected name 'test', got %q", s.Single.RunStep.Name)
				}
			},
			wantErr: false,
		},
		{
			name: "multiple steps",
			yaml: "step:\n  - name: test1\n    image: nginx\n  - name: test2\n    image: alpine",
			check: func(t *testing.T, s *StepOrSteps) {
				if len(s.Multiple) != 2 {
					t.Errorf("expected 2 steps, got %d", len(s.Multiple))
					return
				}
				if s.Multiple[0].RunStep == nil || s.Multiple[0].RunStep.Name != "test1" {
					t.Error("expected first step name to be 'test1'")
				}
				if s.Multiple[1].RunStep == nil || s.Multiple[1].RunStep.Name != "test2" {
					t.Error("expected second step name to be 'test2'")
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Step *StepOrSteps `yaml:"step"`
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

func TestStepOrSteps_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		sos      *StepOrSteps
		expected string
	}{
		{
			name: "single step",
			sos: &StepOrSteps{
				Single: &Step{
					RunStep: &RunStep{
						Image: "alpine",
						Cmd:   "echo hello",
					},
				},
			},
			expected: "value:\n  name: \"\"\n  image: alpine\n  cmd: echo hello\n",
		},
		{
			name: "multiple steps",
			sos: &StepOrSteps{
				Multiple: []Step{
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
			expected: "value:\n- name: \"\"\n  image: alpine\n  cmd: echo step1\n- name: \"\"\n  image: alpine\n  cmd: echo step2\n",
		},
		{
			name:     "nil value",
			sos:      nil,
			expected: "value: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *StepOrSteps `yaml:"value"`
			}{
				Value: tt.sos,
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
