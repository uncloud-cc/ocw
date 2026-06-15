package ocw

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// EngineOptions allows injecting a custom Runtime and selecting a specific job.
type EngineOptions struct {
	Runtime Runtime // if nil, DockerRuntime is used
	JobName string  // if empty, compiles the top-level flow
}

// Engine encapsulates a compiled workflow, state, event bus, and runtime.
// Compilation happens during construction so Run() is ready to execute immediately.
type Engine struct {
	Bus      *EventBus      // public so callers can subscribe before running
	State    *State         // public so callers can read inputs, outputs, and runID
	Workflow *flow.Workflow // public so callers can inspect the compiled graph

	runtime Runtime
	schema  *schema.OCW
	baseDir string
}

// NewEngine creates an Engine from a parsed schema, a prepared state, and a base directory.
// The caller is responsible for building the state (including env vars, secrets, and inputs).
// The workflow is compiled immediately. If opts.Runtime is nil, a DockerRuntime is created.
func NewEngine(
	schema *schema.OCW,
	state *State,
	baseDir string,
	opts EngineOptions,
) (*Engine, error) {
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}

	bus := NewEventBus()
	bus.SetSecrets(false, state.GetSecretValues())

	state.Meta["name"] = schema.Name
	if state.Steps == nil {
		state.Steps = make(map[string]map[string]string)
	}

	var runtime Runtime
	if opts.Runtime != nil {
		runtime = opts.Runtime
	} else {
		var err error
		runtime, err = NewDockerRuntime(
			schema.Volumes,
			baseDir,
			state.RunID,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create docker runtime: %w", err)
		}
	}

	workflow, err := Compile(schema, opts.JobName, runtime, state, bus, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot compile workflow: %w", err)
	}

	return &Engine{
		Bus:      bus,
		State:    state,
		Workflow: workflow,
		runtime:  runtime,
		schema:   schema,
		baseDir:  baseDir,
	}, nil
}

// Run executes the pre-compiled workflow.
func (e *Engine) Run(ctx context.Context) error {
	return e.Workflow.Do(ctx)
}

// ResolvedOutputs returns the resolved outputs for the compiled workflow.
func (e *Engine) ResolvedOutputs() (map[string]string, error) {
	var rawOutputs map[string]string
	if e.schema.Outputs != nil {
		rawOutputs = e.schema.Outputs
	}
	return e.State.ResolveOutputs(rawOutputs)
}

// Close cleans up the runtime.
func (e *Engine) Close() error {
	if e.runtime != nil {
		return e.runtime.Close()
	}
	return nil
}
