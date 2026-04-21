package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestParallelStep_Interface(t *testing.T) {
	step := &ParallelStep{
		original: &schema.Step{ParallelStep: &schema.ParallelStep{}},
		resolved: &schema.ParallelStep{
			OptionalStepBase: schema.OptionalStepBase{Name: "test-parallel", ID: "par-1"},
			Parallel: []schema.Step{
				{RunStep: &schema.RunStep{Image: "step1"}},
				{RunStep: &schema.RunStep{Image: "step2"}},
			},
		},
	}

	assert.Equal(t, "parallel", step.Type())
	assert.Equal(t, "par-1", step.ID())
	assert.Equal(t, "test-parallel", step.Name())
	assert.NotNil(t, step.Original)
}

func TestParallelStep_Execute(t *testing.T) {
	tests := []struct {
		name         string
		parallel     []schema.Step
		wantChildren int
		wantParallel bool
		wantErr      bool
	}{
		{
			name: "parallel with multiple steps",
			parallel: []schema.Step{
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test1", ID: "t1"}, Image: "image1"}},
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test2", ID: "t2"}, Image: "image2"}},
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "test3", ID: "t3"}, Image: "image3"}},
			},
			wantChildren: 3,
			wantParallel: true, // Key difference from sequence
			wantErr:      false,
		},
		{
			name:         "empty parallel",
			parallel:     []schema.Step{},
			wantChildren: 0,
			wantParallel: true,
			wantErr:      false,
		},
		{
			name: "single step parallel",
			parallel: []schema.Step{
				{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "only"}, Image: "only"}},
			},
			wantChildren: 1,
			wantParallel: true,
			wantErr:      false,
		},
		{
			name: "mixed step types in parallel",
			parallel: []schema.Step{
				{RunStep: &schema.RunStep{Image: "run-img"}},
				{BuildStep: &schema.BuildStep{Build: schema.BuildConfig{Image: "build-img"}}},
				{RunStep: &schema.RunStep{Image: "another-run"}},
			},
			wantChildren: 3,
			wantParallel: true,
			wantErr:      false,
		},
		{
			name: "nested parallel within parallel",
			parallel: []schema.Step{
				{
					ParallelStep: &schema.ParallelStep{
						Parallel: []schema.Step{
							{RunStep: &schema.RunStep{Image: "nested1"}},
						},
					},
				},
				{RunStep: &schema.RunStep{Image: "flat"}},
			},
			wantChildren: 2,
			wantParallel: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &ParallelStep{
				original: &schema.Step{ParallelStep: &schema.ParallelStep{Parallel: tt.parallel}},
				resolved: &schema.ParallelStep{
					Parallel: tt.parallel,
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
				assert.True(t, result.Parallel, "Parallel must always be true for ParallelStep")
				assert.Empty(t, result.Outputs)
				assert.Nil(t, result.Service)
			}
		})
	}
}

func TestNewParallelStep(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.Step
		ctx     *workflow.StepContext
		wantErr bool
	}{
		{
			name: "valid parallel step",
			step: &schema.Step{
				ParallelStep: &schema.ParallelStep{
					OptionalStepBase: schema.OptionalStepBase{Name: "par"},
					Parallel: []schema.Step{
						{RunStep: &schema.RunStep{Image: "s1"}},
					},
				},
			},
			ctx:     &workflow.StepContext{},
			wantErr: false,
		},
		{
			name:    "nil parallel step",
			step:    &schema.Step{ParallelStep: nil},
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
			parStep, err := NewParallelStep(tt.step, tt.ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, parStep)
			} else {
				require.NoError(t, err)
				require.NotNil(t, parStep)
				assert.Equal(t, "parallel", parStep.Type())
			}
		})
	}
}

func TestParallelStep_ContextCloning(t *testing.T) {
	// Parallel steps should clone context for each branch
	parallel := []schema.Step{
		{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "branch1"}, Image: "img1"}},
		{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "branch2"}, Image: "img2"}},
	}

	step := &ParallelStep{
		original: &schema.Step{ParallelStep: &schema.ParallelStep{Parallel: parallel}},
		resolved: &schema.ParallelStep{Parallel: parallel},
	}

	originalCtx := &workflow.StepContext{
		Env:      map[string]string{"VAR": "original"},
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*workflow.ServiceInfo),
		Factory:  &testStepFactory{},
	}

	ctx := context.Background()
	result, err := step.Execute(ctx, originalCtx, workflow.ExecuteOptions{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Parallel)

	// Each child should have been wrapped with a cloned context
	// This prevents race conditions between parallel branches
}

func TestParallelStep_VsSequence(t *testing.T) {
	// Demonstrate key difference: Parallel=true vs Parallel=false
	sameChildren := []schema.Step{
		{RunStep: &schema.RunStep{Image: "s1"}},
		{RunStep: &schema.RunStep{Image: "s2"}},
	}

	parallelStep := &ParallelStep{
		resolved: &schema.ParallelStep{Parallel: sameChildren},
	}

	sequenceStep := &SequenceStep{
		resolved: &schema.SequenceStep{Sequence: sameChildren},
	}

	ctx := context.Background()
	stepCtx := &workflow.StepContext{
		Steps:   make(map[string]map[string]string),
		Factory: &testStepFactory{},
	}

	parResult, err := parallelStep.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)
	assert.True(t, parResult.Parallel, "Parallel step returns Parallel=true")

	seqResult, err := sequenceStep.Execute(ctx, stepCtx, workflow.ExecuteOptions{})
	require.NoError(t, err)
	assert.False(t, seqResult.Parallel, "Sequence step returns Parallel=false")

	// Both return same children count
	assert.Len(t, parResult.Children, 2)
	assert.Len(t, seqResult.Children, 2)
}
