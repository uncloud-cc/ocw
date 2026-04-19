package switchstep

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.SwitchStep into an executable switchstep.Step.
// Note: The switch value should already be interpolated before calling this.
func Parse(ss *schema.SwitchStep, value string, branches map[string][]steps.Step, defaultSteps []steps.Step) (*Step, error) {
	// TODO: Implement parsing logic
	// 1. Convert branches map to []Branch
	// 2. Create Step with value and branches
	return nil, fmt.Errorf("not implemented")
}
