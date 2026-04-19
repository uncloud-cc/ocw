package container

import (
	"context"
	"io"
)

// Registry handles pushing and pulling container images.
type Registry interface {
	// Push pushes a container image to a registry.
	Push(ctx context.Context, opts PushOptions) error

	// Pull pulls a container image from a registry.
	Pull(ctx context.Context, image string, opts PullOptions) error
}

// PushOptions configures image pushing.
type PushOptions struct {
	// Image is the image reference to push (required).
	Image string

	// AllTags pushes all tags of the image.
	AllTags bool

	// Progress receives push progress updates.
	Progress io.Writer

	// Quiet suppresses progress output.
	Quiet bool
}

// PullOptions configures image pulling.
type PullOptions struct {
	// Platform is the target platform (e.g., "linux/amd64").
	Platform string

	// AllTags pulls all tags of the image.
	AllTags bool

	// Progress receives pull progress updates.
	Progress io.Writer

	// Quiet suppresses progress output.
	Quiet bool
}
