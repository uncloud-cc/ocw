// Package parallel implements the parallel step for running child steps concurrently.
package parallel

import (
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes child steps concurrently. Implements CompositeStep.
type Step struct {
	id       string
	name     string
	children []steps.Step
}

// New creates a new parallel step.
func New(id, name string, children []steps.Step) *Step {
	return &Step{
		id:       id,
		name:     name,
		children: children,
	}
}

// ID returns the step's identifier.
func (s *Step) ID() string {
	return s.id
}

// Name returns the step's display name.
func (s *Step) Name() string {
	return s.name
}

// Children returns all child steps.
func (s *Step) Children() []steps.Step {
	return s.children
}

// Iterator returns a fresh iterator for executing this composite step.
func (s *Step) Iterator(stepContext *steps.StepContext) steps.StepIterator {
	return &Iterator{
		steps:   s.children,
		context: stepContext,
		started: false,
		results: nil,
	}
}
