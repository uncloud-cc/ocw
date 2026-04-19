package parallel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestParseReturnsErrorNotImplemented(t *testing.T) {
	ps := &schema.ParallelStep{
		OptionalStepBase: schema.OptionalStepBase{
			ID:   schema.ID("par1"),
			Name: schema.Name("My Parallel"),
		},
		Parallel: []schema.Step{},
	}

	childSteps := []steps.Step{}

	_, err := Parse(ps, childSteps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// Test specification: Parse should create Step with ID and name
func TestParseCreatesStepWithIDAndName(t *testing.T) {
	// TODO: When implemented, test ID/name assignment
	t.Skip("Not yet implemented")
}

// Test specification: Parse should store child steps
func TestParseStoresChildSteps(t *testing.T) {
	// TODO: When implemented, test child step storage
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle empty parallel
func TestParseHandlesEmptyParallel(t *testing.T) {
	// TODO: When implemented, test empty parallel parsing
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle parallel without ID
func TestParseHandlesMissingID(t *testing.T) {
	// TODO: When implemented, test optional ID
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle parallel without name
func TestParseHandlesMissingName(t *testing.T) {
	// TODO: When implemented, test optional name
	t.Skip("Not yet implemented")
}
