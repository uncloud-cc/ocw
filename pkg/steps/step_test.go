package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uncloud-cc/ocw/pkg/container"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// === Interface Compliance Tests ===
// These tests verify that the interfaces are properly defined

// TestStepInterface verifies Step interface can be implemented
func TestStepInterface(t *testing.T) {
	// Mock implementation
	mockStep := &mockStepImpl{
		id:   "step1",
		name: "Test Step",
	}

	var _ Step = mockStep
	assert.Equal(t, "step1", mockStep.ID())
	assert.Equal(t, "Test Step", mockStep.Name())
}

// TestSimpleStepInterface verifies SimpleStep interface can be implemented
func TestSimpleStepInterface(t *testing.T) {
	mockStep := &mockSimpleStepImpl{
		mockStepImpl: mockStepImpl{
			id:   "run1",
			name: "Run Container",
		},
	}

	var _ Step = mockStep
	var _ SimpleStep = mockStep

	// Execute method can be called (returns error since it's not implemented)
	_, err := mockStep.Execute(context.Background(), nil)
	assert.Error(t, err) // Mock returns error
}

// TestCompositeStepInterface verifies CompositeStep interface can be implemented
func TestCompositeStepInterface(t *testing.T) {
	mockStep := &mockCompositeStepImpl{
		mockStepImpl: mockStepImpl{
			id:   "seq1",
			name: "Sequence",
		},
		children: []Step{},
	}

	var _ Step = mockStep
	var _ CompositeStep = mockStep

	assert.Empty(t, mockStep.Children())

	// Iterator can be called
	iter := mockStep.Iterator(NewStepContext())
	assert.NotNil(t, iter)
}

// TestStepIteratorInterface verifies StepIterator interface can be implemented
func TestStepIteratorInterface(t *testing.T) {
	iter := &mockIteratorImpl{}

	var _ StepIterator = iter

	// Test Next method
	steps, done, err := iter.Next(nil)
	assert.Empty(t, steps)
	assert.True(t, done)
	assert.NoError(t, err)

	// Test Result method
	result := iter.Result()
	assert.NotNil(t, result)
}

// === Mock Implementations ===

type mockStepImpl struct {
	id   string
	name string
}

func (m *mockStepImpl) ID() string   { return m.id }
func (m *mockStepImpl) Name() string { return m.name }

type mockSimpleStepImpl struct {
	mockStepImpl
}

func (m *mockSimpleStepImpl) Execute(ctx context.Context, exec Executor) (*Result, error) {
	return nil, assert.AnError
}

type mockCompositeStepImpl struct {
	mockStepImpl
	children []Step
}

func (m *mockCompositeStepImpl) Children() []Step {
	return m.children
}

func (m *mockCompositeStepImpl) Iterator(stepContext *StepContext) StepIterator {
	return &mockIteratorImpl{}
}

type mockIteratorImpl struct{}

func (m *mockIteratorImpl) Next(lastResults []*Result) ([]Step, bool, error) {
	return nil, true, nil
}

func (m *mockIteratorImpl) Result() *Result {
	return Success()
}

// === Type Assertions ===
// These ensure interfaces are defined correctly at compile time

func TestStepInterfaceTypes(t *testing.T) {
	// Ensure Step is an interface
	var _ Step = (*mockStepImpl)(nil)
}

func TestSimpleStepInterfaceTypes(t *testing.T) {
	// SimpleStep embeds Step
	var _ Step = (*mockSimpleStepImpl)(nil)
	var _ SimpleStep = (*mockSimpleStepImpl)(nil)
}

func TestCompositeStepInterfaceTypes(t *testing.T) {
	// CompositeStep embeds Step
	var _ Step = (*mockCompositeStepImpl)(nil)
	var _ CompositeStep = (*mockCompositeStepImpl)(nil)
}

// === Verify Interfaces Have Expected Methods ===

func TestStepHasRequiredMethods(t *testing.T) {
	// This test documents the expected methods
	// If the interface changes, this will fail to compile
	var step Step = &mockStepImpl{}
	_ = step.ID()
	_ = step.Name()
}

func TestSimpleStepHasRequiredMethods(t *testing.T) {
	var step SimpleStep = &mockSimpleStepImpl{}
	_ = step.ID()
	_ = step.Name()
	_, _ = step.Execute(context.Background(), nil)
}

func TestCompositeStepHasRequiredMethods(t *testing.T) {
	var step CompositeStep = &mockCompositeStepImpl{
		children: []Step{&mockStepImpl{}},
	}
	_ = step.ID()
	_ = step.Name()
	_ = step.Children()
	_ = step.Iterator(NewStepContext())
}

func TestStepIteratorHasRequiredMethods(t *testing.T) {
	var iter StepIterator = &mockIteratorImpl{}
	_, _, _ = iter.Next(nil)
	_ = iter.Result()
}

// === Executor Interface Test ===
// This tests that Executor can be implemented (compile-time check)

func TestExecutorCanBeImplemented(t *testing.T) {
	// This test will fail to compile if Executor interface is invalid
	var _ Executor = (*mockExecutorImpl)(nil)
}

type mockExecutorImpl struct{}

func (m *mockExecutorImpl) Container() container.Runtime {
	return nil
}

func (m *mockExecutorImpl) Outputs() *OutputStore {
	return nil
}

func (m *mockExecutorImpl) Logger() Logger {
	return nil
}

func (m *mockExecutorImpl) WorkDir() string {
	return ""
}

func (m *mockExecutorImpl) ResolvedVolumes() map[string]ResolvedVolume {
	return nil
}

func (m *mockExecutorImpl) RegisterService(id string, containerID container.ContainerID, healthCheck *schema.HealthCheck) {
}

func (m *mockExecutorImpl) WaitForServices(ctx context.Context, serviceIDs []string) error {
	return nil
}
