package sequence

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.SequenceStep into an executable sequence.Step.
// Note: Child steps are NOT parsed here - they are parsed lazily by the runtime
// as the iterator yields them, allowing proper scope accumulation.
func Parse(ss *schema.SequenceStep, childSteps []steps.Step) (*Step, error) {
	// TODO: Implement parsing logic
	return nil, fmt.Errorf("not implemented")
}
