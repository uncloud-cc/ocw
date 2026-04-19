package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestIteratorImplementsStepIterator(t *testing.T) {
	// Compile-time check that Iterator implements StepIterator
	var _ steps.StepIterator = (*Iterator)(nil)
}

func TestIteratorReturnsNotImplemented(t *testing.T) {
	iter := &Iterator{
		steps:   []steps.Step{},
		context: steps.NewStepContext(),
		index:   0,
		outputs: make(map[string]string),
	}

	// TODO: Currently returns nil, true, nil (not implemented)
	// Will need to update when implemented
	steps, done, err := iter.Next(nil)
	assert.Nil(t, steps)
	assert.True(t, done)
	assert.NoError(t, err)
}

func TestIteratorResultReturnsSuccess(t *testing.T) {
	iter := &Iterator{
		outputs: map[string]string{"key": "value"},
	}

	result := iter.Result()
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "value", result.Outputs["key"])
}

// Test specification: Next should return first step on first call
func TestIteratorReturnsFirstStepInitially(t *testing.T) {
	// TODO: When implemented, test initial call
	t.Skip("Not yet implemented")
}

// Test specification: Next should use previous results to update scope
func TestIteratorUpdatesScopeFromResults(t *testing.T) {
	// TODO: When implemented, test scope update with results
	t.Skip("Not yet implemented")
}

// Test specification: Next should check for step failure in results
func TestIteratorChecksForFailures(t *testing.T) {
	// TODO: When implemented, test failure detection
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should track step index correctly
func TestIteratorTracksIndex(t *testing.T) {
	// TODO: When implemented, test index progression
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should store outputs by step ID
func TestIteratorStoresStepOutputs(t *testing.T) {
	// TODO: When implemented, test output storage by ID
	t.Skip("Not yet implemented")
}
