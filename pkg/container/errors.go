package container

import (
	"errors"
	"fmt"
)

// Container runtime errors.
var (
	// ErrImageNotFound is returned when an image cannot be found.
	ErrImageNotFound = errors.New("image not found")
	// ErrContainerNotFound is returned when a container cannot be found.
	ErrContainerNotFound = errors.New("container not found")
	// ErrContainerNotRunning is returned when trying to stop a non-running container.
	ErrContainerNotRunning = errors.New("container is not running")
	// ErrBuildFailed is returned when an image build fails.
	ErrBuildFailed = errors.New("build failed")
	// ErrNetworkNotFound is returned when a network cannot be found.
	ErrNetworkNotFound = errors.New("network not found")
	// ErrVolumeNotFound is returned when a volume cannot be found.
	ErrVolumeNotFound = errors.New("volume not found")
)

// ContainerError wraps an error with container context.
type ContainerError struct {
	// ContainerID is the ID of the container where the error occurred.
	ContainerID ContainerID
	// Operation is the operation being performed: "create", "start", "stop", etc.
	Operation string
	// Err is the underlying error.
	Err error
}

// Error returns the error message.
func (e *ContainerError) Error() string {
	return fmt.Sprintf("%s container %s: %v", e.Operation, e.ContainerID, e.Err)
}

// Unwrap returns the underlying error.
func (e *ContainerError) Unwrap() error {
	return e.Err
}
