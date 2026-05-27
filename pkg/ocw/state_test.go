package ocw

import (
	"fmt"
	"reflect"
	"testing"
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

func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}
