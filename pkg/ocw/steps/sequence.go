package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// SequenceStep represents a composite step that runs steps in sequence.
type SequenceStep struct {
	original *schema.Step
	resolved *schema.SequenceStep
	// runtime state for status reporting
	state         string
	currentIndex  int
	totalSteps    int
	currentStepID string
}

func (s *SequenceStep) Type() string { return "sequence" }
func (s *SequenceStep) ID() string   { return string(s.resolved.ID) }
func (s *SequenceStep) Name() string { return string(s.resolved.Name) }

func (s *SequenceStep) Original() *schema.Step { return s.original }

func (s *SequenceStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	if stepCtx.Factory == nil {
		return nil, fmt.Errorf("StepContext.Factory is nil")
	}

	// Wrap all child steps
	children := make([]workflow.Step, len(s.resolved.Sequence))
	for i, child := range s.resolved.Sequence {
		childCopy := child
		wrapped, err := stepCtx.Factory.Create(&childCopy, stepCtx)
		if err != nil {
			return nil, fmt.Errorf("create sequence step %d: %w", i, err)
		}
		children[i] = wrapped
	}

	s.totalSteps = len(children)

	return &workflow.StepResult{
		StepID:   s.ID(),
		Children: children,
		Parallel: false,
	}, nil
}

// Status returns current execution status.
// For SequenceStep: {"state": "executing", "current_step": "2/5", "current_step_id": "test"}
func (s *SequenceStep) Status() workflow.StepStatus {
	progress := 0.0
	if s.totalSteps > 0 {
		progress = float64(s.currentIndex) / float64(s.totalSteps)
	}
	return workflow.StepStatus{
		State:    s.state,
		Message:  s.currentStepID,
		Progress: progress,
		Metadata: map[string]interface{}{
			"current_index": s.currentIndex,
			"total_steps":   s.totalSteps,
			"current_id":    s.currentStepID,
		},
	}
}

// NewSequenceStep creates a new sequence step.
func NewSequenceStep(step *schema.Step, ctx *workflow.StepContext) (*SequenceStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.SequenceStep == nil {
		return nil, fmt.Errorf("step is not a sequence step")
	}

	return &SequenceStep{
		original: step,
		resolved: step.SequenceStep,
		state:    "pending",
	}, nil
}
