package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// SwitchStep represents a composite step that conditionally executes branches.
type SwitchStep struct {
	original    *schema.Step
	resolved    *schema.SwitchStep
	state       string
	switchValue string
	matchedCase string
}

func (s *SwitchStep) Type() string { return "switch" }
func (s *SwitchStep) ID() string   { return string(s.resolved.ID) }
func (s *SwitchStep) Name() string { return string(s.resolved.Name) }

func (s *SwitchStep) Original() *schema.Step { return s.original }

func (s *SwitchStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	if stepCtx.Factory == nil {
		return nil, fmt.Errorf("StepContext.Factory is nil")
	}

	// Evaluate the switch expression
	switchValue := s.resolved.Switch

	// Try to resolve as template
	resolvedValue, err := workflow.ResolveString(switchValue, stepCtx)
	if err == nil && resolvedValue != switchValue {
		switchValue = resolvedValue
	}

	s.switchValue = switchValue

	// Find matching case
	var matchedSteps *schema.StepOrSteps
	if caseSteps, ok := s.resolved.Case[switchValue]; ok {
		matchedSteps = &caseSteps
		s.matchedCase = switchValue
	} else if s.resolved.Default != nil {
		matchedSteps = s.resolved.Default
		s.matchedCase = "default"
	} else {
		return nil, fmt.Errorf("no matching case for switch value %q and no default", switchValue)
	}

	// Convert matched steps to executable steps
	var children []workflow.Step
	if matchedSteps.Single != nil {
		wrapped, err := stepCtx.Factory.Create(matchedSteps.Single, stepCtx)
		if err != nil {
			return nil, fmt.Errorf("create matched step: %w", err)
		}
		children = []workflow.Step{wrapped}
	} else if matchedSteps.Multiple != nil {
		children = make([]workflow.Step, len(matchedSteps.Multiple))
		for i, child := range matchedSteps.Multiple {
			childCopy := child
			wrapped, err := stepCtx.Factory.Create(&childCopy, stepCtx)
			if err != nil {
				return nil, fmt.Errorf("create matched step %d: %w", i, err)
			}
			children[i] = wrapped
		}
	}

	return &workflow.StepResult{
		StepID:   s.ID(),
		Children: children,
		Parallel: false,
	}, nil
}

// Status returns current execution status.
// For SwitchStep: {"state": "evaluating", "switch_value": "dev", "matched_case": "dev"}
func (s *SwitchStep) Status() workflow.StepStatus {
	return workflow.StepStatus{
		State:   s.state,
		Message: s.matchedCase,
		Metadata: map[string]interface{}{
			"switch_value": s.switchValue,
			"matched_case": s.matchedCase,
		},
	}
}

// NewSwitchStep creates a new switch step.
func NewSwitchStep(step *schema.Step, ctx *workflow.StepContext) (*SwitchStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.SwitchStep == nil {
		return nil, fmt.Errorf("step is not a switch step")
	}

	return &SwitchStep{
		original: step,
		resolved: step.SwitchStep,
		state:    "pending",
	}, nil
}
