package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uncloud-cc/ocw/pkg/container"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestResolvedVolume(t *testing.T) {
	vol := ResolvedVolume{
		HostPath:  "/home/user/project/data",
		MountPath: "/data",
		ReadOnly:  true,
	}

	assert.Equal(t, "/home/user/project/data", vol.HostPath)
	assert.Equal(t, "/data", vol.MountPath)
	assert.True(t, vol.ReadOnly)
}

func TestResolvedVolumeReadWrite(t *testing.T) {
	vol := ResolvedVolume{
		HostPath:  "/tmp/workspace",
		MountPath: "/workspace",
		ReadOnly:  false,
	}

	assert.Equal(t, "/tmp/workspace", vol.HostPath)
	assert.Equal(t, "/workspace", vol.MountPath)
	assert.False(t, vol.ReadOnly)
}

func TestResolvedVolumeDefaultMountPath(t *testing.T) {
	// Test case showing the default mount path pattern
	vol := ResolvedVolume{
		HostPath:  "/volumes/mydata",
		MountPath: "/volumes/mydata", // Default: same as volume name
		ReadOnly:  false,
	}

	assert.Equal(t, "/volumes/mydata", vol.MountPath)
}

// Test that verifies Executor interface can be implemented
// This is a compile-time check - the test passes if it compiles
func TestExecutorInterface(t *testing.T) {
	// This test ensures the Executor interface is properly defined
	// The actual verification is at compile time - if this compiles, the interface is valid
	var _ Executor = (*mockExecutorForTest)(nil)
	assert.True(t, true) // Test passes if we get here
}

// Test that verifies Logger interface can be implemented
// This is a compile-time check - the test passes if it compiles
func TestLoggerInterface(t *testing.T) {
	var _ Logger = (*MockLogger)(nil)
	assert.True(t, true) // Test passes if we get here
}

// mockExecutorForTest is a minimal mock for compile-time interface check
type mockExecutorForTest struct{}

func (m *mockExecutorForTest) Container() container.Runtime               { return nil }
func (m *mockExecutorForTest) Outputs() *OutputStore                      { return nil }
func (m *mockExecutorForTest) Logger() Logger                             { return nil }
func (m *mockExecutorForTest) WorkDir() string                            { return "" }
func (m *mockExecutorForTest) ResolvedVolumes() map[string]ResolvedVolume { return nil }
func (m *mockExecutorForTest) RegisterService(id string, containerID container.ContainerID, healthCheck *schema.HealthCheck) {
}
func (m *mockExecutorForTest) WaitForServices(ctx context.Context, serviceIDs []string) error {
	return nil
}

// MockLogger is a test implementation of Logger
type MockLogger struct {
	DebugMessages []string
	InfoMessages  []string
	WarnMessages  []string
	ErrorMessages []string
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.DebugMessages = append(m.DebugMessages, msg)
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.InfoMessages = append(m.InfoMessages, msg)
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.WarnMessages = append(m.WarnMessages, msg)
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.ErrorMessages = append(m.ErrorMessages, msg)
}

func (m *MockLogger) WithStep(stepID, stepName string) Logger {
	return m
}

// Verify MockLogger implements Logger interface
var _ Logger = (*MockLogger)(nil)

func TestMockLogger(t *testing.T) {
	logger := &MockLogger{}

	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "key", "value")
	logger.Warn("warn message", "key", "value")
	logger.Error("error message", "key", "value")

	assert.Len(t, logger.DebugMessages, 1)
	assert.Len(t, logger.InfoMessages, 1)
	assert.Len(t, logger.WarnMessages, 1)
	assert.Len(t, logger.ErrorMessages, 1)

	assert.Contains(t, logger.DebugMessages[0], "debug message")
	assert.Contains(t, logger.InfoMessages[0], "info message")
	assert.Contains(t, logger.WarnMessages[0], "warn message")
	assert.Contains(t, logger.ErrorMessages[0], "error message")
}

func TestMockLoggerWithStep(t *testing.T) {
	logger := &MockLogger{}
	scopedLogger := logger.WithStep("step1", "Build Image")

	// Verify WithStep returns a Logger (can be the same or a new one)
	assert.NotNil(t, scopedLogger)

	// The returned logger should still work
	scopedLogger.Info("scoped message")
	assert.Len(t, logger.InfoMessages, 1)
}
