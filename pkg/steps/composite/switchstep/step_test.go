package switchstep

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
	branches := []Branch{
		{Value: "linux", Steps: []steps.Step{&mockStep{id: "build-linux", name: "Build Linux"}}},
		{Value: "windows", Steps: []steps.Step{&mockStep{id: "build-windows", name: "Build Windows"}}},
	}
	defaultSteps := []steps.Step{&mockStep{id: "build-default", name: "Build Default"}}

	step := New("switch1", "Platform Switch", "linux", branches, defaultSteps)

	assert.Equal(t, "switch1", step.ID())
	assert.Equal(t, "Platform Switch", step.Name())
	assert.Equal(t, "linux", step.value)
	assert.Len(t, step.branches, 2)
	assert.Len(t, step.defaultSteps, 1)
}

func TestStepIDAndName(t *testing.T) {
	step := &Step{
		id:   "switch1",
		name: "Test Switch",
	}

	assert.Equal(t, "switch1", step.ID())
	assert.Equal(t, "Test Switch", step.Name())
}

func TestStepChildren(t *testing.T) {
	branches := []Branch{
		{Value: "a", Steps: []steps.Step{&mockStep{id: "step1", name: "Step 1"}}},
		{Value: "b", Steps: []steps.Step{&mockStep{id: "step2", name: "Step 2"}}},
	}
	defaultSteps := []steps.Step{&mockStep{id: "step3", name: "Step 3"}}

	step := &Step{
		id:           "switch1",
		name:         "Test Switch",
		branches:     branches,
		defaultSteps: defaultSteps,
	}

	children := step.Children()
	assert.Len(t, children, 3)
}

func TestStepChildrenEmpty(t *testing.T) {
	step := &Step{
		id:       "switch1",
		name:     "Test Switch",
		branches: []Branch{},
	}

	children := step.Children()
	assert.Empty(t, children)
}

func TestStepIterator(t *testing.T) {
	step := New("switch1", "Test", "value", []Branch{}, nil)
	context := steps.NewStepContext()

	iterator := step.Iterator(context)
	require.NotNil(t, iterator)
}

// Test specification: Iterator should select matching branch
func TestIteratorSelectsMatchingBranch(t *testing.T) {
	// TODO: When implemented, test branch selection
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should fall back to default if no match
func TestIteratorFallsBackToDefault(t *testing.T) {
	// TODO: When implemented, test default branch
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should return empty if no match and no default
func TestIteratorReturnsEmptyIfNoMatchOrDefault(t *testing.T) {
	// TODO: When implemented, test empty behavior
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should execute branch steps sequentially
func TestIteratorExecutesBranchSequentially(t *testing.T) {
	// TODO: When implemented, test sequential execution
	t.Skip("Not yet implemented")
}

// Test specification: Iterator should fail fast on step failure
func TestIteratorFailsFastOnError(t *testing.T) {
	// TODO: When implemented, test failure behavior
	t.Skip("Not yet implemented")
}

// Test specification: Branch should store value and steps
type BranchStoresValueAndSteps struct{}

func (b *BranchStoresValueAndSteps) Test(t *testing.T) {
	branch := Branch{
		Value: "test",
		Steps: []steps.Step{&mockStep{id: "step1", name: "Step 1"}},
	}

	assert.Equal(t, "test", branch.Value)
	assert.Len(t, branch.Steps, 1)
}

// mockStep is a minimal mock for testing
type mockStep struct {
	id   string
	name string
}

func (m *mockStep) ID() string   { return m.id }
func (m *mockStep) Name() string { return m.name }
