package container

import (
	"io"
	"time"
)

// PullOptions configures image pulling.
type PullOptions struct {
	// Platform is the target platform, e.g., "linux/amd64".
	Platform string
	// Quiet suppresses progress output.
	Quiet bool
}

// CreateOptions configures container creation.
type CreateOptions struct {
	// Image is the container image to use (required).
	Image string
	// Name is an optional container name.
	Name string
	// Cmd is the command to run (overrides image CMD).
	Cmd []string
	// Entrypoint overrides the container's default entrypoint.
	Entrypoint []string
	// Env is a map of environment variables.
	Env map[string]string
	// WorkingDir sets the working directory inside the container.
	WorkingDir string
	// Mounts is a list of volume/bind mounts.
	Mounts []Mount
	// Ports is a list of port mappings.
	Ports []PortMapping
	// Network is the network to attach the container to.
	Network NetworkID
	// NetworkMode sets the network mode: "bridge", "host", "none", or network name.
	NetworkMode string

	// Resource limits
	// CPUs is the number of CPUs to allocate.
	CPUs float64
	// Memory is the memory limit in bytes.
	Memory int64
	// GPUs is the GPU configuration ("all" or specific count).
	GPUs string

	// HealthCheck configures container health checking.
	HealthCheck *HealthCheckConfig

	// TTY allocates a pseudo-TTY.
	TTY bool
	// OpenStdin keeps stdin open even if not attached.
	OpenStdin bool

	// Labels are key-value pairs for container identification.
	Labels map[string]string
}

// Mount represents a volume or bind mount.
type Mount struct {
	// Type is the mount type: "bind" or "volume".
	Type string
	// Source is the host path or volume name.
	Source string
	// Target is the container path where the mount is attached.
	Target string
	// ReadOnly mounts the volume as read-only.
	ReadOnly bool
}

// PortMapping represents a port exposure configuration.
type PortMapping struct {
	// ContainerPort is the port inside the container.
	ContainerPort int
	// HostPort is the port on the host (0 = auto-assign).
	HostPort int
	// Protocol is the network protocol: "tcp" or "udp" (default: "tcp").
	Protocol string
}

// HealthCheckConfig configures container health checking.
type HealthCheckConfig struct {
	// Test is the command to run for health checks.
	Test []string
	// Interval is the time between health checks.
	Interval time.Duration
	// Timeout is the maximum time for a health check to complete.
	Timeout time.Duration
	// Retries is the number of consecutive failures before marking unhealthy.
	Retries int
	// StartPeriod is the grace period before health checks begin.
	StartPeriod time.Duration
}

// BuildOptions configures image building.
type BuildOptions struct {
	// ContextPath is the path to the build context.
	ContextPath string
	// Dockerfile is the path to the Dockerfile (relative to context).
	Dockerfile string
	// Tags are the image tags to apply.
	Tags []string
	// BuildArgs are build-time variables.
	BuildArgs map[string]string
	// Target is the multi-stage build target.
	Target string
	// Platform is a list of target platforms for multi-platform builds.
	Platform []string
	// CacheFrom lists cache sources.
	CacheFrom []string
	// CacheTo lists cache export destinations.
	CacheTo []string
	// NoCache disables the build cache.
	NoCache bool
	// Pull always pulls base images.
	Pull bool
	// Push pushes the image to a registry after building.
	Push bool
	// Load loads the image into the local image store.
	Load bool
	// Labels are metadata labels for the image.
	Labels map[string]string
	// Secrets are secrets available during build.
	Secrets []BuildSecret

	// ProgressWriter receives build progress output.
	ProgressWriter io.Writer
}

// BuildSecret represents a secret available during build.
type BuildSecret struct {
	// ID is the secret identifier.
	ID string
	// Source is the file path or environment variable name.
	Source string
	// IsEnv indicates if Source is an environment variable name.
	IsEnv bool
}

// NetworkOptions configures network creation.
type NetworkOptions struct {
	// Driver is the network driver: "bridge", "overlay", etc.
	Driver string
	// Labels are metadata labels.
	Labels map[string]string
	// Internal restricts external connectivity.
	Internal bool
}

// LogOptions configures log retrieval.
type LogOptions struct {
	// Follow streams logs in real-time.
	Follow bool
	// Timestamps includes timestamps in log output.
	Timestamps bool
	// Since returns logs after this time.
	Since time.Time
	// Until returns logs before this time.
	Until time.Time
	// Tail limits the number of log lines ("all" or number).
	Tail string
}

// AttachOptions configures container attachment.
type AttachOptions struct {
	// Stdin attaches to stdin.
	Stdin bool
	// Stdout attaches to stdout.
	Stdout bool
	// Stderr attaches to stderr.
	Stderr bool
	// Stream enables streaming mode.
	Stream bool
}
