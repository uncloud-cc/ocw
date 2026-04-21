package steps

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// testStepFactory is a simple factory for tests that creates real step instances.
type testStepFactory struct{}

func (f *testStepFactory) Create(step *schema.Step, ctx *workflow.StepContext) (workflow.Step, error) {
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
		return nil, fmt.Errorf("unknown step type")
	}
}
