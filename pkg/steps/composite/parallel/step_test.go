package parallel

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

	step := New("par1", "My Parallel", childSteps)

	assert.Equal(t, "par1", step.ID())
	assert.Equal(t, "My Parallel", step.Name())
	assert.Len(t, step.Children(), 2)
}

func TestStepIDAndName(t *testing.T) {
	step := &Step{
		id:   "par1",
		name: "Test Parallel",
	}

	assert.Equal(t, "par1", step.ID())
	assert.Equal(t, "Test Parallel", step.Name())
}

func TestStepChildren(t *testing.T) {
	children := []steps.Step{
		&mockStep{id: "step1", name: "Step 1"},
		&mockStep{id: "step2", name: "Step 2"},
	}

	step := &Step{
		id:       "par1",
		name:     "Test Parallel",
		children: children,
	}

	assert.Equal(t, children, step.Children())
}

func TestStepIterator(t *testing.T) {
	children := []steps.Step{
		&mockStep{id: "step1", name: "Step 1"},
		&mockStep{id: "step2", name: "Step 2"},
	}

	step := New("par1", "Test Parallel", children)
	context := steps.NewStepContext()

	iterator := step.Iterator(context)
	require.NotNil(t, iterator)
}

// Test specification: Iterator should yield all steps on first call
func TestIteratorYieldsAllStepsInitially(t *testing.T) {
	// TODO: When implemented, test that Next() returns all steps on first call
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should return done=true on second call
func TestIteratorReturnsDoneAfterFirstCall(t *testing.T) {
	// TODO: When implemented, test done=true behavior
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should detect any step failures
func TestIteratorDetectsFailures(t *testing.T) {
	// TODO: When implemented, test failure detection in parallel
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should include step info in error
func TestIteratorErrorIncludesStepInfo(t *testing.T) {
	// TODO: When implemented, test error context
	t.Skip("Not yet implemented")
}

// Test specification: Iterator Result should merge all outputs
func TestIteratorResultMergesOutputs(t *testing.T) {
	// TODO: When implemented, test output merging
	t.Skip("Not yet implemented")
}

// Test specification: Empty parallel should return done immediately
func TestEmptyParallelReturnsDone(t *testing.T) {
	// TODO: When implemented, test empty parallel behavior
	t.Skip("Not yet implemented")
}

// Test specification: Single child should work correctly
func TestSingleChildWorks(t *testing.T) {
	// TODO: When implemented, test single child case
	t.Skip("Not yet implemented")
}

// mockStep is a minimal mock for testing
type mockStep struct {
	id   string
	name string
}

func (m *mockStep) ID() string   { return m.id }
func (m *mockStep) Name() string { return m.name }
