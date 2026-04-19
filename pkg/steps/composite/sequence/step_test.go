package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestStepImplementsCompositeStep(t *testing.T) {
	// Compile-time check that Step implements CompositeStep
	var _ steps.CompositeStep = (*Step)(nil)
	var _ steps.Step = (*Step)(nil)
}

func TestNewStep(t *testing.T) {
	childSteps := []steps.Step{
		&mockStep{id: "step1", name: "Step 1"},
		&mockStep{id: "step2", name: "Step 2"},
	}

	step := New("seq1", "My Sequence", childSteps)

	assert.Equal(t, "seq1", step.ID())
	assert.Equal(t, "My Sequence", step.Name())
	assert.Len(t, step.Children(), 2)
}

func TestStepIDAndName(t *testing.T) {
	step := &Step{
		id:   "seq1",
		name: "Test Sequence",
	}

	assert.Equal(t, "seq1", step.ID())
	assert.Equal(t, "Test Sequence", step.Name())
}

func TestStepChildren(t *testing.T) {
	children := []steps.Step{
		&mockStep{id: "step1", name: "Step 1"},
		&mockStep{id: "step2", name: "Step 2"},
	}

	step := &Step{
		id:       "seq1",
		name:     "Test Sequence",
		children: children,
	}

	assert.Equal(t, children, step.Children())
}

func TestStepIterator(t *testing.T) {
	children := []steps.Step{
		&mockStep{id: "step1", name: "Step 1"},
		&mockStep{id: "step2", name: "Step 2"},
	}

	step := New("seq1", "Test Sequence", children)
	context := steps.NewStepContext()

	iterator := step.Iterator(context)
	require.NotNil(t, iterator)
}

// Test specification: Iterator should yield steps one at a time
func TestIteratorYieldsStepsSequentially(t *testing.T) {
	// TODO: When implemented, test that Next() returns one step at a time
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should stop after all steps complete
func TestIteratorStopsWhenDone(t *testing.T) {
	// TODO: When implemented, test that done=true after all steps
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should fail fast on step failure
func TestIteratorFailsFastOnError(t *testing.T) {
	// TODO: When implemented, test that error is returned on failure
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should accumulate outputs from previous steps
func TestIteratorAccumulatesOutputs(t *testing.T) {
	// TODO: When implemented, test output accumulation in scope
	t.Skip("Not yet implemented")
}

// Test specification: Iterator Result should contain all accumulated outputs
func TestIteratorResultContainsOutputs(t *testing.T) {
	// TODO: When implemented, test final result outputs
	t.Skip("Not yet implemented")
}

// Test specification: Empty sequence should return done immediately
func TestEmptySequenceReturnsDone(t *testing.T) {
	// TODO: When implemented, test empty sequence behavior
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should return valid scope after each step
func TestIteratorReturnsValidScope(t *testing.T) {
	// TODO: When implemented, test scope provider interface
	t.Skip("Not yet implemented")
}

// mockStep is a minimal mock for testing
type mockStep struct {
	id   string
	name string
}

func (m *mockStep) ID() string   { return m.id }
func (m *mockStep) Name() string { return m.name }
