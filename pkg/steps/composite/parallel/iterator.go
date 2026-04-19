package parallel

import (
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator yields all steps at once for parallel execution.
type Iterator struct {
	steps   []steps.Step
	context *steps.StepContext
	started bool
	results []*steps.Result
}

// Next returns the next step(s) to execute.
func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
	// TODO: Implement parallel iteration
	// 1. First call: return all steps
	// 2. Second call: check for any failures, return done
	return nil, true, nil
}

// Result returns the final combined result after iteration completes.
func (it *Iterator) Result() *steps.Result {
	return steps.Merge(it.results)
}
