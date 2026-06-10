package ocw

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestInterpolateTemplate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		state       *State
		expected    string
		expectedErr error
	}{
		{
			name:  "All namespaces work",
			input: "{{ meta.job }}-{{ inputs.SOMETHING }}-{{ steps.build.id }}",
			state: &State{
				Meta:    map[string]string{"job": "<jobName>"},
				Inputs:  map[string]string{"SOMETHING": "<env>"},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{"build": {"id": "<stepOutput>"}},
			},
			expected:    "<jobName>-<env>-<stepOutput>",
			expectedErr: nil,
		},
		{
			name:  "String without templates returns the same string without complaints",
			input: "Look ma, no templates!",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "Look ma, no templates!",
			expectedErr: nil,
		},
		{
			name:  "Returns error when it can't find meta key",
			input: "{{ meta.doesNotExist }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'doesNotExist' in meta namespace"),
		},
		{
			name:  "Returns error when it can't find inputs key",
			input: "{{ inputs.doesNotExist }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'doesNotExist' in inputs"),
		},
		{
			name:  "Returns error when there's not enough parts in referencing step output",
			input: "{{ steps.notEnoughParts }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Step output references in templates need three parts: {{ steps.<stepId>.<key> }} (e.g. {{ steps.build.image }})"),
		},
		{
			name:  "Returns error when it can't find specified step in outputs",
			input: "{{ steps.doesNotExist.output }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not any outputs for step 'doesNotExist'"),
		},
		{
			name:  "Returns error when it can't find key in step outputs",
			input: "{{ steps.mystep.outputValue }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{"mystep": {}},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'outputValue' in step outputs for step 'mystep'"),
		},
		{
			name:  "Secrets namespace works",
			input: "{{ secrets.API_KEY }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{"API_KEY": "secret123"},
				Steps:   map[string]map[string]string{},
			},
			expected:    "secret123",
			expectedErr: nil,
		},
		{
			name:  "Returns error when it can't find secrets key",
			input: "{{ secrets.doesNotExist }}",
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'doesNotExist' in secrets"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.state.InterpolateTemplate(tt.input)
			if result != tt.expected {
				t.Errorf("InterpolateTemplate() > %s = %q; expected %q", tt.name, result, tt.expected)
			}

			if !errorsEqual(err, tt.expectedErr) {
				t.Errorf("InterpolateTemplate() > %s error = %v; expected %v", tt.name, err, tt.expectedErr)
			}
		})
	}
}

func TestState_SetStepOutput(t *testing.T) {
	tests := []struct {
		name         string
		initial      *State
		stepID       string
		key          string
		value        string
		expectedStep map[string]string
	}{
		{
			name: "creates new step bucket",
			initial: &State{
				Steps: make(map[string]map[string]string),
			},
			stepID:       "build",
			key:          "image",
			value:        "my-image",
			expectedStep: map[string]string{"image": "my-image"},
		},
		{
			name: "adds to existing step bucket",
			initial: &State{
				Steps: map[string]map[string]string{
					"build": {"image": "old-image"},
				},
			},
			stepID:       "build",
			key:          "tag",
			value:        "latest",
			expectedStep: map[string]string{"image": "old-image", "tag": "latest"},
		},
		{
			name: "overwrites existing key",
			initial: &State{
				Steps: map[string]map[string]string{
					"build": {"image": "old-image"},
				},
			},
			stepID:       "build",
			key:          "image",
			value:        "new-image",
			expectedStep: map[string]string{"image": "new-image"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			state.SetStepOutput(tt.stepID, tt.key, tt.value)
			if !reflect.DeepEqual(state.Steps[tt.stepID], tt.expectedStep) {
				t.Errorf("Steps[%q] = %v; expected %v", tt.stepID, state.Steps[tt.stepID], tt.expectedStep)
			}
		})
	}
}

