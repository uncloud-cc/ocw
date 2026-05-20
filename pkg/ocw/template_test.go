package ocw

import (
	"fmt"
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
			input: "{{ meta.job }}-{{ env.SOMETHING }}-{{ steps.build.id }}",
			state: &State{
				Meta:  map[string]string{"job": "<jobName>"},
				Env:   map[string]string{"SOMETHING": "<env>"},
				Steps: map[string]map[string]string{"build": {"id": "<stepOutput>"}},
			},
			expected:    "<jobName>-<env>-<stepOutput>",
			expectedErr: nil,
		},
		{
			name:  "String without templates returns the same string without complaints",
			input: "Look ma, no templates!",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{},
			},
			expected:    "Look ma, no templates!",
			expectedErr: nil,
		},
		{
			name:  "Returns error when it can't find meta key",
			input: "{{ meta.doesNotExist }}",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'doesNotExist' in meta namespace"),
		},
		{
			name:  "Returns error when it can't find env key",
			input: "{{ env.doesNotExist }}",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'doesNotExist' in env namespace"),
		},
		{
			name:  "Returns error when there's not enough parts in referencing step output",
			input: "{{ steps.notEnoughParts }}",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Step output references in templates need three parts: {{ steps.<stepId>.<key> }} (e.g. {{ steps.build.image }})"),
		},
		{
			name:  "Returns error when it can't find specified step in outputs",
			input: "{{ steps.doesNotExist.output }}",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not any outputs for step 'doesNotExist'"),
		},
		{
			name:  "Returns error when it can't find key in step outputs",
			input: "{{ steps.mystep.outputValue }}",
			state: &State{
				Meta:  map[string]string{},
				Env:   map[string]string{},
				Steps: map[string]map[string]string{"mystep": {}},
			},
			expected:    "",
			expectedErr: fmt.Errorf("Could not find key 'outputValue' in step outputs for step 'mystep'"),
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

func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}
