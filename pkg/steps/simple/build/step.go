// Package build implements the build step for building container images.
package build

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/steps"
)

// Step builds a container image. Implements SimpleStep.
type Step struct {
	id   string
	name string
	// Build configuration
	contextPath string
	dockerfile  string
	tags        []string
	buildArgs   map[string]string
	target      string
	platform    []string
	cacheFrom   []string
	cacheTo     []string
	noCache     bool
	pull        bool
	push        bool
	load        bool
	labels      map[string]string
	secrets     []string // Build secrets
	// Output control
	quiet bool
}

// ID returns the step's identifier.
func (s *Step) ID() string {
	return s.id
}

// Name returns the step's display name.
func (s *Step) Name() string {
	return s.name
}

// Execute runs the build and returns its result.
func (s *Step) Execute(ctx context.Context, exec steps.Executor) (*steps.Result, error) {
	// TODO: Implement image build
	// 1. Create build options from configuration
	// 2. Call container runtime Build
	// 3. Handle progress output
	// 4. Return result with image ID
	return nil, fmt.Errorf("not implemented")
}
