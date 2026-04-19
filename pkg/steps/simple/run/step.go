// Package run implements the run step for executing containers.
package run

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/container"
	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes a container. Implements SimpleStep.
type Step struct {
	id          string
	name        string
	image       string
	cmd         []string
	entrypoint  []string
	env         map[string]string
	workdir     string
	mounts      []container.Mount
	ports       []container.PortMapping
	cpus        float64
	memory      int64
	gpus        string
	platform    string
	background  bool
	healthCheck *container.HealthCheckConfig
	pullPolicy  string
	quiet       bool
	tty         bool
	needs       []string
}

// ID returns the step's identifier.
func (s *Step) ID() string {
	return s.id
}

// Name returns the step's display name.
func (s *Step) Name() string {
	return s.name
}

// Execute runs the step and returns its result.
func (s *Step) Execute(ctx context.Context, exec steps.Executor) (*steps.Result, error) {
	// TODO: Implement container execution
	// 1. Wait for dependencies (needs)
	// 2. Pull image if needed
	// 3. Create container
	// 4. Start container
	// 5. Handle background vs foreground
	// 6. Wait for completion
	// 7. Cleanup
	// 8. Return result
	return nil, fmt.Errorf("not implemented")
}
