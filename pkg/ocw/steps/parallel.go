package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ParallelStep represents a composite step that runs steps in parallel.
type ParallelStep struct {
	original *schema.Step
	resolved *schema.ParallelStep
	// runtime state for status reporting
	state        string
	activeCount  int
	totalCount   int
	completedIDs []string
}

func (s *ParallelStep) Type() string { return "parallel" }
func (s *ParallelStep) ID() string   { return string(s.resolved.ID) }
func (s *ParallelStep) Name() string { return string(s.resolved.Name) }

func (s *ParallelStep) Original() *schema.Step { return s.original }

func (s *ParallelStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	if stepCtx.Factory == nil {
		return nil, fmt.Errorf("StepContext.Factory is nil")
	}

	// Wrap all child steps with cloned context for each branch
	children := make([]workflow.Step, len(s.resolved.Parallel))
	for i, child := range s.resolved.Parallel {
		childCopy := child
		branchCtx := stepCtx.Clone()
		wrapped, err := branchCtx.Factory.Create(&childCopy, branchCtx)
		if err != nil {
			return nil, fmt.Errorf("create parallel step %d: %w", i, err)
		}
		children[i] = wrapped
	}

	s.totalCount = len(children)
	s.activeCount = len(children)

	return &workflow.StepResult{
		StepID:   s.ID(),
		Children: children,
		Parallel: true,
	}, nil
}

// Status returns current execution status.
// For ParallelStep: {"state": "executing", "active": 3, "completed": ["step1", "step2"]}
func (s *ParallelStep) Status() workflow.StepStatus {
	progress := 0.0
	if s.totalCount > 0 {
		progress = float64(len(s.completedIDs)) / float64(s.totalCount)
	}
	return workflow.StepStatus{
		State:    s.state,
		Message:  s.state,
		Progress: progress,
		Metadata: map[string]interface{}{
			"active_count":  s.activeCount,
			"total_count":   s.totalCount,
			"completed_ids": s.completedIDs,
		},
	}
}

// NewParallelStep creates a new parallel step.
func NewParallelStep(step *schema.Step, ctx *workflow.StepContext) (*ParallelStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.ParallelStep == nil {
		return nil, fmt.Errorf("step is not a parallel step")
	}

	return &ParallelStep{
		original: step,
		resolved: step.ParallelStep,
		state:    "pending",
	}, nil
}
