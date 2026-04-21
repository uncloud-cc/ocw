package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// WorkflowStep represents a composite step that loads and runs another workflow.
type WorkflowStep struct {
	original      *schema.Step
	resolved      *schema.WorkflowStep
	state         string
	workflowPath  string
	childWorkflow string
}

func (s *WorkflowStep) Type() string { return "workflow" }
func (s *WorkflowStep) ID() string   { return string(s.resolved.ID) }
func (s *WorkflowStep) Name() string { return string(s.resolved.Name) }

func (s *WorkflowStep) Original() *schema.Step { return s.original }

func (s *WorkflowStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	// Resolve workflow path
	workflowPath := s.resolved.Workflow.From
	resolvedPath, err := workflow.ResolveString(workflowPath, stepCtx)
	if err == nil {
		workflowPath = resolvedPath
	}
	s.workflowPath = workflowPath

	// Load the external workflow file
	ocwSchema, err := ocw.ParseFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", workflowPath, err)
	}

	// Validate the loaded workflow
	if err := ocwSchema.Validate(); err != nil {
		return nil, fmt.Errorf("validate workflow %s: %w", workflowPath, err)
	}

	s.childWorkflow = string(ocwSchema.Name)

	// Create a new context for the sub-workflow
	subCtx := stepCtx.Clone()
	subCtx.Workflow = workflow.WorkflowMeta{
		Name: string(ocwSchema.Name),
		ID:   string(ocwSchema.ID),
		Path: workflowPath,
	}

	// Handle inheritance of env and secrets
	if s.resolved.Workflow.Inherit != nil {
		// Inherit secrets
		if s.resolved.Workflow.Inherit.Secrets == schema.InheritAll {
			for k, v := range stepCtx.Secrets {
				subCtx.Secrets[k] = v
			}
		}
		// Inherit env
		if s.resolved.Workflow.Inherit.Env == schema.InheritAll {
			for k, v := range stepCtx.Env {
				subCtx.Env[k] = v
			}
		}
	}

	// Add workflow-specific env
	for k, v := range s.resolved.Workflow.Env {
		subCtx.Env[k] = v.Value
	}

	// Add workflow-specific secrets
	for k, v := range s.resolved.Workflow.Secrets {
		if v.Secure != nil {
			subCtx.Secrets[k] = v.Secure.Secure
		} else {
			subCtx.Secrets[k] = v.Plain
		}
	}

	// Find entry point for the sub-workflow
	// For now, just return an empty result - the caller would need to create a new Engine
	// to execute the sub-workflow properly
	return &workflow.StepResult{
		StepID:   s.ID(),
		Children: []workflow.Step{}, // Empty for now - would need proper implementation
		Parallel: false,
	}, nil
}

// Status returns current execution status.
// For WorkflowStep: {"state": "loading", "workflow_path": "./child.yaml"}
func (s *WorkflowStep) Status() workflow.StepStatus {
	return workflow.StepStatus{
		State:   s.state,
		Message: s.workflowPath,
		Metadata: map[string]interface{}{
			"workflow_path":  s.workflowPath,
			"child_workflow": s.childWorkflow,
		},
	}
}

// NewWorkflowStep creates a new workflow step.
func NewWorkflowStep(step *schema.Step, ctx *workflow.StepContext) (*WorkflowStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.WorkflowStep == nil {
		return nil, fmt.Errorf("step is not a workflow step")
	}

	return &WorkflowStep{
		original: step,
		resolved: step.WorkflowStep,
		state:    "pending",
	}, nil
}
