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

// ContainerStatus represents the current state of a container.
type ContainerStatus string

const (
	// StatusCreated indicates the container has been created but not started.
	StatusCreated ContainerStatus = "created"
	// StatusRunning indicates the container is currently running.
	StatusRunning ContainerStatus = "running"
	// StatusPaused indicates the container is paused.
	StatusPaused ContainerStatus = "paused"
	// StatusRestarting indicates the container is restarting.
	StatusRestarting ContainerStatus = "restarting"
	// StatusExited indicates the container has exited.
	StatusExited ContainerStatus = "exited"
	// StatusDead indicates the container is dead (failed to stop cleanly).
	StatusDead ContainerStatus = "dead"
)

// HealthStatus represents the health check status of a container.
type HealthStatus string

const (
	// HealthNone indicates the container has no health check configured.
	HealthNone HealthStatus = ""
	// HealthStarting indicates the health check is still in the start period.
	HealthStarting HealthStatus = "starting"
	// HealthHealthy indicates the health check is passing.
	HealthHealthy HealthStatus = "healthy"
	// HealthUnhealthy indicates the health check is failing.
	HealthUnhealthy HealthStatus = "unhealthy"
)

// Protocol represents a network protocol.
type Protocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "tcp"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "udp"
)

// MountType represents the type of a volume mount.
type MountType string

const (
	// MountTypeBind is a bind mount from the host filesystem.
	MountTypeBind MountType = "bind"
	// MountTypeVolume is a Docker-managed volume.
	MountTypeVolume MountType = "volume"
	// MountTypeTmpfs is a tmpfs mount (in-memory).
	MountTypeTmpfs MountType = "tmpfs"
)

// NetworkDriver represents a network driver type.
type NetworkDriver string

const (
	// NetworkDriverBridge is the default bridge network driver.
	NetworkDriverBridge NetworkDriver = "bridge"
	// NetworkDriverHost uses the host's network stack.
	NetworkDriverHost NetworkDriver = "host"
	// NetworkDriverNone disables networking.
	NetworkDriverNone NetworkDriver = "none"
	// NetworkDriverOverlay is for multi-host networking (Swarm).
	NetworkDriverOverlay NetworkDriver = "overlay"
)

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
	// Status is the container's current status.
	Status ContainerStatus
	// Health is the health check status.
	Health HealthStatus
	// Ports lists the container's port bindings.
	Ports []PortBinding
}

// PortBinding represents a port mapping between container and host.
type PortBinding struct {
	// ContainerPort is the port inside the container.
	ContainerPort int
	// HostPort is the port on the host machine.
	HostPort int
	// Protocol is the network protocol.
	Protocol Protocol
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
