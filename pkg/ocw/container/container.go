package container

import (
	"context"
	"io"
	"time"
)

// ContainerManager manages the lifecycle of containers.
type ContainerManager interface {
	// Create creates a container without starting it.
	Create(ctx context.Context, opts CreateOptions) (Container, error)

	// Get retrieves an existing container by ID or name.
	Get(ctx context.Context, idOrName string) (Container, error)

	// List returns containers matching the given filters.
	List(ctx context.Context, opts ListOptions) ([]Container, error)
}

// Container represents a container instance.
type Container interface {
	// ID returns the unique identifier of this container.
	ID() string

	// Name returns the container name (if assigned).
	Name() string

	// Start starts a created container.
	Start(ctx context.Context) error

	// Stop gracefully stops the container.
	// If timeout is zero, a default timeout is used.
	Stop(ctx context.Context, timeout time.Duration) error

	// Kill forcefully terminates the container with a signal.
	// If signal is empty, SIGKILL is used.
	Kill(ctx context.Context, signal string) error

	// Restart stops and starts the container.
	// If timeout is zero, a default timeout is used for the stop operation.
	Restart(ctx context.Context, timeout time.Duration) error

	// Remove removes the container.
	// If force is true, the container is killed if running.
	Remove(ctx context.Context, force bool) error

	// Wait blocks until the container exits and returns its exit code.
	Wait(ctx context.Context) (int, error)

	// Logs returns readers for stdout and stderr.
	// The caller is responsible for closing the readers.
	Logs(ctx context.Context, opts LogOptions) (io.ReadCloser, error)

	// Exec runs a command inside the container.
	Exec(ctx context.Context, opts ExecOptions) (ExecResult, error)

	// Inspect returns detailed information about the container.
	Inspect(ctx context.Context) (ContainerInfo, error)
}

// CreateOptions configures container creation.
type CreateOptions struct {
	// Name assigns a name to the container. If empty, one is generated.
	Name string

	// Image is the container image to use (required).
	Image string

	// Command overrides the image's default command.
	Command []string

	// Entrypoint overrides the image's entrypoint.
	Entrypoint []string

	// Workdir sets the working directory inside the container.
	Workdir string

	// Env are environment variables to set in the container.
	Env map[string]string

	// Mounts are filesystem mounts into the container.
	Mounts []Mount

	// Networks to attach the container to.
	Networks []string

	// NetworkAliases are DNS names for this container within networks.
	NetworkAliases map[string][]string // network name -> aliases

	// ExposedPorts are ports to expose from the container.
	ExposedPorts []PortBinding

	// Resources constrains CPU, memory, and other resources.
	Resources ResourceLimits

	// Healthcheck configures container health monitoring.
	Healthcheck *Healthcheck

	// TTY allocates a pseudo-TTY for the container.
	TTY bool

	// Stdin keeps stdin open even if not attached.
	OpenStdin bool

	// Labels are metadata key-value pairs attached to the container.
	Labels map[string]string

	// StopTimeout is the default timeout for stop operations.
	StopTimeout time.Duration

	// AutoRemove automatically removes the container when it exits.
	AutoRemove bool
}

// ListOptions configures container listing.
type ListOptions struct {
	// All includes stopped containers. By default, only running containers are listed.
	All bool

	// Filters restricts the list to containers matching these criteria.
	// Common filters: "name", "label", "status", "ancestor" (image).
	Filters map[string][]string

	// Limit restricts the number of containers returned.
	Limit int
}

// LogOptions configures log retrieval.
type LogOptions struct {
	// Stdout includes stdout in the output.
	Stdout bool

	// Stderr includes stderr in the output.
	Stderr bool

	// Follow streams logs in real-time.
	Follow bool

	// Timestamps prefixes each log line with its timestamp.
	Timestamps bool

	// Since returns logs after this time.
	Since time.Time

	// Until returns logs before this time.
	Until time.Time

	// Tail returns only the last N lines. "all" or empty returns all lines.
	Tail string
}

// ExecOptions configures command execution inside a container.
type ExecOptions struct {
	// Command is the command to run (required).
	Command []string

	// Workdir is the working directory for the command.
	Workdir string

	// Env are additional environment variables.
	Env []string

	// TTY allocates a pseudo-TTY.
	TTY bool

	// Stdin provides input to the command.
	Stdin io.Reader

	// Stdout receives command output.
	Stdout io.Writer

	// Stderr receives command error output.
	Stderr io.Writer

	// Detach runs the command in the background.
	Detach bool

	// User runs the command as this user.
	User string
}

