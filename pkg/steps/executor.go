package steps

import (
	"context"

	"github.com/uncloud-cc/ocw/pkg/container"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// Executor provides the execution environment for simple steps.
// It is passed to SimpleStep.Execute() and provides access to:
//   - The container runtime
//   - Output storage for step communication
//   - Logging
//   - Service registration for background containers
type Executor interface {
	// Container returns the container runtime.
	Container() container.Runtime

	// Outputs returns the output store for reading/writing step outputs.
	Outputs() *OutputStore

	// Logger returns the logger for this execution.
	Logger() Logger

	// WorkDir returns the workflow's working directory (where the OCW file is).
	WorkDir() string

	// ResolvedVolumes returns the resolved volume definitions.
	// Key is volume name, value is the resolved host path and mount settings.
	ResolvedVolumes() map[string]ResolvedVolume

	// RegisterService registers a background container as a service.
	// The runtime will track its health and clean it up when the job completes.
	RegisterService(id string, containerID container.ContainerID, healthCheck *schema.HealthCheck)

	// WaitForServices waits for the specified service IDs to become healthy.
	// Used to implement the "needs" field on steps.
	WaitForServices(ctx context.Context, serviceIDs []string) error
}

// ResolvedVolume contains a fully resolved volume ready for mounting.
type ResolvedVolume struct {
	// HostPath is the resolved host filesystem path.
	HostPath string
	// MountPath is the default mount path inside the container.
	MountPath string
	// ReadOnly indicates if the volume should be mounted read-only.
	ReadOnly bool
}

// Logger provides logging capabilities for steps.
type Logger interface {
	// Debug logs a debug message.
	Debug(msg string, args ...any)
	// Info logs an info message.
	Info(msg string, args ...any)
	// Warn logs a warning message.
	Warn(msg string, args ...any)
	// Error logs an error message.
	Error(msg string, args ...any)

	// WithStep returns a logger scoped to a specific step.
	WithStep(stepID, stepName string) Logger
}
