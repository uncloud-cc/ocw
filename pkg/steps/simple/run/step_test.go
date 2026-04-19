package run

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/container"
	"github.com/uncloud-cc/ocw/pkg/container/mock"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

func TestStepImplementsSimpleStep(t *testing.T) {
	// Compile-time check that Step implements SimpleStep
	var _ steps.SimpleStep = (*Step)(nil)
	var _ steps.Step = (*Step)(nil)
}

func TestStepIDAndName(t *testing.T) {
	step := &Step{
		id:   "step1",
		name: "Run Test",
	}

	assert.Equal(t, "step1", step.ID())
	assert.Equal(t, "Run Test", step.Name())
}

func TestStepExecuteNotImplemented(t *testing.T) {
	step := &Step{
		id:         "run1",
		name:       "Run Container",
		image:      "alpine:latest",
		cmd:        []string{"echo", "hello"},
		pullPolicy: "missing",
	}

	mockExec := &mockExecutor{}
	ctx := context.Background()

	_, err := step.Execute(ctx, mockExec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

// Test specification: Execute should pull image if missing
func TestStepExecutePullsImageIfMissing(t *testing.T) {
	// TODO: When implemented, test that image is pulled when pullPolicy is "missing"
	t.Skip("Not yet implemented")
}

// Test specification: Execute should wait for dependencies (needs)
func TestStepExecuteWaitsForDependencies(t *testing.T) {
	// TODO: When implemented, test that step waits for services in needs
	t.Skip("Not yet implemented")
}

// Test specification: Execute should create and start container
func TestStepExecuteCreatesAndStartsContainer(t *testing.T) {
	// TODO: When implemented, test container creation flow
	t.Skip("Not yet implemented")
}

// Test specification: Execute should return success result for foreground container
func TestStepExecuteReturnsSuccessForForeground(t *testing.T) {
	// TODO: When implemented, test success result with exit code 0
	t.Skip("Not yet implemented")
}

// Test specification: Execute should return failed result for failed container
func TestStepExecuteReturnsFailedForErrorExit(t *testing.T) {
	// TODO: When implemented, test failed result with non-zero exit code
	t.Skip("Not yet implemented")
}

// Test specification: Execute should handle background containers
func TestStepExecuteHandlesBackground(t *testing.T) {
	// TODO: When implemented, test background service registration
	t.Skip("Not yet implemented")
}

// Test specification: Execute should cleanup container after execution
func TestStepExecuteCleansUpContainer(t *testing.T) {
	// TODO: When implemented, test container removal
	t.Skip("Not yet implemented")
}

// Test specification: Execute should apply environment variables
func TestStepExecuteAppliesEnv(t *testing.T) {
	// TODO: When implemented, test env var application
	t.Skip("Not yet implemented")
}

// Test specification: Execute should apply volume mounts
func TestStepExecuteAppliesMounts(t *testing.T) {
	// TODO: When implemented, test volume mounting
	t.Skip("Not yet implemented")
}

// Test specification: Execute should apply resource limits
func TestStepExecuteAppliesResourceLimits(t *testing.T) {
	// TODO: When implemented, test CPU/memory/GPU limits
	t.Skip("Not yet implemented")
}

// Test specification: Execute should apply port mappings
func TestStepExecuteAppliesPortMappings(t *testing.T) {
	// TODO: When implemented, test port exposure
	t.Skip("Not yet implemented")
}

// mockExecutor is a minimal mock for testing
type mockExecutor struct {
	mockRuntime *mock.Runtime
}

func (m *mockExecutor) Container() container.Runtime {
	return m.mockRuntime
}

func (m *mockExecutor) Outputs() *steps.OutputStore {
	return steps.NewOutputStore()
}

func (m *mockExecutor) Logger() steps.Logger {
	return nil
}

func (m *mockExecutor) WorkDir() string {
	return "/workspace"
}

func (m *mockExecutor) ResolvedVolumes() map[string]steps.ResolvedVolume {
	return map[string]steps.ResolvedVolume{
		"workspace": {HostPath: "/workspace", MountPath: "/workspace", ReadOnly: false},
	}
}

func (m *mockExecutor) RegisterService(id string, containerID container.ContainerID, healthCheck *schema.HealthCheck) {
}

func (m *mockExecutor) WaitForServices(ctx context.Context, serviceIDs []string) error {
	return nil
}
