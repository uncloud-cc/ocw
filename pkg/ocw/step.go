package ocw

import (
	"context"
	"io"
	"time"

	"github.com/uncloud-cc/ocw/pkg/ocw/container"
)

// StepType identifies the kind of step in a workflow.
type StepType string

const (
	StepTypeRun      StepType = "run"      // Container execution
	StepTypeBuild    StepType = "build"    // Image build
	StepTypeParallel StepType = "parallel" // Parallel execution of children
	StepTypeSequence StepType = "sequence" // Sequential execution of children
	StepTypeSwitch   StepType = "switch"   // Conditional branching
	StepTypeWorkflow StepType = "workflow" // Nested workflow invocation
)

// Step represents a single unit of work in an OCW workflow.
// Steps form a tree structure where composite steps (parallel, sequence, switch)
// contain child steps.
type Step interface {
	// Type returns the kind of step this is.
	Type() StepType

	// Name returns the human-readable name for display and logging.
	Name() string

	// ID returns the optional identifier used to reference this step's outputs.
	// Returns empty string if no ID is set.
	ID() string

	// Description returns an optional human-readable description.
	Description() string

	// Children returns child steps for composite step types (parallel, sequence, switch).
	// Returns nil for leaf steps (run, build).
	Children() []Step

	// Needs returns the IDs of steps/services that must complete or be healthy
	// before this step can run.
	Needs() []string
}

// StepExecutor executes steps within an execution context.
// Implementations handle the specifics of each step type.
type StepExecutor interface {
	// Execute runs a step and returns its result.
	// The context controls cancellation and timeout.
	Execute(ctx context.Context, step Step, inputs StepInputs) (StepResult, error)
}

// StepInputs provides all input data available to a step during execution.
type StepInputs struct {
	// Env contains environment variables from workflow, job, and step levels.
	// Later declarations override earlier ones.
	Env map[string]string

	// Secrets contains secret values available to the step.
	Secrets map[string]string

	// WorkflowInputs are inputs passed to the workflow.
	WorkflowInputs map[string]any

	// StepOutputs contains outputs from previously executed steps.
	// Keyed by step ID, each value is that step's outputs.
	StepOutputs map[string]StepOutputs

	// Config contains namespaced configuration values.
	Config map[string]map[string]any

	// Volumes are named volumes available to the step.
	Volumes map[string]VolumeRef

	// WorkingDir is the workflow's working directory (workspace root).
	WorkingDir string
}

// StepOutputs contains the outputs produced by a step.
type StepOutputs map[string]string

// VolumeRef references a volume available to a step.
type VolumeRef struct {
	// Name is the volume name.
	Name string

	// MountPath is where the volume should be mounted.
	MountPath string

	// ReadOnly indicates if the mount should be read-only.
	ReadOnly bool

	// HostPath is the source path on the host (for bind mounts).
	HostPath string
}

// StepResult contains the outcome of executing a step.
type StepResult struct {
	// Status indicates whether the step succeeded, failed, or was skipped.
	Status StepStatus

	// ExitCode is the process exit code for run/build steps.
	// Zero indicates success, non-zero indicates failure.
	// May be nil for composite steps or steps that don't produce an exit code.
	ExitCode *int

	// Error contains the error message if the step failed.
	// This is set for infrastructure errors (container start failed, etc.)
	// as opposed to the command itself returning non-zero.
	Error string

	// Outputs are key-value pairs produced by this step.
	// These can be referenced by subsequent steps via step outputs expressions.
	Outputs StepOutputs

	// Logs provides access to the step's log output.
	Logs *StepLogs

	// Duration is how long the step took to execute.
	Duration time.Duration

	// StartedAt is when execution began.
	StartedAt time.Time

	// FinishedAt is when execution completed.
	FinishedAt time.Time

	// Children contains results for child steps (for composite steps).
	Children []StepResult

	// Metadata contains additional step-type-specific information.
	// For example, build steps may include image ID and digest.
	Metadata map[string]string
}

// StepStatus represents the execution outcome of a step.
type StepStatus string

const (
	// StepStatusPending means the step has not started yet.
	StepStatusPending StepStatus = "pending"

	// StepStatusRunning means the step is currently executing.
	StepStatusRunning StepStatus = "running"

	// StepStatusSuccess means the step completed successfully.
	StepStatusSuccess StepStatus = "success"

	// StepStatusFailure means the step failed (non-zero exit or error).
	StepStatusFailure StepStatus = "failure"

	// StepStatusCancelled means the step was cancelled before completion.
	StepStatusCancelled StepStatus = "cancelled"

	// StepStatusSkipped means the step was skipped (e.g., switch branch not taken).
	StepStatusSkipped StepStatus = "skipped"
)

// StepLogs provides access to step log output.
type StepLogs struct {
	// Stdout contains standard output.
	Stdout io.Reader

	// Stderr contains standard error output.
	Stderr io.Reader

	// Combined provides an interleaved stream of stdout and stderr.
	// May be nil if not supported.
	Combined io.Reader
}

// StepProgress reports step execution progress.
type StepProgress struct {
	// StepID identifies which step this progress is for.
	StepID string

	// Status is the current step status.
	Status StepStatus

	// Message is a human-readable progress message.
	Message string

	// Percent is the completion percentage (0-100), or -1 if unknown.
	Percent int

	// Timestamp is when this progress update was generated.
	Timestamp time.Time
}

// StepProgressHandler receives step progress updates.
type StepProgressHandler func(progress StepProgress)

// ExecutionContext provides context for step execution within a workflow.
type ExecutionContext struct {
	// Runtime is the container runtime to use for execution.
	Runtime container.Runtime

	// Network is the network for inter-container communication.
	Network container.Network

	// ProgressHandler receives progress updates during execution.
	ProgressHandler StepProgressHandler

	// LogWriter receives real-time log output.
	LogWriter io.Writer

	// DryRun if true, validates steps without executing them.
	DryRun bool
}
