package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestSwitchStep_Interface(t *testing.T) {
	step := &SwitchStep{
		original: &schema.Step{SwitchStep: &schema.SwitchStep{}},
		resolved: &schema.SwitchStep{
			OptionalStepBase: schema.OptionalStepBase{Name: "test-switch", ID: "sw-1"},
			Switch:           "${{ env.ENV }}",
			Case: map[string]schema.StepOrSteps{
				"dev":  {Single: &schema.Step{RunStep: &schema.RunStep{Image: "dev-img"}}},
				"prod": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "prod-img"}}},
			},
			Default: &schema.StepOrSteps{Single: &schema.Step{RunStep: &schema.RunStep{Image: "default-img"}}},
		},
	}

	assert.Equal(t, "switch", step.Type())
	assert.Equal(t, "sw-1", step.ID())
	assert.Equal(t, "test-switch", step.Name())
	assert.NotNil(t, step.Original)
}

func TestSwitchStep_Execute(t *testing.T) {
	tests := []struct {
		name         string
		switchExpr   string
		cases        map[string]schema.StepOrSteps
		defaultCase  *schema.StepOrSteps
		ctx          *workflow.StepContext
		wantMatch    string
		wantChildren int
		wantParallel bool
		wantErr      bool
	}{
		{
			name:       "matches dev case",
			switchExpr: "dev",
			cases: map[string]schema.StepOrSteps{
				"dev":  {Single: &schema.Step{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "dev-step"}, Image: "dev"}}},
				"prod": {Single: &schema.Step{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "prod-step"}, Image: "prod"}}},
			},
			defaultCase: nil,
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantMatch:    "dev",
			wantChildren: 1,
			wantParallel: false,
			wantErr:      false,
		},
		{
			name:       "matches prod case",
			switchExpr: "prod",
			cases: map[string]schema.StepOrSteps{
				"dev":  {Single: &schema.Step{RunStep: &schema.RunStep{Image: "dev"}}},
				"prod": {Single: &schema.Step{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "prod-deploy"}, Image: "prod"}}},
			},
			defaultCase: nil,
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantMatch:    "prod",
			wantChildren: 1,
			wantParallel: false,
		},
		{
			name:       "falls back to default",
			switchExpr: "staging",
			cases: map[string]schema.StepOrSteps{
				"dev":  {Single: &schema.Step{RunStep: &schema.RunStep{Image: "dev"}}},
				"prod": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "prod"}}},
			},
			defaultCase: &schema.StepOrSteps{
				Single: &schema.Step{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "default-step"}, Image: "default"}},
			},
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantMatch:    "default",
			wantChildren: 1,
			wantParallel: false,
			wantErr:      false,
		},
		{
			name:       "template expression resolution",
			switchExpr: "${{ env.DEPLOY_ENV }}",
			cases: map[string]schema.StepOrSteps{
				"production": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "prod"}}},
			},
			defaultCase: nil,
			ctx: &workflow.StepContext{
				Env:     map[string]string{"DEPLOY_ENV": "production"},
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantMatch:    "production",
			wantChildren: 1,
			wantParallel: false,
		},
		{
			name:       "no match and no default - error",
			switchExpr: "unknown",
			cases: map[string]schema.StepOrSteps{
				"dev": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "dev"}}},
			},
			defaultCase: nil,
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantErr: true,
		},
		{
			name:       "multiple steps in case",
			switchExpr: "complex",
			cases: map[string]schema.StepOrSteps{
				"complex": {Multiple: []schema.Step{
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step1"}, Image: "img1"}},
					{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step2"}, Image: "img2"}},
				}},
			},
			defaultCase: nil,
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantChildren: 2,
			wantParallel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &SwitchStep{
				original: &schema.Step{SwitchStep: &schema.SwitchStep{
					Switch:  tt.switchExpr,
					Case:    tt.cases,
					Default: tt.defaultCase,
				}},
				resolved: &schema.SwitchStep{
					Switch:  tt.switchExpr,
					Case:    tt.cases,
					Default: tt.defaultCase,
				},
			}

			ctx := context.Background()
			result, err := step.Execute(ctx, tt.ctx, workflow.ExecuteOptions{})

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantChildren, len(result.Children))
				assert.Equal(t, tt.wantParallel, result.Parallel)
			}
		})
	}
}

func TestNewSwitchStep(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.Step
		ctx     *workflow.StepContext
		wantErr bool
	}{
		{
			name: "valid switch step",
			step: &schema.Step{
				SwitchStep: &schema.SwitchStep{
					Switch: "${{ env.ENV }}",
					Case: map[string]schema.StepOrSteps{
						"dev": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "dev"}}},
					},
				},
			},
			ctx:     &workflow.StepContext{},
			wantErr: false,
		},
		{
			name:    "nil switch step",
			step:    &schema.Step{SwitchStep: nil},
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
			switchStep, err := NewSwitchStep(tt.step, tt.ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, switchStep)
			} else {
				require.NoError(t, err)
				require.NotNil(t, switchStep)
				assert.Equal(t, "switch", switchStep.Type())
			}
		})
	}
}

func TestSwitchStep_BooleanValues(t *testing.T) {
	// Test switch on boolean-like values
	tests := []struct {
		name       string
		switchExpr string
		cases      map[string]schema.StepOrSteps
		ctx        *workflow.StepContext
		wantErr    bool
	}{
		{
			name:       "switch on true",
			switchExpr: "true",
			cases: map[string]schema.StepOrSteps{
				"true":  {Single: &schema.Step{RunStep: &schema.RunStep{Image: "when-true"}}},
				"false": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "when-false"}}},
			},
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantErr: false,
		},
		{
			name:       "switch on false",
			switchExpr: "false",
			cases: map[string]schema.StepOrSteps{
				"true":  {Single: &schema.Step{RunStep: &schema.RunStep{Image: "when-true"}}},
				"false": {Single: &schema.Step{RunStep: &schema.RunStep{Image: "when-false"}}},
			},
			ctx: &workflow.StepContext{
				Steps:   make(map[string]map[string]string),
				Factory: &testStepFactory{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &SwitchStep{
				resolved: &schema.SwitchStep{
					Switch: tt.switchExpr,
					Case:   tt.cases,
				},
			}

			ctx := context.Background()
			result, err := step.Execute(ctx, tt.ctx, workflow.ExecuteOptions{})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}
