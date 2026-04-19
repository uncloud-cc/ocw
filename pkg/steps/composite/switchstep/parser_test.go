package switchstep

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestParseReturnsErrorNotImplemented(t *testing.T) {
	ss := &schema.SwitchStep{
		OptionalStepBase: schema.OptionalStepBase{
			ID:   schema.ID("switch1"),
			Name: schema.Name("My Switch"),
		},
		Switch: "{{ inputs.platform }}",
		Case:   map[string]schema.StepOrSteps{},
	}

	branches := map[string][]steps.Step{}
	var defaultSteps []steps.Step

	_, err := Parse(ss, "linux", branches, defaultSteps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// Test specification: Parse should create Step with resolved value
func TestParseCreatesStepWithValue(t *testing.T) {
	// TODO: When implemented, test value assignment
	t.Skip("Not yet implemented")
}

// Test specification: Parse should convert branches map to Branch slice
func TestParseConvertsBranches(t *testing.T) {
	// TODO: When implemented, test branch conversion
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle default branch
func TestParseHandlesDefault(t *testing.T) {
	// TODO: When implemented, test default handling
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle empty default
func TestParseHandlesEmptyDefault(t *testing.T) {
	// TODO: When implemented, test nil default
	t.Skip("Not yet implemented")
}

// Test specification: Parse should set ID and name
func TestParseSetsIDAndName(t *testing.T) {
	// TODO: When implemented, test ID/name assignment
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle switch without ID
func TestParseHandlesMissingID(t *testing.T) {
	// TODO: When implemented, test optional ID
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle switch without name
func TestParseHandlesMissingName(t *testing.T) {
	// TODO: When implemented, test optional name
	t.Skip("Not yet implemented")
}

// Test specification: Parse should validate that value is resolved
func TestParseValidatesResolvedValue(t *testing.T) {
	// TODO: When implemented, test that value doesn't contain templates
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle empty case map
func TestParseHandlesEmptyCase(t *testing.T) {
	// TODO: When implemented, test empty cases
	t.Skip("Not yet implemented")
}
