package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestParseReturnsErrorNotImplemented(t *testing.T) {
	rs := &schema.RunStep{
		StepBase: schema.StepBase{
			ID:   schema.ID("step1"),
			Name: schema.Name("Test Step"),
		},
		Image: "alpine:latest",
		Cmd:   "echo hello",
	}

	stepContext := steps.NewStepContext()
	volumes := map[string]steps.ResolvedVolume{}

	_, err := Parse(rs, stepContext, volumes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// Test specification: Parse should interpolate image field
func TestParseInterpolatesImage(t *testing.T) {
	// TODO: When implemented, test image interpolation
	t.Skip("Not yet implemented")
}

// Test specification: Parse should interpolate command and args
func TestParseInterpolatesCommand(t *testing.T) {
	// TODO: When implemented, test cmd interpolation
	t.Skip("Not yet implemented")
}

// Test specification: Parse should interpolate entrypoint
func TestParseInterpolatesEntrypoint(t *testing.T) {
	// TODO: When implemented, test entrypoint interpolation
	t.Skip("Not yet implemented")
}

// Test specification: Parse should interpolate workdir
func TestParseInterpolatesWorkdir(t *testing.T) {
	// TODO: When implemented, test workdir interpolation
	t.Skip("Not yet implemented")
}

// Test specification: Parse should parse environment variables
func TestParseParsesEnv(t *testing.T) {
	// TODO: When implemented, test env var parsing with interpolation
	t.Skip("Not yet implemented")
}

// Test specification: Parse should resolve volume mounts
func TestParseResolvesVolumeMounts(t *testing.T) {
	// TODO: When implemented, test volume mount resolution
	t.Skip("Not yet implemented")
}

// Test specification: Parse should error on missing volume
func TestParseErrorsOnMissingVolume(t *testing.T) {
	// TODO: When implemented, test error when volume not found
	t.Skip("Not yet implemented")
}

// Test specification: Parse should parse port exposures
func TestParseParsesPortExposures(t *testing.T) {
	// TODO: When implemented, test port mapping conversion
	t.Skip("Not yet implemented")
}

// Test specification: Parse should parse health check config
func TestParseParsesHealthCheck(t *testing.T) {
	// TODO: When implemented, test health check config conversion
	t.Skip("Not yet implemented")
}

// Test specification: Parse should parse resource limits
func TestParseParsesResourceLimits(t *testing.T) {
	// TODO: When implemented, test CPU/memory/GPU parsing
	t.Skip("Not yet implemented")
}

// Test specification: Parse should set pull policy
func TestParseSetsPullPolicy(t *testing.T) {
	// TODO: When implemented, test pull policy parsing
	t.Skip("Not yet implemented")
}

// Test specification: Parse should set background flag
func TestParseSetsBackground(t *testing.T) {
	// TODO: When implemented, test background flag
	t.Skip("Not yet implemented")
}

// Test specification: Parse should copy needs from step
func TestParseCopiesNeeds(t *testing.T) {
	// TODO: When implemented, test needs array copy
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle missing image gracefully
func TestParseErrorsOnMissingImage(t *testing.T) {
	// TODO: When implemented, test validation
	t.Skip("Not yet implemented")
}
