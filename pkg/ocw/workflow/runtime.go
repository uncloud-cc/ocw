package workflow

import (
	"context"
	"io"
)

// -----------------------------------------------------------------------------
// Container Runtime Interface
// -----------------------------------------------------------------------------

// ContainerRuntime abstracts container operations.
// Implemented by the CLI for Docker, Podman, etc.
type ContainerRuntime interface {
	// Run executes a container and waits for completion (unless background)
	Run(ctx context.Context, opts RunOptions) (*RunResult, error)

	// Build builds a container image
	Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)

	// Stop stops a running container
	Stop(ctx context.Context, containerID string) error

	// Logs streams logs from a container
	Logs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error)
}

// RunOptions configures a container run.
type RunOptions struct {
	Image       string
	Cmd         string
	Args        []string
	Entrypoint  string
	Workdir     string
	Env         map[string]string
	Volumes     []VolumeMount
	Background  bool
	HealthCheck *HealthCheckConfig
	Expose      []PortMapping
	TTY         bool

	// Resource limits
	CPUs   string
	Memory string
	GPUs   string

	// Network
	NetworkID string
	Hostname  string
}

// VolumeMount represents a volume mount.
type VolumeMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// HealthCheckConfig for background containers.
type HealthCheckConfig struct {
	Cmd         string
	Interval    string
	Timeout     string
	Retries     int
	StartPeriod string
}

// RunResult contains the result of running a container.
type RunResult struct {
	ContainerID string
	ExitCode    int
	Outputs     map[string]string // Parsed from container's output mechanism
}

// BuildOptions configures an image build.
type BuildOptions struct {
	Context    string
	Dockerfile string
	Target     string
	BuildArgs  map[string]string
	Tags       []string
	Platform   []string
	CacheFrom  []string
	CacheTo    []string
	Push       bool
	Load       bool
}

// BuildResult contains the result of building an image.
type BuildResult struct {
	ImageID  string
	ImageRef string // Full image reference (name:tag)
	Digest   string
}
