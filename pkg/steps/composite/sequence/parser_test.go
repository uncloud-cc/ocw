package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestParseReturnsErrorNotImplemented(t *testing.T) {
	ss := &schema.SequenceStep{
		OptionalStepBase: schema.OptionalStepBase{
			ID:   schema.ID("seq1"),
			Name: schema.Name("My Sequence"),
		},
		Sequence: []schema.Step{},
	}

	childSteps := []steps.Step{}

	_, err := Parse(ss, childSteps)
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

// Test specification: Parse should handle empty sequence
func TestParseHandlesEmptySequence(t *testing.T) {
	// TODO: When implemented, test empty sequence parsing
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle sequence without ID
func TestParseHandlesMissingID(t *testing.T) {
	// TODO: When implemented, test optional ID
	t.Skip("Not yet implemented")
}

// Test specification: Parse should handle sequence without name
func TestParseHandlesMissingName(t *testing.T) {
	// TODO: When implemented, test optional name
	t.Skip("Not yet implemented")
}
