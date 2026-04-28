package ocw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ContainerRuntime is the interface that abstracts container operations.
// The CLI provides a concrete implementation; tests use the dummy.
type ContainerRuntime interface {
	Run(ctx context.Context, step *schema.RunStep, scope *Scope) (*StepResult, error)
	Build(ctx context.Context, step *schema.BuildStep, scope *Scope) (*StepResult, error)

	// StartService launches a long-running background container.
	// It returns once the container is running (before health checks pass).
	StartService(ctx context.Context, step *schema.RunStep, scope *Scope) (*ServiceHandle, error)
	// StopService stops a running background container.
	StopService(ctx context.Context, handle *ServiceHandle) error
	// CheckHealth runs the health check for a service and returns nil if healthy.
	CheckHealth(ctx context.Context, handle *ServiceHandle, check *schema.HealthCheck) error
}

// DummyRuntime is a ContainerRuntime for testing and early development.
// It logs what would happen without running real containers.
type DummyRuntime struct {
	Logger *log.Logger
	// Runs records every Run call for test assertions.
	Runs []DummyRun
	// Builds records every Build call for test assertions.
	Builds []DummyBuild
	// Services records every StartService call for test assertions.
	Services []DummyService
	// Stopped records every StopService call for test assertions.
	Stopped []string
	mu      sync.Mutex
	nextID  int
}

// DummyRun records a single Run invocation.
type DummyRun struct {
	Image string
	Cmd   string
	Name  string
}

// DummyBuild records a single Build invocation.
type DummyBuild struct {
	Image string
	Name  string
}

// DummyService records a single StartService invocation.
type DummyService struct {
	Image       string
	Name        string
	ContainerID string
}

// NewDummyRuntime creates a DummyRuntime with the given logger.
func NewDummyRuntime(logger *log.Logger) *DummyRuntime {
	if logger == nil {
		logger = log.Default()
	}
	return &DummyRuntime{Logger: logger}
}

func (d *DummyRuntime) Run(_ context.Context, step *schema.RunStep, _ *Scope) (*StepResult, error) {
	d.mu.Lock()
	d.Runs = append(d.Runs, DummyRun{Image: step.Image, Cmd: step.Cmd, Name: step.Name})
	d.mu.Unlock()

	d.Logger.Printf("  [dummy] run: image=%s cmd=%q", step.Image, step.Cmd)
	return &StepResult{
		ID:     step.ID,
		Status: StatusSuccess,
		Output: StepOutput{Values: make(map[string]string)},
	}, nil
}

func (d *DummyRuntime) Build(_ context.Context, step *schema.BuildStep, _ *Scope) (*StepResult, error) {
	d.mu.Lock()
	d.Builds = append(d.Builds, DummyBuild{Image: step.Build.Image, Name: step.Name})
	d.mu.Unlock()

	d.Logger.Printf("  [dummy] build: image=%s", step.Build.Image)
	return &StepResult{
		ID:     step.ID,
		Status: StatusSuccess,
		Output: StepOutput{Values: map[string]string{"image": step.Build.Image}},
	}, nil
}

func (d *DummyRuntime) StartService(_ context.Context, step *schema.RunStep, _ *Scope) (*ServiceHandle, error) {
	d.mu.Lock()
	d.nextID++
	containerID := fmt.Sprintf("dummy-container-%d", d.nextID)
	d.Services = append(d.Services, DummyService{Image: step.Image, Name: step.Name, ContainerID: containerID})
	d.mu.Unlock()

	d.Logger.Printf("  [dummy] start service: image=%s name=%q container=%s", step.Image, step.Name, containerID)
	return &ServiceHandle{
		ID:          step.ID,
		Name:        step.Name,
		ContainerID: containerID,
	}, nil
}

func (d *DummyRuntime) StopService(_ context.Context, handle *ServiceHandle) error {
	d.mu.Lock()
	d.Stopped = append(d.Stopped, handle.ContainerID)
	d.mu.Unlock()

	d.Logger.Printf("  [dummy] stop service: container=%s name=%q", handle.ContainerID, handle.Name)
	return nil
}

func (d *DummyRuntime) CheckHealth(_ context.Context, _ *ServiceHandle, _ *schema.HealthCheck) error {
	// Dummy always reports healthy immediately.
	return nil
}
