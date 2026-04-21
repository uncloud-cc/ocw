package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// RunStep represents a leaf step that runs a container.
type RunStep struct {
	original *schema.Step
	resolved *schema.RunStep
	// runtime state for status reporting
	containerID string
	state       string
	message     string
	progress    float64
}

func (s *RunStep) Type() string { return "run" }
func (s *RunStep) ID() string   { return string(s.resolved.ID) }
func (s *RunStep) Name() string { return string(s.resolved.Name) }

func (s *RunStep) Original() *schema.Step { return s.original }

func (s *RunStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	if stepCtx.Runtime == nil {
		return nil, fmt.Errorf("no container runtime available")
	}

	// Build run options
	runOpts := workflow.RunOptions{
		Image:      s.resolved.Image,
		Cmd:        s.resolved.Cmd,
		Args:       s.resolved.Args,
		Entrypoint: s.resolved.Entrypoint,
		Workdir:    s.resolved.Workdir,
		Background: s.resolved.Background,
		TTY:        s.resolved.TTY,
		Env:        make(map[string]string),
	}

	// Merge environment variables
	for k, v := range s.resolved.Env {
		runOpts.Env[k] = v.Value
	}
	if s.resolved.RunEnv != nil && s.resolved.RunEnv.Map != nil {
		for k, v := range s.resolved.RunEnv.Map {
			runOpts.Env[k] = v
		}
	}

	// Handle resource limits
	if s.resolved.CPUs != nil && s.resolved.CPUs.String != nil {
		runOpts.CPUs = *s.resolved.CPUs.String
	}
	runOpts.Memory = s.resolved.Memory
	if s.resolved.GPUs != nil && s.resolved.GPUs.String != nil {
		runOpts.GPUs = *s.resolved.GPUs.String
	}

	// Handle expose ports
	if s.resolved.Expose != nil {
		runOpts.Expose = make([]workflow.PortMapping, len(s.resolved.Expose.Ports))
		for i, p := range s.resolved.Expose.Ports {
			runOpts.Expose[i] = workflow.PortMapping{
				ContainerPort: p.ContainerPort,
				HostPort:      p.HostPort,
				Protocol:      p.Protocol,
			}
		}
	}

	// Handle healthcheck
	if s.resolved.HealthCheck != nil {
		runOpts.HealthCheck = &workflow.HealthCheckConfig{
			Cmd:         s.resolved.HealthCheck.Cmd,
			Interval:    s.resolved.HealthCheck.Interval,
			Timeout:     s.resolved.HealthCheck.Timeout,
			Retries:     s.resolved.HealthCheck.Retries,
			StartPeriod: s.resolved.HealthCheck.StartPeriod,
		}
	}

	// Call runtime
	result, err := stepCtx.Runtime.Run(ctx, runOpts)
	if err != nil {
		return nil, err
	}

	stepResult := &workflow.StepResult{
		StepID:  s.ID(),
		Outputs: result.Outputs,
	}

	// Handle background service
	if s.resolved.Background {
		stepResult.Service = &workflow.ServiceInfo{
			StepID:      s.ID(),
			ContainerID: result.ContainerID,
			Ports:       runOpts.Expose,
		}
	}

	return stepResult, nil
}

// Status returns current execution status.
// For RunStep: {"state": "running", "message": "Pulling image...", "container_id": "abc"}
func (s *RunStep) Status() workflow.StepStatus {
	return workflow.StepStatus{
		State:    s.state,
		Message:  s.message,
		Progress: s.progress,
		Metadata: map[string]interface{}{
			"container_id": s.containerID,
		},
	}
}

// NewRunStep creates a new run step.
func NewRunStep(step *schema.Step, ctx *workflow.StepContext) (*RunStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.RunStep == nil {
		return nil, fmt.Errorf("step is not a run step")
	}

	return &RunStep{
		original: step,
		resolved: step.RunStep,
		state:    "pending",
	}, nil
}
