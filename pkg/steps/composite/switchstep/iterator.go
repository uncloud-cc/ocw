package switchstep

import (
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator executes the selected branch sequentially.
type Iterator struct {
	steps   []steps.Step
	context *steps.StepContext
	index   int
	outputs map[string]string
	done    bool
}

// Next returns the next step(s) to execute.
func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
	// TODO: Implement branch iteration
	// Similar to sequence iterator but for the selected branch
	return nil, true, nil
}

// Result returns the final combined result after iteration completes.
func (it *Iterator) Result() *steps.Result {
	return steps.SuccessWithOutputs(it.outputs)
}
