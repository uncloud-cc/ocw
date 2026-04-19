// Package container defines the abstract interface for container operations.
// Implementations are provided externally (Docker, Podman, etc.).
package container

import "io"

// ContainerID is an opaque identifier for a container.
type ContainerID string

// ImageID is an opaque identifier for an image.
type ImageID string

// VolumeID is an opaque identifier for a volume.
type VolumeID string

// NetworkID is an opaque identifier for a network.
type NetworkID string

// ExitResult contains the result of a container execution.
type ExitResult struct {
	// StatusCode is the container's exit code (0 = success).
	StatusCode int
	// OOMKilled indicates if the container was killed due to out-of-memory.
	OOMKilled bool
	// Error contains any error message if the container failed to run.
	Error string
}

// ContainerInfo contains information about a running or exited container.
type ContainerInfo struct {
	// ID is the container's unique identifier.
	ID ContainerID
	// Name is the container's human-readable name.
	Name string
	// Image is the container's image name.
	Image string
	// Status is the container's current status: "created", "running", "paused", "exited", "dead".
	Status string
	// Health is the health check status: "healthy", "unhealthy", "starting", or "" (no healthcheck).
	Health string
	// Ports lists the container's port bindings.
	Ports []PortBinding
}

// PortBinding represents a port mapping between container and host.
type PortBinding struct {
	// ContainerPort is the port inside the container.
	ContainerPort int
	// HostPort is the port on the host machine.
	HostPort int
	// Protocol is the network protocol: "tcp" or "udp".
	Protocol string
}

// Streams represents attached container I/O streams.
type Streams struct {
	// Stdin is the write end of the container's stdin.
	Stdin io.WriteCloser
	// Stdout is the read end of the container's stdout.
	Stdout io.ReadCloser
	// Stderr is the read end of the container's stderr.
	Stderr io.ReadCloser
}
