package ocw

import "github.com/uncloud-cc/ocw/pkg/ocw/container"

// Runtime combines all container capabilities into a single interface.
// Implementations like Docker and Podman that support the full feature set
// should implement this interface.
//
// For specialized implementations that only support a subset of features
// (e.g., Kaniko for builds only, or a read-only container inspector),
// implement the individual interfaces instead.
type Runtime interface {
	container.ContainerManager
	container.ImageManager
	container.Builder
	container.Registry
	container.NetworkManager
	container.VolumeManager

	// Close releases any resources held by the runtime.
	Close() error
}
