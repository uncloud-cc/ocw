package container

import (
	"context"
	"io"
	"time"
)

// BuildManager manages image builds and build cache.
type BuildManager interface {
	// Build builds a container image from a Dockerfile/build context.
	// Returns a Build handle to track progress and get results.
	Build(ctx context.Context, opts BuildOptions) (Build, error)

	// Get retrieves an existing build by ID.
	Get(ctx context.Context, id string) (Build, error)

	// List returns builds matching the given filters.
	List(ctx context.Context, opts BuildListOptions) ([]Build, error)

	// Prune removes build cache.
	Prune(ctx context.Context, opts BuildPruneOptions) (BuildPruneResult, error)
}

// Build represents a build operation.
type Build interface {
	// ID returns the unique identifier of this build.
	ID() string

	// Wait blocks until the build completes and returns the result.
	Wait(ctx context.Context) (BuildResult, error)

	// Cancel cancels a running build.
	Cancel(ctx context.Context) error

	// Logs returns a reader for build output.
	Logs(ctx context.Context) (io.ReadCloser, error)

	// Inspect returns detailed information about the build.
	Inspect(ctx context.Context) (BuildInfo, error)
}

// BuildOptions configures image building.
type BuildOptions struct {
	// Context is the path to the build context directory.
	Context string

	// Dockerfile is the path to the Dockerfile (relative to context).
	Dockerfile string

	// Tags are the image tags to apply (e.g., "myapp:latest").
	Tags []string

	// BuildArgs are build-time variables.
	BuildArgs map[string]string

	// Target is the build stage to target (for multi-stage builds).
	Target string

	// Platform is the target platform (e.g., "linux/amd64").
	// Can be a single platform or comma-separated list for multi-platform builds.
	Platform string

	// NoCache disables build caching.
	NoCache bool

	// Pull always pulls base images.
	Pull bool

	// CacheFrom are cache sources (e.g., "type=registry,ref=myapp:cache").
	CacheFrom []string

	// CacheTo are cache export destinations.
	CacheTo []string

	// Labels are metadata to apply to the image.
	Labels map[string]string

	// Secrets are build secrets available during build.
	// Keys are secret IDs, values are the secret content.
	Secrets map[string]string

	// Output configures where to write the build result.
	Output *BuildOutput

	// Progress receives build progress updates.
	Progress io.Writer

	// ShmSize is the size of /dev/shm (e.g., "256m").
	ShmSize string

	// Quiet suppresses build output.
	Quiet bool
}

// BuildListOptions configures build listing.
type BuildListOptions struct {
	// All includes completed builds. By default, only active builds are listed.
	All bool

	// Filters restricts the list to builds matching these criteria.
	Filters map[string][]string
}

// BuildPruneOptions configures build cache pruning.
type BuildPruneOptions struct {
	// All removes all build cache, not just dangling layers.
	All bool

	// KeepStorage is the amount of cache to keep (e.g., "10GB").
	KeepStorage string

	// Filters restricts which cache entries are pruned.
	Filters map[string][]string
}

// BuildPruneResult contains the result of a cache prune operation.
type BuildPruneResult struct {
	// CachesDeleted are the IDs of deleted cache entries.
	CachesDeleted []string

	// SpaceReclaimed is the disk space freed in bytes.
	SpaceReclaimed int64
}

// BuildOutput configures build output destination.
type BuildOutput struct {
	// Type is the output type: "docker", "image", "local", "tar", "oci", "registry".
	Type BuildOutputType

	// Dest is the output destination (path or registry).
	Dest string

	// Push pushes to registry after build.
	Push bool

	// Compression is the compression type for tar/oci outputs.
	Compression string

	// CompressionLevel is the compression level (0-9).
	CompressionLevel int
}

// BuildOutputType identifies the build output format.
type BuildOutputType string

const (
	BuildOutputDocker   BuildOutputType = "docker"
	BuildOutputImage    BuildOutputType = "image"
	BuildOutputLocal    BuildOutputType = "local"
	BuildOutputTar      BuildOutputType = "tar"
	BuildOutputOCI      BuildOutputType = "oci"
	BuildOutputRegistry BuildOutputType = "registry"
)

// BuildInfo contains detailed build metadata.
type BuildInfo struct {
	// ID is the build identifier.
	ID string

	// Status is the current build status.
	Status BuildStatus

	// ImageID is the built image's ID (if completed successfully).
	ImageID string

	// Tags are the tags applied to the image.
	Tags []string

	// Digest is the image digest (if pushed or loaded).
	Digest string

	// StartedAt is when the build started.
	StartedAt time.Time

	// CompletedAt is when the build completed (if finished).
	CompletedAt time.Time

	// Duration is how long the build took.
	Duration time.Duration

	// Error contains the error message if the build failed.
	Error string

	// Metadata contains additional build metadata.
	Metadata map[string]string
}

// BuildStatus represents the state of a build.
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusSuccess   BuildStatus = "success"
	BuildStatusFailure   BuildStatus = "failure"
	BuildStatusCancelled BuildStatus = "cancelled"
)

// BuildResult contains the results of a successful build.
type BuildResult struct {
	// ImageID is the built image's ID.
	ImageID string

	// Tags are the tags applied to the image.
	Tags []string

	// Digest is the image digest (if pushed or loaded).
	Digest string

	// Metadata contains additional build metadata.
	Metadata map[string]string
}
