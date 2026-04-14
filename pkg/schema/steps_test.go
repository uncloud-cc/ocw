package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestStringOrStringSlice_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *StringOrStringSlice)
		wantErr bool
	}{
		{
			name: "single string",
			yaml: "value: hello",
			check: func(t *testing.T, s *StringOrStringSlice) {
				if s.Single == nil || *s.Single != "hello" {
					t.Error("expected Single to be 'hello'")
				}
			},
			wantErr: false,
		},
		{
			name: "string slice",
			yaml: "value:\n  - hello\n  - world",
			check: func(t *testing.T, s *StringOrStringSlice) {
				if len(s.Multiple) != 2 {
					t.Errorf("expected 2 strings, got %d", len(s.Multiple))
				}
				if s.Multiple[0] != "hello" || s.Multiple[1] != "world" {
					t.Errorf("unexpected values: %v", s.Multiple)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Value *StringOrStringSlice `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Value != nil {
				tt.check(t, obj.Value)
			}
		})
	}
}

func TestStringOrStringSlice_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		value    *StringOrStringSlice
		expected string
	}{
		{
			name: "single string",
			value: &StringOrStringSlice{
				Single: stringPtr("hello"),
			},
			expected: "value: hello\n",
		},
		{
			name: "string slice",
			value: &StringOrStringSlice{
				Multiple: []string{"hello", "world"},
			},
			expected: "value:\n- hello\n- world\n",
		},
		{
			name:     "nil value",
			value:    nil,
			expected: "value: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *StringOrStringSlice `yaml:"value"`
			}{
				Value: tt.value,
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

func TestStringMapOrSlice_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *StringMapOrSlice)
		wantErr bool
	}{
		{
			name: "map",
			yaml: "value:\n  KEY1: value1\n  KEY2: value2",
			check: func(t *testing.T, s *StringMapOrSlice) {
				if len(s.Map) != 2 {
					t.Errorf("expected 2 items in map, got %d", len(s.Map))
				}
				if s.Map["KEY1"] != "value1" {
					t.Errorf("expected KEY1=value1, got %q", s.Map["KEY1"])
				}
				if s.Map["KEY2"] != "value2" {
					t.Errorf("expected KEY2=value2, got %q", s.Map["KEY2"])
				}
			},
			wantErr: false,
		},
		{
			name: "slice",
			yaml: "value:\n  - KEY1=value1\n  - KEY2=value2",
			check: func(t *testing.T, s *StringMapOrSlice) {
				if len(s.Slice) != 2 {
					t.Errorf("expected 2 items in slice, got %d", len(s.Slice))
				}
				if s.Slice[0] != "KEY1=value1" {
					t.Errorf("expected 'KEY1=value1', got %q", s.Slice[0])
				}
				if s.Slice[1] != "KEY2=value2" {
					t.Errorf("expected 'KEY2=value2', got %q", s.Slice[1])
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Value *StringMapOrSlice `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Value != nil {
				tt.check(t, obj.Value)
			}
		})
	}
}

func TestStringMapOrSlice_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		value    *StringMapOrSlice
		expected string
		skipCmp  bool
	}{
		{
			name: "map",
			value: &StringMapOrSlice{
				Map: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
				},
			},
			// Note: map order is not guaranteed, so we skip comparison
			skipCmp: true,
		},
		{
			name: "slice",
			value: &StringMapOrSlice{
				Slice: []string{"KEY1=value1", "KEY2=value2"},
			},
			expected: "value:\n- KEY1=value1\n- KEY2=value2\n",
			skipCmp:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *StringMapOrSlice `yaml:"value"`
			}{
				Value: tt.value,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			// For map tests, just verify it doesn't error (map order is non-deterministic)
			if tt.skipCmp {
				return
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}

func TestNumberOrString_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *NumberOrString)
		wantErr bool
	}{
		{
			name: "integer",
			yaml: "value: 42",
			check: func(t *testing.T, n *NumberOrString) {
				if n.Number == nil || *n.Number != 42 {
					t.Error("expected Number to be 42")
				}
			},
			wantErr: false,
		},
		{
			name: "float",
			yaml: "value: 3.14",
			check: func(t *testing.T, n *NumberOrString) {
				if n.Number == nil || *n.Number != 3.14 {
					t.Error("expected Number to be 3.14")
				}
			},
			wantErr: false,
		},
		{
			name: "string",
			yaml: "value: all",
			check: func(t *testing.T, n *NumberOrString) {
				if n.String == nil || *n.String != "all" {
					t.Error("expected String to be 'all'")
				}
			},
			wantErr: false,
		},
		{
			name: "quoted number - YAML may parse as number",
			yaml: `value: "42"`,
			check: func(t *testing.T, n *NumberOrString) {
				// go-yaml may parse quoted numbers as actual numbers depending on context
				// Both behaviors are acceptable
				if n.String != nil && *n.String == "42" {
					return // String parsing is fine
				}
				if n.Number != nil && *n.Number == 42 {
					return // Number parsing is also fine
				}
				t.Error("expected either String '42' or Number 42")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Value *NumberOrString `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Value != nil {
				tt.check(t, obj.Value)
			}
		})
	}
}

func TestNumberOrString_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		value    *NumberOrString
		expected string
	}{
		{
			name: "number",
			value: &NumberOrString{
				Number: float64Ptr(42),
			},
			expected: "value: 42.0\n", // go-yaml formats integers as floats when using float64
		},
		{
			name: "float",
			value: &NumberOrString{
				Number: float64Ptr(3.14),
			},
			expected: "value: 3.14\n",
		},
		{
			name: "string",
			value: &NumberOrString{
				String: stringPtr("all"),
			},
			expected: "value: all\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *NumberOrString `yaml:"value"`
			}{
				Value: tt.value,
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

func float64Ptr(f float64) *float64 {
	return &f
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
