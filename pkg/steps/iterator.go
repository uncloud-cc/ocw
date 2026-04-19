package steps

// StepIterator yields steps to execute from a composite step.
// The runtime drives iteration by calling Next() repeatedly.
type StepIterator interface {
	// Next returns the next step(s) to execute.
	//
	// Parameters:
	//   - lastResults: Results from the previous Next() call's steps.
	//                  Nil on first call.
	//
	// Returns:
	//   - steps: The next step(s) to execute. Multiple steps means parallel execution.
	//   - done: True if iteration is complete (steps will be empty).
	//   - err: Error if iteration cannot continue.
	//
	// The runtime calls Next() in a loop:
	//   1. Call Next(nil) to get first step(s)
	//   2. Execute returned steps
	//   3. Call Next(results) with execution results
	//   4. Repeat until done=true or err!=nil
	Next(lastResults []*Result) (steps []Step, done bool, err error)

	// Result returns the final combined result after iteration completes.
	// Only valid to call after Next() returns done=true.
	Result() *Result
}
