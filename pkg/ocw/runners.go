package ocw

import (
	"context"
	"fmt"
	"log"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// StepStatus represents the outcome of a step execution.
type StepStatus int

const (
	StatusSuccess StepStatus = iota
	StatusFailed
	StatusSkipped
)

// StepResult is the outcome of running a single step.
type StepResult struct {
	ID     string
	Status StepStatus
	Output StepOutput
	Err    error
}

// StepRunner is what the engine operates on. Every schema.Step becomes
// a StepRunner -- either a simple (leaf) runner or a composite (iterator).
type StepRunner interface {
	// ID returns the step's identifier (user-provided or synthetic).
	ID() string
	// Name returns the human-readable name.
	Name() string
}

// SimpleRunner is a leaf step that does actual work (Run, Build).
type SimpleRunner interface {
	StepRunner
	Execute(ctx context.Context, scope *Scope) (*StepResult, error)
}

// CompositeRunner is a control-flow step that yields child steps.
type CompositeRunner interface {
	StepRunner
	Iterator(scope *Scope) StepIterator
}

// StepIterator is the core iteration protocol.
// The engine calls Next repeatedly, executing what comes back,
// then feeding results into the next call.
type StepIterator interface {
	// Next returns the next batch of steps to execute.
	// Pass nil on the first call; pass previous results on subsequent calls.
	// When done, returns (nil, true, nil).
	// Multiple returned runners should be executed in parallel.
	Next(prev []*StepResult) (runners []StepRunner, done bool, err error)
}

// StepFactory converts a parsed schema step into an executable runner.
type StepFactory func(step schema.Step) StepRunner

// newStepFactory creates a factory that dispatches schema.Step variants
// to the correct runner type. Background RunSteps become serviceRunners.
func newStepFactory(rt ContainerRuntime, services *ServiceRegistry, logger *log.Logger) StepFactory {
	var f StepFactory
	f = func(step schema.Step) StepRunner {
		switch {
		case step.RunStep != nil:
			if step.RunStep.Background {
				return &serviceRunner{step: step.RunStep, rt: rt, services: services, logger: logger}
			}
			return &runRunner{step: step.RunStep, rt: rt}
		case step.BuildStep != nil:
			return &buildRunner{step: step.BuildStep, rt: rt}
		case step.SequenceStep != nil:
			return &sequenceRunner{step: step.SequenceStep, factory: f}
		case step.ParallelStep != nil:
			return &parallelRunner{step: step.ParallelStep, factory: f}
		case step.SwitchStep != nil:
			return &switchRunner{step: step.SwitchStep, factory: f}
		case step.WorkflowStep != nil:
			return &workflowRunner{step: step.WorkflowStep, factory: f}
		default:
			panic("unrecognized step type in factory")
		}
	}
	return f
}

// ---------------------------------------------------------------------------
// Concrete runners: leaf steps
// ---------------------------------------------------------------------------

// runRunner executes a container via the ContainerRuntime.
type runRunner struct {
	step *schema.RunStep
	rt   ContainerRuntime
}

func (r *runRunner) ID() string      { return r.step.ID }
func (r *runRunner) Name() string    { return r.step.Name }
func (r *runRunner) Needs() []string { return r.step.Needs }

func (r *runRunner) Execute(ctx context.Context, scope *Scope) (*StepResult, error) {
	return r.rt.Run(ctx, r.step, scope)
}

// buildRunner builds an image via the ContainerRuntime.
type buildRunner struct {
	step *schema.BuildStep
	rt   ContainerRuntime
}

func (b *buildRunner) ID() string      { return b.step.ID }
func (b *buildRunner) Name() string    { return b.step.Name }
func (b *buildRunner) Needs() []string { return b.step.Needs }

func (b *buildRunner) Execute(ctx context.Context, scope *Scope) (*StepResult, error) {
	return b.rt.Build(ctx, b.step, scope)
}

// ---------------------------------------------------------------------------
// Concrete runners: composite steps
// ---------------------------------------------------------------------------

// --- Sequence ---

type sequenceRunner struct {
	step    *schema.SequenceStep
	factory StepFactory
}

func (s *sequenceRunner) ID() string   { return s.step.ID }
func (s *sequenceRunner) Name() string { return s.step.Name }

func (s *sequenceRunner) Iterator(scope *Scope) StepIterator {
	return &sequenceIterator{
		steps:   s.step.Sequence,
		factory: s.factory,
		index:   0,
	}
}

type sequenceIterator struct {
	steps   []schema.Step
	factory StepFactory
	index   int
}

func (it *sequenceIterator) Next(_ []*StepResult) ([]StepRunner, bool, error) {
	if it.index >= len(it.steps) {
		return nil, true, nil
	}
	runner := it.factory(it.steps[it.index])
	it.index++
	return []StepRunner{runner}, false, nil
}

// --- Parallel ---

type parallelRunner struct {
	step    *schema.ParallelStep
	factory StepFactory
}

func (p *parallelRunner) ID() string   { return p.step.ID }
func (p *parallelRunner) Name() string { return p.step.Name }

func (p *parallelRunner) Iterator(scope *Scope) StepIterator {
	return &parallelIterator{
		steps:   p.step.Parallel,
		factory: p.factory,
	}
}

type parallelIterator struct {
	steps   []schema.Step
	factory StepFactory
	emitted bool
}

func (it *parallelIterator) Next(_ []*StepResult) ([]StepRunner, bool, error) {
	if it.emitted {
		return nil, true, nil
	}
	it.emitted = true
	runners := make([]StepRunner, len(it.steps))
	for i, s := range it.steps {
		runners[i] = it.factory(s)
	}
	return runners, false, nil
}

// --- Switch ---

type switchRunner struct {
	step    *schema.SwitchStep
	factory StepFactory
}

func (sw *switchRunner) ID() string   { return sw.step.ID }
func (sw *switchRunner) Name() string { return sw.step.Name }

func (sw *switchRunner) Iterator(scope *Scope) StepIterator {
	return &switchIterator{
		step:    sw.step,
		scope:   scope,
		factory: sw.factory,
	}
}

type switchIterator struct {
	step    *schema.SwitchStep
	scope   *Scope
	factory StepFactory
	emitted bool
}

func (it *switchIterator) Next(_ []*StepResult) ([]StepRunner, bool, error) {
	if it.emitted {
		return nil, true, nil
	}
	it.emitted = true

	value, err := it.scope.Interpolate(it.step.Switch)
	if err != nil {
		return nil, false, fmt.Errorf("switch expression: %w", err)
	}

	if branch, ok := it.step.Case[value]; ok {
		runner := it.factory(branch)
		return []StepRunner{runner}, false, nil
	}
	if it.step.Default != nil {
		runner := it.factory(*it.step.Default)
		return []StepRunner{runner}, false, nil
	}
	// No match and no default: nothing to run.
	return nil, true, nil
}

// --- Workflow (sub-workflow) ---

// workflowRunner is a placeholder for sub-workflow invocation.
// A real implementation would load and parse the referenced workflow,
// then drive it the same way the top-level engine does.
type workflowRunner struct {
	step    *schema.WorkflowStep
	factory StepFactory
}

func (w *workflowRunner) ID() string   { return w.step.ID }
func (w *workflowRunner) Name() string { return w.step.Name }

func (w *workflowRunner) Iterator(_ *Scope) StepIterator {
	return &workflowIterator{step: w.step}
}

type workflowIterator struct {
	step *schema.WorkflowStep
}

func (it *workflowIterator) Next(_ []*StepResult) ([]StepRunner, bool, error) {
	// TODO: load the referenced workflow, parse it, and iterate its steps.
	return nil, true, fmt.Errorf("workflow step %q: sub-workflow execution not yet implemented", it.step.Workflow.From)
}