func TestState_SetStepOutputs(t *testing.T) {
	tests := []struct {
		name          string
		initial       *State
		stepID        string
		outputs       map[string]string
		expectedSteps map[string]map[string]string
	}{
		{
			name: "writes multiple outputs for new step",
			initial: &State{
				Steps: make(map[string]map[string]string),
			},
			stepID: "deploy",
			outputs: map[string]string{
				"url":   "https://example.com",
				"image": "app:v1",
			},
			expectedSteps: map[string]map[string]string{
				"deploy": {"url": "https://example.com", "image": "app:v1"},
			},
		},
		{
			name: "merges with existing step outputs",
			initial: &State{
				Steps: map[string]map[string]string{
					"deploy": {"url": "https://old.com"},
				},
			},
			stepID: "deploy",
			outputs: map[string]string{
				"image": "app:v2",
			},
			expectedSteps: map[string]map[string]string{
				"deploy": {"url": "https://old.com", "image": "app:v2"},
			},
		},
		{
			name: "overwrites existing keys",
			initial: &State{
				Steps: map[string]map[string]string{
					"deploy": {"url": "https://old.com"},
				},
			},
			stepID: "deploy",
			outputs: map[string]string{
				"url": "https://new.com",
			},
			expectedSteps: map[string]map[string]string{
				"deploy": {"url": "https://new.com"},
			},
		},
		{
			name: "does not affect other steps",
			initial: &State{
				Steps: map[string]map[string]string{
					"build": {"image": "builder"},
				},
			},
			stepID: "deploy",
			outputs: map[string]string{
				"url": "https://example.com",
			},
			expectedSteps: map[string]map[string]string{
				"build":  {"image": "builder"},
				"deploy": {"url": "https://example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			state.SetStepOutputs(tt.stepID, tt.outputs)
			if !reflect.DeepEqual(state.Steps, tt.expectedSteps) {
				t.Errorf("Steps = %v; expected %v", state.Steps, tt.expectedSteps)
			}
		})
	}
}

func TestState_Clone(t *testing.T) {
	tests := []struct {
		name        string
		original    *State
		mutateClone bool
		mutateOrig  bool
	}{
		{
			name: "clone is independent of original",
			original: &State{
				Meta:    map[string]string{"name": "workflow"},
				Inputs:  map[string]string{"KEY": "val"},
				Secrets: map[string]string{"API_KEY": "secret"},
				Steps:   map[string]map[string]string{"build": {"image": "img"}},
			},
			mutateClone: true,
			mutateOrig:  false,
		},
		{
			name: "original mutation does not affect clone",
			original: &State{
				Meta:    map[string]string{"name": "workflow"},
				Inputs:  map[string]string{"KEY": "val"},
				Secrets: map[string]string{"API_KEY": "secret"},
				Steps:   map[string]map[string]string{"build": {"image": "img"}},
			},
			mutateClone: false,
			mutateOrig:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := tt.original.Clone()

			// Verify initial deep equality
			if !reflect.DeepEqual(clone.Meta, tt.original.Meta) {
				t.Errorf("clone.Meta != original.Meta initially")
			}
			if !reflect.DeepEqual(clone.Inputs, tt.original.Inputs) {
				t.Errorf("clone.Inputs != original.Inputs initially")
			}
			if !reflect.DeepEqual(clone.Secrets, tt.original.Secrets) {
				t.Errorf("clone.Secrets != original.Secrets initially")
			}
			if !reflect.DeepEqual(clone.Steps, tt.original.Steps) {
				t.Errorf("clone.Steps != original.Steps initially")
			}

			if tt.mutateClone {
				clone.Meta["name"] = "mutated"
				clone.Inputs["KEY"] = "mutated"
				clone.Secrets["API_KEY"] = "mutated"
				clone.Steps["build"]["image"] = "mutated"
				clone.Steps["new"] = map[string]string{"x": "y"}

				if reflect.DeepEqual(clone.Meta, tt.original.Meta) {
					t.Errorf("clone.Meta mutation leaked to original")
				}
				if reflect.DeepEqual(clone.Inputs, tt.original.Inputs) {
					t.Errorf("clone.Inputs mutation leaked to original")
				}
				if reflect.DeepEqual(clone.Secrets, tt.original.Secrets) {
					t.Errorf("clone.Secrets mutation leaked to original")
				}
				if reflect.DeepEqual(clone.Steps, tt.original.Steps) {
					t.Errorf("clone.Steps mutation leaked to original")
				}
			}

			if tt.mutateOrig {
				tt.original.Meta["name"] = "mutated"
				tt.original.Inputs["KEY"] = "mutated"
				tt.original.Secrets["API_KEY"] = "mutated"
				tt.original.Steps["build"]["image"] = "mutated"
				tt.original.Steps["new"] = map[string]string{"x": "y"}

				if reflect.DeepEqual(clone.Meta, tt.original.Meta) {
					t.Errorf("original.Meta mutation leaked to clone")
				}
				if reflect.DeepEqual(clone.Inputs, tt.original.Inputs) {
					t.Errorf("original.Inputs mutation leaked to clone")
				}
				if reflect.DeepEqual(clone.Secrets, tt.original.Secrets) {
					t.Errorf("original.Secrets mutation leaked to clone")
				}
				if reflect.DeepEqual(clone.Steps, tt.original.Steps) {
					t.Errorf("original.Steps mutation leaked to clone")
				}
			}
		})
	}
}

