// Package switchstep implements the switch step for conditional branching.
// Named "switchstep" because "switch" is a Go reserved word.
package switchstep

import (
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Branch represents a case branch.
type Branch struct {
	// Value is the case value to match.
	Value string
	// Steps are the steps to execute if matched.
	Steps []steps.Step
}

// Step conditionally executes a branch based on a value. Implements CompositeStep.
type Step struct {
	id           string
	name         string
	value        string
	branches     []Branch
	defaultSteps []steps.Step
}

// New creates a new switch step.
func New(id, name, value string, branches []Branch, defaultSteps []steps.Step) *Step {
	return &Step{
		id:           id,
		name:         name,
		value:        value,
		branches:     branches,
		defaultSteps: defaultSteps,
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

// Children returns all possible children for validation/visualization.
func (s *Step) Children() []steps.Step {
	var all []steps.Step
	for _, b := range s.branches {
		all = append(all, b.Steps...)
	}
	all = append(all, s.defaultSteps...)
	return all
}

// Iterator returns a fresh iterator for executing this composite step.
func (s *Step) Iterator(stepContext *steps.StepContext) steps.StepIterator {
	// TODO: Find matching branch and create iterator
	return &Iterator{
		done: true,
	}
}
