package container

import (
	"errors"
	"fmt"
)

// Common errors returned by container operations.
var (
	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a resource already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrInUse indicates a resource is in use and cannot be removed.
	ErrInUse = errors.New("in use")

	// ErrTimeout indicates an operation timed out.
	ErrTimeout = errors.New("timeout")

	// ErrUnhealthy indicates a health check failed.
	ErrUnhealthy = errors.New("unhealthy")
)

// ContainerError represents an error from container operations.
type ContainerError struct {
	// ContainerID is the container that encountered the error.
	ContainerID string

	// Operation is what was being attempted (e.g., "start", "stop", "exec").
	Operation string

	// Cause is the underlying error.
	Cause error
}

func (e *ContainerError) Error() string {
	if e.ContainerID != "" {
		return fmt.Sprintf("container %s %s failed: %v", e.ContainerID, e.Operation, e.Cause)
	}
	return fmt.Sprintf("container %s failed: %v", e.Operation, e.Cause)
}

func (e *ContainerError) Unwrap() error {
	return e.Cause
}

// BuildError represents an error during image building.
type BuildError struct {
	// Image is the image that was being built.
	Image string

	// Stage is the build stage that failed (for multi-stage builds).
	Stage string

	// Cause is the underlying error.
	Cause error
}

func (e *BuildError) Error() string {
	if e.Stage != "" {
		return fmt.Sprintf("build %q failed at stage %q: %v", e.Image, e.Stage, e.Cause)
	}
	return fmt.Sprintf("build %q failed: %v", e.Image, e.Cause)
}

func (e *BuildError) Unwrap() error {
	return e.Cause
}

// RegistryError represents an error with registry operations.
type RegistryError struct {
	// Image is the image reference.
	Image string

	// Operation is what was being attempted ("push" or "pull").
	Operation string

	// Cause is the underlying error.
	Cause error
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("%s %q failed: %v", e.Operation, e.Image, e.Cause)
}

func (e *RegistryError) Unwrap() error {
	return e.Cause
}

// NetworkError represents an error with network operations.
type NetworkError struct {
	// Network is the network name or ID.
	Network string

	// Operation is what was being attempted.
	Operation string

	// Cause is the underlying error.
	Cause error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network %q %s failed: %v", e.Network, e.Operation, e.Cause)
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// VolumeError represents an error with volume operations.
type VolumeError struct {
	// Volume is the volume name.
	Volume string

	// Operation is what was being attempted.
	Operation string

	// Cause is the underlying error.
	Cause error
}

func (e *VolumeError) Error() string {
	return fmt.Sprintf("volume %q %s failed: %v", e.Volume, e.Operation, e.Cause)
}

func (e *VolumeError) Unwrap() error {
	return e.Cause
}

// IsNotFound returns true if the error indicates a resource was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists returns true if the error indicates a resource already exists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsInUse returns true if the error indicates a resource is in use.
func IsInUse(err error) bool {
	return errors.Is(err, ErrInUse)
}

// IsTimeout returns true if the error indicates a timeout.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsUnhealthy returns true if the error indicates a health check failed.
func IsUnhealthy(err error) bool {
	return errors.Is(err, ErrUnhealthy)
}
