package parallel

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
		started: false,
	}

	// TODO: Currently returns nil, true, nil (not implemented)
	steps, done, err := iter.Next(nil)
	assert.Nil(t, steps)
	assert.True(t, done)
	assert.NoError(t, err)
}

func TestIteratorResultMerges(t *testing.T) {
	iter := &Iterator{
		results: []*steps.Result{
			steps.SuccessWithOutputs(map[string]string{"a": "1"}),
			steps.SuccessWithOutputs(map[string]string{"b": "2"}),
		},
	}

	result := iter.Result()
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "1", result.Outputs["a"])
	assert.Equal(t, "2", result.Outputs["b"])
}

func TestIteratorResultWithEmptyResults(t *testing.T) {
	iter := &Iterator{
		results: []*steps.Result{},
	}

	result := iter.Result()
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Empty(t, result.Outputs)
}

// Test specification: First call should return all steps and not done
func TestIteratorFirstCallReturnsAllSteps(t *testing.T) {
	// TODO: When implemented, test initial call returns all children
	t.Skip("Not yet implemented")
}

// Test specification: Second call should check results and return done
func TestIteratorSecondCallReturnsDone(t *testing.T) {
	// TODO: When implemented, test second call behavior
	t.Skip("Not yet implemented")
}

// Test specification: Second call should detect failures in results
func TestIteratorSecondCallDetectsFailures(t *testing.T) {
	// TODO: When implemented, test failure detection
	t.Skip("Not yet implemented")
}

// Test specification: Error should identify which step failed
func TestIteratorErrorIdentifiesFailedStep(t *testing.T) {
	// TODO: When implemented, test step identification in error
	t.Skip("Not yet implemented")
}

// Test specification: started flag should track state
func TestIteratorStartedFlag(t *testing.T) {
	// TODO: When implemented, test started state tracking
	t.Skip("Not yet implemented")
}

// Test specification: results should be stored from lastResults
func TestIteratorStoresResults(t *testing.T) {
	// TODO: When implemented, test result storage
	t.Skip("Not yet implemented")
}