// ExecResult contains the result of an exec operation.
type ExecResult struct {
	// ExitCode is the command's exit code.
	ExitCode int
}

// Mount describes a filesystem mount into a container.
type Mount struct {
	// Type is the mount type: "bind", "volume", or "tmpfs".
	Type MountType

	// Source is the host path (for bind) or volume name (for volume).
	Source string

	// Target is the path inside the container.
	Target string

	// ReadOnly makes the mount read-only.
	ReadOnly bool
}

// MountType identifies the type of mount.
type MountType string

const (
	MountTypeBind   MountType = "bind"
	MountTypeVolume MountType = "volume"
	MountTypeTmpfs  MountType = "tmpfs"
)

// PortBinding maps a container port to a host port.
type PortBinding struct {
	// ContainerPort is the port inside the container.
	ContainerPort int

	// HostPort is the port on the host. Zero means auto-assign.
	HostPort int

	// HostIP is the host IP to bind to. Empty means all interfaces.
	HostIP string

	// Protocol is "tcp" or "udp". Defaults to "tcp".
	Protocol string
}

// ResourceLimits constrains container resource usage.
type ResourceLimits struct {
	// CPUs is the number of CPUs (can be fractional, e.g., 0.5).
	CPUs float64

	// Memory is the memory limit in bytes.
	Memory int64

	// MemorySwap is the total memory limit (memory + swap) in bytes.
	// -1 means unlimited swap.
	MemorySwap int64

	// GPUs is the number of GPUs to allocate ("all" or a number).
	GPUs string

	// PidsLimit is the maximum number of processes.
	PidsLimit int64
}

// Healthcheck configures container health monitoring.
type Healthcheck struct {
	// Cmd is the command to run for health checks.
	Cmd []string

	// Interval is the time between health checks.
	Interval time.Duration

	// Timeout is the maximum time for a health check to complete.
	Timeout time.Duration

	// Retries is the number of consecutive failures before unhealthy.
	Retries int

	// StartPeriod is the grace period before health checks begin.
	StartPeriod time.Duration
}

// ContainerInfo contains detailed container metadata.
type ContainerInfo struct {
	// ID is the container identifier.
	ID string

	// Name is the container name.
	Name string

	// Image is the image the container was created from.
	Image string

	// State is the current container state.
	State ContainerState

	// ExitCode is set when the container has exited.
	ExitCode int

	// CreatedAt is when the container was created.
	CreatedAt time.Time

	// StartedAt is when the container started.
	StartedAt time.Time

	// FinishedAt is when the container stopped (if applicable).
	FinishedAt time.Time

	// Health contains health check status if configured.
	Health *HealthStatus

	// Mounts lists the container's mounts.
	Mounts []Mount

	// NetworkSettings contains network configuration.
	NetworkSettings NetworkSettings

	// Labels are the container's metadata.
	Labels map[string]string
}

// ContainerState represents the lifecycle state of a container.
type ContainerState string

const (
	ContainerStateCreated    ContainerState = "created"
	ContainerStateRunning    ContainerState = "running"
	ContainerStatePaused     ContainerState = "paused"
	ContainerStateRestarting ContainerState = "restarting"
	ContainerStateExited     ContainerState = "exited"
	ContainerStateDead       ContainerState = "dead"
)

// HealthStatus represents container health check results.
type HealthStatus struct {
	// Status is "starting", "healthy", or "unhealthy".
	Status string

	// FailingStreak is the number of consecutive failures.
	FailingStreak int

	// Log contains recent health check results.
	Log []HealthCheckResult
}

// HealthCheckResult is a single health check execution result.
type HealthCheckResult struct {
	Start    time.Time
	End      time.Time
	ExitCode int
	Output   string
}

// NetworkSettings contains container network information.
type NetworkSettings struct {
	// IPAddress is the container's primary IP address.
	IPAddress string

	// Networks maps network names to their settings.
	Networks map[string]ContainerNetworkSettings

	// Ports maps container ports to host bindings.
	Ports map[string][]PortBinding
}

// ContainerNetworkSettings contains per-network settings for a container.
type ContainerNetworkSettings struct {
	// NetworkID is the network's ID.
	NetworkID string

	// IPAddress is the container's IP on this network.
	IPAddress string

	// Gateway is the gateway for this network.
	Gateway string

	// Aliases are the DNS aliases for this container on the network.
	Aliases []string
}
