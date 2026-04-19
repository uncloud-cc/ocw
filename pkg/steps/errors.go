package steps

import "fmt"

// StepError represents a step execution failure.
// This type is used by both the steps package and the ocw runtime.
type StepError struct {
	// StepID is the identifier of the step that failed.
	StepID string
	// StepName is the display name of the step that failed.
	StepName string
	// ExitCode is the exit code from the step (0 = success).
	ExitCode int
	// Err is the underlying error (optional).
	Err error
}

// Error returns the error message.
func (e *StepError) Error() string {
	if e.StepName != "" {
		return fmt.Sprintf("step %q (id: %s) failed with exit code %d", e.StepName, e.StepID, e.ExitCode)
	}
	if e.StepID != "" {
		return fmt.Sprintf("step %s failed with exit code %d", e.StepID, e.ExitCode)
	}
	return fmt.Sprintf("step failed with exit code %d", e.ExitCode)
}

// Unwrap returns the underlying error.
func (e *StepError) Unwrap() error {
	return e.Err
}
