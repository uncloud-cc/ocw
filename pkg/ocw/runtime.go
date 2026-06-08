package ocw

import (
	"context"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ExposedServicePort describes a single exposed port for a running service.
type ExposedServicePort struct {
	Protocol      string
	HostPort      int
	ContainerPort int
}

// ServiceInfo describes a running background service.
type ServiceInfo struct {
	Name        string
	ContainerID string
	Exposed     []ExposedServicePort
}

// Runtime is the interface for executing steps.
type Runtime interface {
	Run(ctx context.Context, step *schema.RunStep, prefix string) (map[string]string, error)
	Build(ctx context.Context, step *schema.BuildStep, prefix string) (map[string]string, error)
	// StartService launches a background container and returns information
	// about the running service (e.g. containerID, exposed ports).
	StartService(ctx context.Context, step *schema.RunStep, prefix string) (map[string]string, error)
	// HasActiveServices reports whether there are background containers still running.
	HasActiveServices() bool
	// ListServices returns information about all active background services.
	ListServices() []ServiceInfo
}
