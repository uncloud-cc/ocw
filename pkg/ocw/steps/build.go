package steps

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// BuildStep represents a leaf step that builds a container image.
type BuildStep struct {
	original *schema.Step
	resolved *schema.BuildStep
	// runtime state for status reporting
	state    string
	message  string
	progress float64
	stage    string // e.g., "layer 5/10"
}

func (s *BuildStep) Type() string { return "build" }
func (s *BuildStep) ID() string   { return string(s.resolved.ID) }
func (s *BuildStep) Name() string { return string(s.resolved.Name) }

func (s *BuildStep) Original() *schema.Step { return s.original }

func (s *BuildStep) Execute(ctx context.Context, stepCtx *workflow.StepContext, opts workflow.ExecuteOptions) (*workflow.StepResult, error) {
	if stepCtx.Runtime == nil {
		return nil, fmt.Errorf("no container runtime available")
	}

	// Build the build options from resolved config
	buildOpts := workflow.BuildOptions{
		Context:    s.resolved.Build.Context,
		Dockerfile: s.resolved.Build.Dockerfile,
		Target:     s.resolved.Build.Target,
		BuildArgs:  s.resolved.Build.BuildArgs,
		Tags:       s.resolved.Build.Tags,
		Platform:   []string{},
		CacheFrom:  s.resolved.Build.CacheFrom,
		CacheTo:    s.resolved.Build.CacheTo,
		Push:       s.resolved.Build.Push,
		Load:       s.resolved.Build.Load,
	}

	// Handle platform
	if s.resolved.Build.Platform != nil {
		if s.resolved.Build.Platform.Single != nil {
			buildOpts.Platform = []string{*s.resolved.Build.Platform.Single}
		} else if s.resolved.Build.Platform.Multiple != nil {
			buildOpts.Platform = s.resolved.Build.Platform.Multiple
		}
	}

	// Call runtime
	result, err := stepCtx.Runtime.Build(ctx, buildOpts)
	if err != nil {
		return nil, err
	}

	// Create outputs
	outputs := make(map[string]string)
	if result.ImageRef != "" {
		outputs["image"] = result.ImageRef
	}
	if result.ImageID != "" {
		outputs["imageId"] = result.ImageID
	}
	if result.Digest != "" {
		outputs["digest"] = result.Digest
	}

	return &workflow.StepResult{
		StepID:  s.ID(),
		Outputs: outputs,
	}, nil
}

// Status returns current execution status.
// For BuildStep: {"state": "building", "stage": "layer 3/10", "progress": 0.3}
func (s *BuildStep) Status() workflow.StepStatus {
	return workflow.StepStatus{
		State:    s.state,
		Message:  s.message,
		Progress: s.progress,
		Metadata: map[string]interface{}{
			"stage":     s.stage,
			"cache_hit": false, // populated by implementation
		},
	}
}

// NewBuildStep creates a new build step.
func NewBuildStep(step *schema.Step, ctx *workflow.StepContext) (*BuildStep, error) {
	if step == nil {
		return nil, fmt.Errorf("step is nil")
	}
	if step.BuildStep == nil {
		return nil, fmt.Errorf("step is not a build step")
	}

	return &BuildStep{
		original: step,
		resolved: step.BuildStep,
		state:    "pending",
	}, nil
}
