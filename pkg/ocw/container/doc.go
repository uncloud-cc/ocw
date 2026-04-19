// Package container provides interfaces for container runtime operations.
//
// This package defines abstractions for:
//
//   - ContainerManager: create, start, stop, remove, and inspect containers
//   - ImageManager: list, inspect, remove, and tag container images
//   - BuildManager: build images, track build progress, manage build cache
//   - Registry: push and pull images to/from registries
//   - NetworkManager: create and manage container networks
//   - VolumeManager: create and manage persistent volumes
//
// The Runtime interface combines all capabilities for implementations
// that support the full feature set (e.g., Docker, Podman).
//
// Implementations can also implement individual interfaces for
// specialized use cases (e.g., a build-only service using Kaniko,
// or a read-only container inspector).
//
// # Interface Design
//
// Each manager interface follows a consistent pattern:
//
//   - Create/Build: create a new resource
//   - Get: retrieve an existing resource by ID or name
//   - List: list resources with optional filters
//   - Prune: remove unused resources (where applicable)
//
// Resources (Container, Image, Build, Network, Volume) are returned as
// interfaces that provide methods to inspect and manipulate the specific
// resource instance.
package container
