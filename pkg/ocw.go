// Package ocw provides a workflow engine for parsing and executing OCW workflow files.
package ocw

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// StepType represents the type of a workflow step
type StepType string

const (
	// StepTypeRun represents a container run step
	StepTypeRun StepType = "run"
	// StepTypeBuild represents a container build step
	StepTypeBuild StepType = "build"
	// StepTypeParallel represents a parallel execution block
	StepTypeParallel StepType = "parallel"
	// StepTypeSequence represents a sequential execution block
	StepTypeSequence StepType = "sequence"
	// StepTypeSwitch represents a conditional switch block
	StepTypeSwitch StepType = "switch"
	// StepTypeJob represents a job reference (when using the 'jobs' syntax)
	StepTypeJob StepType = "job"
)

// OCW represents the root workflow configuration
type OCW struct {
	Name     string                 `yaml:"name"`
	Env      map[string]interface{} `yaml:"env,omitempty"`
	Sequence []Step                 `yaml:"sequence,omitempty"`
	Parallel []Step                 `yaml:"parallel,omitempty"`
	Switch   *SwitchStep            `yaml:"switch,omitempty"`
	Jobs     map[string]Job         `yaml:"jobs,omitempty"`
}

// Job represents a named workflow job that can be referenced
type Job struct {
	Name     string      `yaml:"name,omitempty"`
	Sequence []Step      `yaml:"sequence,omitempty"`
	Parallel []Step      `yaml:"parallel,omitempty"`
	Switch   *SwitchStep `yaml:"switch,omitempty"`
}

