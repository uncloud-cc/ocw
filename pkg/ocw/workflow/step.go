package workflow

import (
	"context"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ------------------------------------------------------------------------------
// Step Interface
// ------------------------------------------------------------------------------

// Step is the interface that all step types implement.
// Leaf steps (run, build) execute actual work.
// Composite steps (sequence, parallel, switch, workflow) return child steps.
type Step interface {
	// Type returns the step type ("run", "build", "sequence", "parallel", "switch", "workflow")
	Type() string

	// ID returns the step identifier for output tracking (may be empty)
	ID() string

	// Name returns the human-readable step name
	Name() string

	// Original returns the underlying schema.Step for inspection
	Original() *schema.Step

	// Execute runs the step and returns results.
	// For leaf steps: executes container operation, returns outputs
	// For composite steps: returns child steps to execute next
	Execute(ctx context.Context, stepCtx *StepContext, opts ExecuteOptions) (*StepResult, error)

	// Status returns the current state of the step for progress reporting.
	// Called by CLI to show real-time progress. Implementation varies by step type.
	Status() StepStatus
}

// StepFactory creates Step instances from schema.Step definitions.
type StepFactory interface {
	// Create instantiates the appropriate Step implementation for the given schema step.
	Create(step *schema.Step, ctx *StepContext) (Step, error)
}

// ExecuteOptions provides callbacks and configuration for step execution.
type ExecuteOptions struct {
	// Logger receives log output from the step
	Logger Logger
}

// Logger interface for step implementations to send logs out.
type Logger interface {
	// Info logs informational messages
	Info(msg string)
	// Debug logs debug messages
	Debug(msg string)
	// Error logs error messages
	Error(msg string)
	// Progress logs progress updates (0.0 to 1.0)
	Progress(percent float64)
}

// StepStatus represents the current state of a step for progress reporting.
// Step implementations define their own status structure and interpretation.
type StepStatus struct {
	// State is the current state (e.g., "pending", "running", "completed", "failed")
	State string

	// Message is a human-readable description of current activity
	Message string

	// Progress is a percentage (0.0 to 1.0) if applicable
	Progress float64

	// Metadata contains step-specific status information.
	// RunStep: {"container_id": "abc", "logs": "..."}
	// BuildStep: {"stage": "layer-5/10", "cache_hit": true}
	// etc.
	Metadata map[string]interface{}
}

// StepResult contains the result of executing a step.
type StepResult struct {
	// StepID identifies which step produced this result
	StepID string

	// Outputs are key-value pairs to merge into StepContext.Steps[StepID]
	Outputs map[string]string

	// Children are the next steps to execute (for composite steps)
	// Empty for leaf steps (run, build)
	Children []Step

	// Parallel indicates whether Children should run in parallel
	Parallel bool

	// Service contains info about a background container (for run steps with background: true)
	Service *ServiceInfo
}
