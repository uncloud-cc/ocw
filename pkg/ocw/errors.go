package ocw

import (
	"errors"
	"fmt"
)

// Common errors returned by OCW operations.
var (
	// ErrInvalidConfig indicates invalid configuration.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrCancelled indicates an operation was cancelled.
	ErrCancelled = errors.New("cancelled")

	// ErrDependencyFailed indicates a step dependency failed.
	ErrDependencyFailed = errors.New("dependency failed")
)

// StepError represents an error that occurred during step execution.
type StepError struct {
	// StepID is the ID of the step that failed (if set).
	StepID string

	// StepName is the name of the step that failed.
	StepName string

	// StepType is the type of step that failed.
	StepType StepType

	// ExitCode is the exit code if the step's command failed.
	// Nil if the error wasn't from command execution.
	ExitCode *int

	// Cause is the underlying error.
	Cause error
}

func (e *StepError) Error() string {
	if e.ExitCode != nil {
		return fmt.Sprintf("step %q failed with exit code %d", e.stepIdentifier(), *e.ExitCode)
	}
	if e.Cause != nil {
		return fmt.Sprintf("step %q failed: %v", e.stepIdentifier(), e.Cause)
	}
	return fmt.Sprintf("step %q failed", e.stepIdentifier())
}

func (e *StepError) stepIdentifier() string {
	if e.StepID != "" {
		return e.StepID
	}
	return e.StepName
}

func (e *StepError) Unwrap() error {
	return e.Cause
}

// NewStepError creates a new StepError.
func NewStepError(step Step, cause error) *StepError {
	return &StepError{
		StepID:   step.ID(),
		StepName: step.Name(),
		StepType: step.Type(),
		Cause:    cause,
	}
}

// NewStepExitError creates a StepError for a non-zero exit code.
func NewStepExitError(step Step, exitCode int) *StepError {
	return &StepError{
		StepID:   step.ID(),
		StepName: step.Name(),
		StepType: step.Type(),
		ExitCode: &exitCode,
	}
}

// WorkflowError represents an error during workflow execution.
type WorkflowError struct {
	// WorkflowName is the name of the workflow that failed.
	WorkflowName string

	// JobName is the job that failed (if applicable).
	JobName string

	// Cause is the underlying error (typically a StepError).
	Cause error
}

func (e *WorkflowError) Error() string {
	if e.JobName != "" {
		return fmt.Sprintf("workflow %q job %q failed: %v", e.WorkflowName, e.JobName, e.Cause)
	}
	return fmt.Sprintf("workflow %q failed: %v", e.WorkflowName, e.Cause)
}

func (e *WorkflowError) Unwrap() error {
	return e.Cause
}

// IsCancelled returns true if the error indicates cancellation.
func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}

// IsDependencyFailed returns true if the error indicates a dependency failure.
func IsDependencyFailed(err error) bool {
	return errors.Is(err, ErrDependencyFailed)
}
