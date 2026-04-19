package steps

// Result contains the outcome of a step execution.
type Result struct {
	// Outputs are key-value pairs that can be referenced by subsequent steps
	// via {{ steps.<id>.<key> }} syntax.
	Outputs map[string]string

	// ExitCode is the container's exit code (for run steps).
	// 0 indicates success, non-zero indicates failure.
	ExitCode int

	// ContainerID is set for background steps, allowing the runtime
	// to track and clean up the container.
	ContainerID string

	// IsBackground indicates this step started a background service.
	IsBackground bool
}

// Success creates a successful result with no outputs.
func Success() *Result {
	return &Result{ExitCode: 0, Outputs: make(map[string]string)}
}

// SuccessWithOutputs creates a successful result with outputs.
func SuccessWithOutputs(outputs map[string]string) *Result {
	return &Result{ExitCode: 0, Outputs: outputs}
}

// Failed creates a failed result with the given exit code.
func Failed(exitCode int) *Result {
	return &Result{ExitCode: exitCode, Outputs: make(map[string]string)}
}

// Merge combines multiple results into one (for parallel steps).
func Merge(results []*Result) *Result {
	merged := &Result{
		Outputs:  make(map[string]string),
		ExitCode: 0,
	}
	for _, r := range results {
		if r == nil {
			continue
		}
		for k, v := range r.Outputs {
			merged.Outputs[k] = v
		}
		if r.ExitCode != 0 {
			merged.ExitCode = r.ExitCode
		}
	}
	return merged
}
