// Package steps defines the step interfaces and provides implementations.
// Steps are divided into two categories:
//   - Simple steps implement Execute() and do actual work
//   - Composite steps implement Iterator() and yield child steps
package steps

import "context"

// Step is the base interface for all steps.
// Use type assertions to determine if a step is Simple or Composite.
type Step interface {
	// ID returns the step's identifier (for output references).
	// Returns empty string if the step has no ID.
	ID() string

	// Name returns the step's display name.
	Name() string
}

// SimpleStep executes directly against the container runtime.
// These are leaf nodes that do actual work (run containers, build images).
type SimpleStep interface {
	Step

	// Execute runs the step and returns its result.
	// The Executor provides access to the container runtime and shared state.
	Execute(ctx context.Context, exec Executor) (*Result, error)
}

// CompositeStep contains other steps and controls their execution flow.
// These are control flow nodes (sequence, parallel, switch).
type CompositeStep interface {
	Step

	// Children returns all child steps (for validation, visualization, etc.)
	Children() []Step

	// Iterator returns a fresh iterator for executing this composite step.
	// The stepContext provides the interpolation context for child steps.
	Iterator(stepContext *StepContext) StepIterator
}
