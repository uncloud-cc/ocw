package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestSequenceStep_Interface(t *testing.T) {
	step := &SequenceStep{
		original: &schema.Step{SequenceStep: &schema.SequenceStep{}},
		resolved: &schema.SequenceStep{
			OptionalStepBase: schema.OptionalStepBase{Name: "test-seq", ID: "seq-1"},
			Sequence: []schema.Step{
				{RunStep: &schema.RunStep{Image: "step1"}},
				{RunStep: &schema.RunStep{Image: "step2"}},
			},
		},
	}

	assert.Equal(t, "sequence", step.Type())
	assert.Equal(t, "seq-1", step.ID())
	assert.Equal(t, "test-seq", step.Name())
	assert.NotNil(t, step.Original)
}

func TestSequenceStep_Execute(t *testing.T) {
	tests := []struct {
		name         string
		sequence     []schema.Step
		wantChildren int
		wantParallel bool
		wantErr      bool
	}{
		{
			name: "sequence with multiple steps",
			sequence: []schema.Step{
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "image1"}},
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step2"}, Image: "image2"}},
				{BuildStep: &schema.BuildStep{StepBase: schema.StepBase{Name: "step3"}, Build: schema.BuildConfig{Image: "image3"}}},
			},
			wantChildren: 3,
			wantParallel: false,
			wantErr:      false,
		},
		{
			name:         "empty sequence",
			sequence:     []schema.Step{},
			wantChildren: 0,
			wantParallel: false,
			wantErr:      false,
		},
		{
			name: "single step sequence",
			sequence: []schema.Step{
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "only"}, Image: "only"}},
			},
			wantChildren: 1,
			wantParallel: false,
			wantErr:      false,
		},
		{
			name: "nested composite steps",
			sequence: []schema.Step{
				{
					SequenceStep: &schema.SequenceStep{
						Sequence: []schema.Step{
							{RunStep: &schema.RunStep{Image: "nested1"}},
						},
					},
				},
				{RunStep: &schema.RunStep{Image: "flat"}},
			},
			wantChildren: 2,
			wantParallel: false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &SequenceStep{
				original: &schema.Step{SequenceStep: &schema.SequenceStep{Sequence: tt.sequence}},
				resolved: &schema.SequenceStep{
					Sequence: tt.sequence,
				},
			}

			ctx := context.Background()
			stepCtx := &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			}

			result, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantChildren, len(result.Children))
				assert.Equal(t, tt.wantParallel, result.Parallel)
				assert.Empty(t, result.Outputs) // Composite steps don't have outputs
				assert.Nil(t, result.Service)   // No background service
			}
		})
	}
}

func TestNewSequenceStep(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.Step
		ctx     *workflow.StepContext
		wantErr bool
	}{
		{
			name: "valid sequence step",
			step: &schema.Step{
				SequenceStep: &schema.SequenceStep{
					OptionalStepBase: schema.OptionalStepBase{Name: "seq"},
					Sequence: []schema.Step{
						{RunStep: &schema.RunStep{Image: "s1"}},
					},
				},
			},
			ctx:     &workflow.StepContext{},
			wantErr: false,
		},
		{
			name:    "nil sequence step",
			step:    &schema.Step{SequenceStep: nil},
			ctx:     &workflow.StepContext{},
			wantErr: true,
		},
		{
			name:    "nil step",
			step:    nil,
			ctx:     &workflow.StepContext{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seqStep, err := NewSequenceStep(tt.step, tt.ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, seqStep)
			} else {
				require.NoError(t, err)
				require.NotNil(t, seqStep)
				assert.Equal(t, "sequence", seqStep.Type())
			}
		})
	}
}

func TestSequenceStep_ChildWrapping(t *testing.T) {
	sequence := []schema.Step{
		{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "run-child"}, Image: "alpine"}},
		{BuildStep: &schema.BuildStep{StepBase: schema.StepBase{Name: "build-child"}, Build: schema.BuildConfig{Image: "app"}}},
	}

	step := &SequenceStep{
		original: &schema.Step{SequenceStep: &schema.SequenceStep{Sequence: sequence}},
		resolved: &schema.SequenceStep{Sequence: sequence},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Steps:   make(map[string]map[string]string),
		Factory: &testStepFactory{},
	}

	result, err := step.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify children are wrapped correctly
	require.Len(t, result.Children, 2)
	assert.Equal(t, "run", result.Children[0].Type())
	assert.Equal(t, "build", result.Children[1].Type())
}

func TestSequenceStep_Propagation(t *testing.T) {
	// Ensure step properties are preserved
	sequence := []schema.Step{
		{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "child", ID: "child-1"}, Image: "img"}},
	}

	step := &SequenceStep{
		original: &schema.Step{
			SequenceStep: &schema.SequenceStep{
				OptionalStepBase: schema.OptionalStepBase{Name: "parent", ID: "parent-1"},
				Sequence:         sequence,
			},
		},
		resolved: &schema.SequenceStep{
			OptionalStepBase: schema.OptionalStepBase{Name: "parent", ID: "parent-1"},
			Sequence:         sequence,
		},
	}

	assert.Equal(t, "parent", step.Name())
	assert.Equal(t, "parent-1", step.ID())
}