// Step represents a single workflow step
type Step struct {
	ID         string            `yaml:"id,omitempty"`
	Name       string            `yaml:"name,omitempty"`
	Image      string            `yaml:"image,omitempty"`
	Cmd        string            `yaml:"cmd,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Background bool              `yaml:"background,omitempty"`
	Watch      *WatchConfig      `yaml:"watch,omitempty"`
	Expose     []int             `yaml:"expose,omitempty"`
	Build      *BuildConfig      `yaml:"build,omitempty"`
	Sequence   []Step            `yaml:"sequence,omitempty"`
	Parallel   []Step            `yaml:"parallel,omitempty"`
	Switch     *SwitchStep       `yaml:"switch,omitempty"`
	Job        string            `yaml:"job,omitempty"`
}

// WatchConfig represents watch configuration for a step
type WatchConfig struct {
	Enabled bool     `yaml:"enabled,omitempty"`
	Paths   []string `yaml:"paths,omitempty"`
}

// BuildConfig represents container build configuration
type BuildConfig struct {
	Image      string            `yaml:"image"`
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Target     string            `yaml:"target,omitempty"`
	BuildArgs  map[string]string `yaml:"buildArgs,omitempty"`
	Tags       []string          `yaml:"tags,omitempty"`
	Push       bool              `yaml:"push,omitempty"`
	Load       bool              `yaml:"load,omitempty"`
	Pull       bool              `yaml:"pull,omitempty"`
	NoCache    bool              `yaml:"noCache,omitempty"`
	Secrets    map[string]string `yaml:"secrets,omitempty"`
}

// SwitchStep represents a conditional switch block
type SwitchStep struct {
	Expression string     `yaml:"expression,omitempty"`
	Cases      []CaseStep `yaml:"cases,omitempty"`
	Default    []Step     `yaml:"default,omitempty"`
}

// CaseStep represents a single case in a switch statement
type CaseStep struct {
	Value string `yaml:"value"`
	Steps []Step `yaml:"steps"`
}

// StepExecutor is the interface for executing workflow steps.
// This interface can be implemented with different backends (mock, Docker, etc.)
type StepExecutor interface {
	// ExecuteRunStep executes a container run step
	ExecuteRunStep(ctx context.Context, step Step) error
	// ExecuteBuildStep executes a container build step
	ExecuteBuildStep(ctx context.Context, step Step, buildConfig BuildConfig) error
}

// Workflow represents a parsed workflow ready for execution
type Workflow struct {
	ocw      *OCW
	executor StepExecutor
}

// StepInfo holds information about a step for iteration
type StepInfo struct {
	Step    Step
	Type    StepType
	Parent  *StepInfo
	JobName string
}

// ParseFile reads and parses an OCW workflow file
func ParseFile(path string) (*OCW, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file: %w", err)
	}

	var ocw OCW
	if err := yaml.Unmarshal(data, &ocw); err != nil {
		return nil, fmt.Errorf("parsing workflow file: %w", err)
	}

	return &ocw, nil
}

// ParseBytes parses OCW workflow data from bytes
func ParseBytes(data []byte) (*OCW, error) {
	var ocw OCW
	if err := yaml.Unmarshal(data, &ocw); err != nil {
		return nil, fmt.Errorf("parsing workflow: %w", err)
	}

	return &ocw, nil
}

// NewWorkflow creates a new Workflow instance from a parsed OCW config
func NewWorkflow(ocw *OCW, executor StepExecutor) *Workflow {
	return &Workflow{
		ocw:      ocw,
		executor: executor,
	}
}

// Name returns the workflow name
func (w *Workflow) Name() string {
	if w.ocw.Name == "" {
		return "Unnamed Workflow"
	}
	return w.ocw.Name
}

// ExtractSteps extracts all steps from the workflow into a flat list
// This includes expanding nested sequences/parallel blocks and job references
func (w *Workflow) ExtractSteps() []StepInfo {
	var steps []StepInfo

	// First, check for top-level workflow structure
	if len(w.ocw.Sequence) > 0 {
		steps = w.extractStepsFromList(w.ocw.Sequence, nil)
	} else if len(w.ocw.Parallel) > 0 {
		steps = w.extractStepsFromList(w.ocw.Parallel, nil)
	} else if w.ocw.Switch != nil {
		steps = w.extractStepsFromSwitch(w.ocw.Switch, nil)
	}

	return steps
}

// extractStepsFromList extracts steps from a list (sequence or parallel)
func (w *Workflow) extractStepsFromList(stepList []Step, parent *StepInfo) []StepInfo {
	var steps []StepInfo

	for _, step := range stepList {
		steps = append(steps, w.extractStepInfo(step, parent)...)
	}

	return steps
}

// extractStepInfo extracts step information from a single step
func (w *Workflow) extractStepInfo(step Step, parent *StepInfo) []StepInfo {
	var steps []StepInfo

	// Determine step type and handle nested structures
	switch {
	// Job reference
	case step.Job != "":
		if job, exists := w.ocw.Jobs[step.Job]; exists {
			jobParent := &StepInfo{
				Step:    step,
				Type:    StepTypeJob,
				Parent:  parent,
				JobName: step.Job,
			}
			if len(job.Sequence) > 0 {
				steps = append(steps, w.extractStepsFromList(job.Sequence, jobParent)...)
			} else if len(job.Parallel) > 0 {
				steps = append(steps, w.extractStepsFromList(job.Parallel, jobParent)...)
			}
		}

	// Build step
	case step.Build != nil:
		steps = append(steps, StepInfo{
			Step:   step,
			Type:   StepTypeBuild,
			Parent: parent,
		})

	// Nested sequence
	case len(step.Sequence) > 0:
		seqParent := &StepInfo{
			Step:   step,
			Type:   StepTypeSequence,
			Parent: parent,
		}
		steps = append(steps, w.extractStepsFromList(step.Sequence, seqParent)...)

	// Nested parallel
	case len(step.Parallel) > 0:
		parParent := &StepInfo{
			Step:   step,
			Type:   StepTypeParallel,
			Parent: parent,
		}
		steps = append(steps, w.extractStepsFromList(step.Parallel, parParent)...)

	// Switch statement
	case step.Switch != nil:
		steps = append(steps, w.extractStepsFromSwitch(step.Switch, parent)...)

	// Regular run step
	case step.Image != "":
		steps = append(steps, StepInfo{
			Step:   step,
			Type:   StepTypeRun,
			Parent: parent,
		})
	}

	return steps
}

// extractStepsFromSwitch extracts steps from a switch block
func (w *Workflow) extractStepsFromSwitch(switchStep *SwitchStep, parent *StepInfo) []StepInfo {
	var steps []StepInfo

	switchParent := &StepInfo{
		Step:   Step{Switch: switchStep},
		Type:   StepTypeSwitch,
		Parent: parent,
	}

	// Extract steps from each case
	for _, caseStep := range switchStep.Cases {
		steps = append(steps, w.extractStepsFromList(caseStep.Steps, switchParent)...)
	}

	// Extract steps from default case
	if len(switchStep.Default) > 0 {
		steps = append(steps, w.extractStepsFromList(switchStep.Default, switchParent)...)
	}

	return steps
}

// Execute runs the workflow by executing all extracted steps
func (w *Workflow) Execute(ctx context.Context) error {
	steps := w.ExtractSteps()

	for _, stepInfo := range steps {
		if err := w.executeStep(ctx, stepInfo); err != nil {
			return fmt.Errorf("executing step %s: %w", stepInfo.Step.Name, err)
		}
	}

	return nil
}

// executeStep executes a single step using the configured executor
func (w *Workflow) executeStep(ctx context.Context, stepInfo StepInfo) error {
	switch stepInfo.Type {
	case StepTypeRun:
		return w.executor.ExecuteRunStep(ctx, stepInfo.Step)
	case StepTypeBuild:
		if stepInfo.Step.Build != nil {
			return w.executor.ExecuteBuildStep(ctx, stepInfo.Step, *stepInfo.Step.Build)
		}
		return fmt.Errorf("build step has no build configuration")
	default:
		// Other step types (sequence, parallel, switch) are containers
		// and their children are already extracted and will be executed individually
		return nil
	}
}
