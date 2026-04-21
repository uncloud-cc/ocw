package steps

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// StepFactory implements workflow.StepFactory by dispatching to concrete constructors.
type StepFactory struct{}

// NewStepFactory creates a concrete step factory.
func NewStepFactory() *StepFactory {
	return &StepFactory{}
}

// Create instantiates the appropriate Step implementation for the given schema step.
func (f *StepFactory) Create(step *schema.Step, ctx *workflow.StepContext) (workflow.Step, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}

	switch {
	case step.RunStep != nil:
		return NewRunStep(step, ctx)
	case step.BuildStep != nil:
		return NewBuildStep(step, ctx)
	case step.SequenceStep != nil:
		return NewSequenceStep(step, ctx)
	case step.ParallelStep != nil:
		return NewParallelStep(step, ctx)
	case step.SwitchStep != nil:
		return NewSwitchStep(step, ctx)
	case step.WorkflowStep != nil:
		return NewWorkflowStep(step, ctx)
	default:
		return nil, fmt.Errorf("unknown step type: step has no recognized type field set")
	}
}
