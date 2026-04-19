package build

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.BuildStep into an executable build.Step.
// This is where template interpolation happens.
func Parse(bs *schema.BuildStep, stepContext *steps.StepContext, volumes map[string]steps.ResolvedVolume) (*Step, error) {
	// TODO: Implement parsing logic
	// 1. Interpolate build configuration fields
	// 2. Convert schema.BuildConfig to build.Step
	// 3. Handle build secrets
	return nil, fmt.Errorf("not implemented")
}
