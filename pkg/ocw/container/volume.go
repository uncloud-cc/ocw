package container

import (
	"context"
)

// VolumeManager creates and manages persistent volumes.
type VolumeManager interface {
	// Create creates a named volume for data persistence.
	Create(ctx context.Context, opts VolumeOptions) (Volume, error)

	// Get retrieves an existing volume by name.
	Get(ctx context.Context, name string) (Volume, error)

	// List returns volumes matching the given filters.
	List(ctx context.Context, opts VolumeListOptions) ([]Volume, error)

	// Prune removes unused volumes.
	Prune(ctx context.Context, opts VolumePruneOptions) (VolumePruneResult, error)
}

// Volume represents a persistent storage volume.
type Volume interface {
	// Name returns the volume name.
	Name() string

	// Driver returns the volume driver.
	Driver() string

	// Mountpoint returns the path on the host where the volume data is stored.
	Mountpoint() string

	// Labels returns the volume's metadata.
	Labels() map[string]string

	// Remove deletes this volume. No containers may be using it.
	Remove(ctx context.Context, force bool) error

	// Inspect returns detailed information about the volume.
	Inspect(ctx context.Context) (VolumeInfo, error)
}

// VolumeOptions configures volume creation.
type VolumeOptions struct {
	// Name is the volume name (required).
	Name string

	// Driver is the volume driver (e.g., "local", "nfs").
	// Defaults to "local" if not specified.
	Driver string

	// DriverOpts are driver-specific options.
	// For the "local" driver, this can include "type", "device", "o" (mount options).
	DriverOpts map[string]string

	// Labels are metadata key-value pairs.
	Labels map[string]string
}

// VolumeListOptions configures volume listing.
type VolumeListOptions struct {
	// Filters restricts the list to volumes matching these criteria.
	// Common filters: "name", "driver", "label", "dangling".
	Filters map[string][]string
}

// VolumePruneOptions configures volume pruning.
type VolumePruneOptions struct {
	// Filters restricts which volumes are pruned.
	// Common filters: "label", "all" (include anonymous volumes).
	Filters map[string][]string
}

// VolumePruneResult contains the result of a prune operation.
type VolumePruneResult struct {
	// VolumesDeleted are the names of deleted volumes.
	VolumesDeleted []string

	// SpaceReclaimed is the disk space freed in bytes.
	SpaceReclaimed int64
}

// VolumeInfo contains detailed volume metadata.
type VolumeInfo struct {
	// Name is the volume name.
	Name string

	// Driver is the volume driver.
	Driver string

	// Mountpoint is the path on the host.
	Mountpoint string

	// Labels are the volume's metadata.
	Labels map[string]string

	// Scope is the volume scope (local, global).
	Scope string

	// Options are the driver options used to create the volume.
	Options map[string]string

	// UsageData contains usage information if available.
	UsageData *VolumeUsageData

	// CreatedAt is when the volume was created.
	CreatedAt string
}

// VolumeUsageData contains volume usage statistics.
type VolumeUsageData struct {
	// Size is the disk space used by the volume in bytes.
	// -1 indicates the size is not available.
	Size int64

	// RefCount is the number of containers using this volume.
	// -1 indicates the count is not available.
	RefCount int64
}
