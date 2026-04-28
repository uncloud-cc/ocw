package ocw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// Services: background containers that outlive individual step execution
// ---------------------------------------------------------------------------

// ServiceHandle is an opaque reference to a running background container.
// The ContainerRuntime creates it; the Engine holds it for later cleanup.
type ServiceHandle struct {
	// ID is the step ID (user-provided or synthetic) that started this service.
	ID string
	// Name is the human-readable step name.
	Name string
	// ContainerID is the runtime-specific container identifier (e.g. Docker container ID).
	ContainerID string
	// Healthy is true once the service has passed its health check (or has none).
	Healthy bool
}

// ServiceRegistry tracks all running background services for a workflow execution.
// The Engine owns one registry per RunWorkflow call.
type ServiceRegistry struct {
	mu       sync.Mutex
	services []*ServiceHandle
}

// Add registers a running service.
func (r *ServiceRegistry) Add(h *ServiceHandle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services = append(r.services, h)
}

// Get returns the service handle for a given step ID, or nil.
func (r *ServiceRegistry) Get(id string) *ServiceHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.services {
		if h.ID == id {
			return h
		}
	}
	return nil
}

// All returns a copy of all registered services.
func (r *ServiceRegistry) All() []*ServiceHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*ServiceHandle, len(r.services))
	copy(out, r.services)
	return out
}

// serviceRunner starts a background container via the ContainerRuntime and
// registers it with the Engine's ServiceRegistry. The step completes once
// the container is running and healthy (if a health check is configured).
type serviceRunner struct {
	step     *schema.RunStep
	rt       ContainerRuntime
	services *ServiceRegistry
	logger   *log.Logger
}

func (s *serviceRunner) ID() string      { return s.step.ID }
func (s *serviceRunner) Name() string    { return s.step.Name }
func (s *serviceRunner) Needs() []string { return s.step.Needs }

func (s *serviceRunner) Execute(ctx context.Context, scope *Scope) (*StepResult, error) {
	handle, err := s.rt.StartService(ctx, s.step, scope)
	if err != nil {
		return &StepResult{ID: s.step.ID, Status: StatusFailed, Err: err}, err
	}

	// If a health check is configured, poll until healthy.
	if s.step.HealthCheck != nil {
		s.logger.Printf("  service %s: waiting for health check", runnerLabel(s))
		if err := s.rt.CheckHealth(ctx, handle, s.step.HealthCheck); err != nil {
			// Health check failed -- stop the container and report failure.
			_ = s.rt.StopService(ctx, handle)
			return &StepResult{ID: s.step.ID, Status: StatusFailed, Err: fmt.Errorf("health check failed for %q: %w", s.step.Name, err)}, err
		}
		handle.Healthy = true
		s.logger.Printf("  service %s: healthy", runnerLabel(s))
	} else {
		// No health check -- consider it immediately healthy.
		handle.Healthy = true
	}

	s.services.Add(handle)
	return &StepResult{
		ID:     s.step.ID,
		Status: StatusSuccess,
		Output: StepOutput{Values: make(map[string]string)},
	}, nil
}
