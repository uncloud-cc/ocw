package sequence

import (
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator yields steps one at a time for sequential execution.
type Iterator struct {
	steps   []steps.Step
	context *steps.StepContext
	index   int
	outputs map[string]string
	done    bool
}

// Next returns the next step(s) to execute.
func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
	// TODO: Implement sequential iteration
	// 1. Process result from previous step (store outputs, check for failure)
	// 2. Check if we're done
	// 3. Return next step
	return nil, true, nil
}

// Result returns the final combined result after iteration completes.
func (it *Iterator) Result() *steps.Result {
	return steps.SuccessWithOutputs(it.outputs)
}
