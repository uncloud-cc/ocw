package container

import (
	"context"
	"time"
)

// ImageManager manages container images.
type ImageManager interface {
	// Get retrieves an image by reference (name, name:tag, or digest).
	Get(ctx context.Context, ref string) (Image, error)

	// List returns images matching the given filters.
	List(ctx context.Context, opts ImageListOptions) ([]Image, error)

	// Remove removes an image.
	Remove(ctx context.Context, ref string, opts ImageRemoveOptions) error

	// Tag adds a tag to an image.
	Tag(ctx context.Context, source, target string) error

	// History returns the history of an image.
	History(ctx context.Context, ref string) ([]ImageHistoryEntry, error)
}

// Image represents a container image.
type Image interface {
	// ID returns the image's unique identifier (digest).
	ID() string

	// Tags returns all tags associated with this image.
	Tags() []string

	// Size returns the image size in bytes.
	Size() int64

	// Created returns when the image was created.
	Created() time.Time

	// Inspect returns detailed information about the image.
	Inspect(ctx context.Context) (ImageInfo, error)
}

// ImageListOptions configures image listing.
type ImageListOptions struct {
	// All includes intermediate images. By default, only top-level images are listed.
	All bool

	// Filters restricts the list to images matching these criteria.
	// Common filters: "reference", "label", "dangling", "before", "since".
	Filters map[string][]string
}

// ImageRemoveOptions configures image removal.
type ImageRemoveOptions struct {
	// Force removes the image even if containers are using it.
	Force bool

	// PruneChildren removes untagged parent images.
	PruneChildren bool
}

// ImageInfo contains detailed image metadata.
type ImageInfo struct {
	// ID is the image identifier (digest).
	ID string

	// Tags are all tags for this image.
	Tags []string

	// Digests are the repo digests for this image.
	Digests []string

	// Size is the image size in bytes.
	Size int64

	// Created is when the image was created.
	Created time.Time

	// Author is the image author.
	Author string

	// Architecture is the target architecture.
	Architecture string

	// OS is the target operating system.
	OS string

	// Config contains the image's default container configuration.
	Config ImageConfig

	// Labels are metadata attached to the image.
	Labels map[string]string
}

// ImageConfig contains default container configuration from the image.
type ImageConfig struct {
	// Cmd is the default command.
	Cmd []string

	// Entrypoint is the default entrypoint.
	Entrypoint []string

	// Env are the default environment variables.
	Env []string

	// WorkingDir is the default working directory.
	WorkingDir string

	// ExposedPorts are ports the image exposes.
	ExposedPorts map[string]struct{}

	// Volumes are volumes the image declares.
	Volumes map[string]struct{}

	// User is the default user.
	User string
}

// ImageHistoryEntry represents a layer in the image history.
type ImageHistoryEntry struct {
	// ID is the layer ID.
	ID string

	// Created is when the layer was created.
	Created time.Time

	// CreatedBy is the command that created this layer.
	CreatedBy string

	// Size is the layer size in bytes.
	Size int64

	// Comment is an optional comment.
	Comment string

	// Tags are tags at this layer (if any).
	Tags []string
}
