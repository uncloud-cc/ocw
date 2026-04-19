package run

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.RunStep into an executable run.Step.
// This is where template interpolation happens.
func Parse(rs *schema.RunStep, stepContext *steps.StepContext, volumes map[string]steps.ResolvedVolume) (*Step, error) {
	// TODO: Implement parsing logic
	// 1. Interpolate image
	// 2. Interpolate command and args
	// 3. Interpolate entrypoint
	// 4. Interpolate workdir
	// 5. Parse environment variables
	// 6. Resolve volume mounts
	// 7. Parse port exposures
	// 8. Parse health check
	return nil, fmt.Errorf("not implemented")
}