func TestNewState(t *testing.T) {
	t.Run("nil inputs returns empty state", func(t *testing.T) {
		state, err := NewState(nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(state.Inputs) != 0 || len(state.Secrets) != 0 || len(state.Steps) != 0 {
			t.Errorf("expected empty state, got Inputs=%v Secrets=%v Steps=%v", state.Inputs, state.Secrets, state.Steps)
		}
	})

	t.Run("empty inputs map returns empty state", func(t *testing.T) {
		empty := schema.Inputs{}
		state, err := NewState(&empty, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(state.Inputs) != 0 || len(state.Secrets) != 0 {
			t.Errorf("expected empty state, got Inputs=%v Secrets=%v", state.Inputs, state.Secrets)
		}
	})

	t.Run("uses schema default when no env or file", func(t *testing.T) {
		inputs := schema.Inputs{
			"DB_PORT": {Value: "8080", IsSecret: false},
			"DB_USER": {Value: "admin", IsSecret: false},
		}
		state, err := NewState(&inputs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Inputs["DB_PORT"] != "8080" {
			t.Errorf("expected DB_PORT=8080, got %q", state.Inputs["DB_PORT"])
		}
		if state.Inputs["DB_USER"] != "admin" {
			t.Errorf("expected DB_USER=admin, got %q", state.Inputs["DB_USER"])
		}
		if state.Env["DB_PORT"] != "8080" {
			t.Errorf("expected env DB_PORT=8080, got %q", state.Env["DB_PORT"])
		}
	})

	t.Run("env var overrides schema default", func(t *testing.T) {
		inputs := schema.Inputs{
			"DB_PORT": {Value: "8080", IsSecret: false},
		}
		t.Setenv("DB_PORT", "5432")
		state, err := NewState(&inputs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Inputs["DB_PORT"] != "5432" {
			t.Errorf("expected DB_PORT=5432, got %q", state.Inputs["DB_PORT"])
		}
		if state.Env["DB_PORT"] != "5432" {
			t.Errorf("expected env DB_PORT=5432, got %q", state.Env["DB_PORT"])
		}
	})

	t.Run("input file overrides env var", func(t *testing.T) {
		inputs := schema.Inputs{
			"DB_PORT": {Value: "8080", IsSecret: false},
		}
		t.Setenv("DB_PORT", "5432")

		tmpDir := t.TempDir()
		inputFile := filepath.Join(tmpDir, "inputs.json")
		if err := os.WriteFile(inputFile, []byte(`{"DB_PORT":"3306"}`), 0644); err != nil {
			t.Fatalf("write input file: %v", err)
		}

		state, err := NewState(&inputs, inputFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Inputs["DB_PORT"] != "3306" {
			t.Errorf("expected DB_PORT=3306, got %q", state.Inputs["DB_PORT"])
		}
		if state.Env["DB_PORT"] != "3306" {
			t.Errorf("expected env DB_PORT=3306, got %q", state.Env["DB_PORT"])
		}
	})

	t.Run("missing required input returns error", func(t *testing.T) {
		inputs := schema.Inputs{
			"API_KEY": {Value: "", IsSecret: true},
		}
		_, err := NewState(&inputs, "")
		if err == nil {
			t.Fatal("expected error for missing required input, got nil")
		}
		expected := `required input "API_KEY" is not set`
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected error to contain %q, got %q", expected, err.Error())
		}
	})

	t.Run("secrets go to Secrets map, plain inputs to Inputs map", func(t *testing.T) {
		inputs := schema.Inputs{
			"DB_PORT":     {Value: "8080", IsSecret: false},
			"DB_PASSWORD": {Value: "secret123", IsSecret: true},
		}
		state, err := NewState(&inputs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Inputs["DB_PORT"] != "8080" {
			t.Errorf("expected Inputs[DB_PORT]=8080, got %q", state.Inputs["DB_PORT"])
		}
		if _, ok := state.Inputs["DB_PASSWORD"]; ok {
			t.Error("expected DB_PASSWORD to NOT be in Inputs")
		}
		if state.Secrets["DB_PASSWORD"] != "secret123" {
			t.Errorf("expected Secrets[DB_PASSWORD]=secret123, got %q", state.Secrets["DB_PASSWORD"])
		}
		if _, ok := state.Secrets["DB_PORT"]; ok {
			t.Error("expected DB_PORT to NOT be in Secrets")
		}
	})

	t.Run("sets os env for template interpolation", func(t *testing.T) {
		inputs := schema.Inputs{
			"APP_NAME": {Value: "myapp", IsSecret: false},
		}
		state, err := NewState(&inputs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result, err := state.InterpolateTemplate("{{ env.APP_NAME }}")
		if err != nil {
			t.Fatalf("template interpolation failed: %v", err)
		}
		if result != "myapp" {
			t.Errorf("expected template result 'myapp', got %q", result)
		}
	})

	t.Run("invalid JSON file returns error", func(t *testing.T) {
		inputs := schema.Inputs{
			"KEY": {Value: "default", IsSecret: false},
		}
		tmpDir := t.TempDir()
		inputFile := filepath.Join(tmpDir, "inputs.json")
		if err := os.WriteFile(inputFile, []byte(`not json`), 0644); err != nil {
			t.Fatalf("write input file: %v", err)
		}
		_, err := NewState(&inputs, inputFile)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "parse input file") {
			t.Errorf("expected error about parsing input file, got %q", err.Error())
		}
	})

	t.Run("missing JSON file returns error", func(t *testing.T) {
		inputs := schema.Inputs{
			"KEY": {Value: "default", IsSecret: false},
		}
		_, err := NewState(&inputs, "/nonexistent/path/inputs.json")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
		if !strings.Contains(err.Error(), "read input file") {
			t.Errorf("expected error about reading input file, got %q", err.Error())
		}
	})

	t.Run("input file with multiple keys", func(t *testing.T) {
		inputs := schema.Inputs{
			"KEY1": {Value: "default1", IsSecret: false},
			"KEY2": {Value: "default2", IsSecret: true},
		}
		tmpDir := t.TempDir()
		inputFile := filepath.Join(tmpDir, "inputs.json")
		if err := os.WriteFile(inputFile, []byte(`{"KEY1":"override1","KEY2":"override2"}`), 0644); err != nil {
			t.Fatalf("write input file: %v", err)
		}
		state, err := NewState(&inputs, inputFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.Inputs["KEY1"] != "override1" {
			t.Errorf("expected KEY1=override1, got %q", state.Inputs["KEY1"])
		}
		if state.Secrets["KEY2"] != "override2" {
			t.Errorf("expected KEY2=override2, got %q", state.Secrets["KEY2"])
		}
	})
}

func TestResolveOutputs(t *testing.T) {
	tests := []struct {
		name        string
		outputs     map[string]string
		state       *State
		expected    map[string]string
		expectedErr error
	}{
		{
			name:        "empty outputs map returns nil",
			outputs:     map[string]string{},
			state:       &State{},
			expected:    nil,
			expectedErr: nil,
		},
		{
			name:    "nil outputs map returns nil",
			outputs: nil,
			state: &State{
				Meta:    map[string]string{"job": "test"},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    nil,
			expectedErr: nil,
		},
		{
			name: "resolves single output",
			outputs: map[string]string{
				"url": "https://example.com/{{ meta.job }}",
			},
			state: &State{
				Meta:    map[string]string{"job": "deploy"},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected: map[string]string{
				"url": "https://example.com/deploy",
			},
			expectedErr: nil,
		},
		{
			name: "resolves multiple outputs with different namespaces",
			outputs: map[string]string{
				"image":   "{{ steps.build.image }}:{{ steps.build.tag }}",
				"api_url": "{{ inputs.BASE_URL }}/api",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{"BASE_URL": "https://app.test"},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{"build": {"image": "myapp", "tag": "v1.2.3"}},
			},
			expected: map[string]string{
				"image":   "myapp:v1.2.3",
				"api_url": "https://app.test/api",
			},
			expectedErr: nil,
		},
		{
			name: "resolves outputs with secrets namespace",
			outputs: map[string]string{
				"token": "Bearer {{ secrets.API_KEY }}",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{"API_KEY": "shh"},
				Steps:   map[string]map[string]string{},
			},
			expected: map[string]string{
				"token": "Bearer shh",
			},
			expectedErr: nil,
		},
		{
			name: "resolves plain string without templates",
			outputs: map[string]string{
				"static": "no templates here",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected: map[string]string{
				"static": "no templates here",
			},
			expectedErr: nil,
		},
		{
			name: "returns error when interpolation fails",
			outputs: map[string]string{
				"bad": "{{ meta.missing }}",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    nil,
			expectedErr: fmt.Errorf(`output "bad": Could not find key 'missing' in meta namespace`),
		},
		{
			name: "returns error on first failing key",
			outputs: map[string]string{
				"good": "ok",
				"bad":  "{{ inputs.nope }}",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected:    nil,
			expectedErr: fmt.Errorf(`output "bad": Could not find key 'nope' in inputs`),
		},
		{
			name: "preserves key order via map iteration",
			outputs: map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
			},
			state: &State{
				Meta:    map[string]string{},
				Inputs:  map[string]string{},
				Secrets: map[string]string{},
				Steps:   map[string]map[string]string{},
			},
			expected: map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.state.ResolveOutputs(tt.outputs)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ResolveOutputs() = %v; expected %v", result, tt.expected)
			}
			if !errorsEqual(err, tt.expectedErr) {
				t.Errorf("ResolveOutputs() error = %v; expected %v", err, tt.expectedErr)
			}
		})
	}
}

