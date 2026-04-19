package container

import (
	"context"
	"io"
	"time"
)

// Runtime is the interface for container operations.
// Implementations are provided externally (Docker, Podman, etc.).
type Runtime interface {
	// Image operations

	// Pull pulls an image from a registry.
	Pull(ctx context.Context, image string, opts PullOptions) error
	// Build builds an image from a Dockerfile.
	Build(ctx context.Context, opts BuildOptions) (ImageID, error)
	// ImageExists checks if an image exists locally.
	ImageExists(ctx context.Context, image string) (bool, error)

	// Container lifecycle

	// Create creates a new container without starting it.
	Create(ctx context.Context, opts CreateOptions) (ContainerID, error)
	// Start starts a created container.
	Start(ctx context.Context, id ContainerID) error
	// Stop gracefully stops a running container.
	Stop(ctx context.Context, id ContainerID, timeout time.Duration) error
	// Remove removes a container.
	Remove(ctx context.Context, id ContainerID, force bool) error
	// Kill sends a signal to a container.
	Kill(ctx context.Context, id ContainerID, signal string) error

	// Container inspection

	// Wait blocks until a container exits and returns its exit result.
	Wait(ctx context.Context, id ContainerID) (ExitResult, error)
	// Inspect returns detailed information about a container.
	Inspect(ctx context.Context, id ContainerID) (ContainerInfo, error)

	// Logs and I/O

	// Logs retrieves container logs.
	Logs(ctx context.Context, id ContainerID, opts LogOptions) (io.ReadCloser, error)
	// Attach attaches to a running container's I/O streams.
	Attach(ctx context.Context, id ContainerID, opts AttachOptions) (Streams, error)

	// Exec runs a command inside a running container.
	Exec(ctx context.Context, id ContainerID, cmd []string) (ExitResult, error)

	// Volumes

	// CreateVolume creates a new named volume.
	CreateVolume(ctx context.Context, name string) (VolumeID, error)
	// RemoveVolume removes a volume.
	RemoveVolume(ctx context.Context, id VolumeID) error

	// Networks

	// CreateNetwork creates a new network.
	CreateNetwork(ctx context.Context, name string, opts NetworkOptions) (NetworkID, error)
	// RemoveNetwork removes a network.
	RemoveNetwork(ctx context.Context, id NetworkID) error
	// ConnectNetwork connects a container to a network.
	ConnectNetwork(ctx context.Context, networkID NetworkID, containerID ContainerID) error
}
