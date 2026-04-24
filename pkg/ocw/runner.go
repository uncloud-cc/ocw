package ocw

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// Scope: the interpolation context that accumulates as steps complete
// ---------------------------------------------------------------------------

// StepOutput holds the key-value outputs from a completed step.
type StepOutput struct {
	Values map[string]string
}

// WorkflowMeta holds metadata about the current workflow.
type WorkflowMeta struct {
	Name string
}

// JobMeta holds metadata about the current job.
type JobMeta struct {
	Name string
}

// Scope is the interpolation context that flows through the workflow.
// Steps read from it and contribute outputs back.
type Scope struct {
	Env      map[string]string
	Secrets  map[string]string
	Steps    map[string]StepOutput // keyed by step ID
	Workflow WorkflowMeta
	Job      JobMeta
	Logger   *log.Logger // for interpolation warnings
}

// NewScope creates a Scope with initialized maps.
func NewScope() *Scope {
	return &Scope{
		Env:     make(map[string]string),
		Secrets: make(map[string]string),
		Steps:   make(map[string]StepOutput),
	}
}

// Clone returns a deep copy. Parallel branches receive cloned scopes
// so they cannot see each other's mutations.
func (s *Scope) Clone() *Scope {
	c := &Scope{
		Env:      make(map[string]string, len(s.Env)),
		Secrets:  make(map[string]string, len(s.Secrets)),
		Steps:    make(map[string]StepOutput, len(s.Steps)),
		Workflow: s.Workflow,
		Job:      s.Job,
		Logger:   s.Logger,
	}
	for k, v := range s.Env {
		c.Env[k] = v
	}
	for k, v := range s.Secrets {
		c.Secrets[k] = v
	}
	for id, out := range s.Steps {
		vals := make(map[string]string, len(out.Values))
		for k, v := range out.Values {
			vals[k] = v
		}
		c.Steps[id] = StepOutput{Values: vals}
	}
	return c
}

// Merge adds the outputs of a completed step into this scope.
func (s *Scope) Merge(id string, output StepOutput) {
	if id != "" {
		s.Steps[id] = output
	}
}

// templatePattern matches {{ ... }} with arbitrary internal whitespace.
var templatePattern = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

// Interpolate performs template substitution on a string.
//
// Supported references:
//   - {{ env.VAR }}         environment variable (warns if unresolved)
//   - {{ secrets.NAME }}    secret value
//   - {{ steps.ID.KEY }}    output from a completed step
//   - {{ workflow.name }}   workflow name
//   - {{ job.name }}        current job name
//
// Whitespace inside {{ }} is flexible: {{ env.X }}, {{env.X}}, {{  env.X  }}
// all work. Unresolved references return an error, except for env references
// where the variable exists in the host environment but not in the workflow
// env block -- those produce a warning and leave the template text in place.
// Env references that are neither in scope nor in the host environment are
// hard errors.
func (s *Scope) Interpolate(tmpl string) (string, error) {
	var errors []string

	result := templatePattern.ReplaceAllStringFunc(tmpl, func(match string) string {
		// Extract the inner expression, trimming delimiters and whitespace.
		inner := templatePattern.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		expr := strings.TrimSpace(inner[1])
		parts := strings.SplitN(expr, ".", 2)

		if len(parts) < 2 {
			errors = append(errors, fmt.Sprintf("invalid template expression %q: expected namespace.key", expr))
			return match
		}

		namespace := parts[0]
		key := parts[1]

		switch namespace {
		case "env":
			if v, ok := s.Env[key]; ok {
				return v
			}
			// Not in scope -- check if it exists in the host environment.
			if _, exists := os.LookupEnv(key); exists {
				// Set in the host but not in the OCW env block: warn, leave as-is.
				if s.Logger != nil {
					s.Logger.Printf("warning: {{ env.%s }} is not declared in the workflow env block but is set in the host environment", key)
				}
				return match
			}
			// Not in scope and not set anywhere: hard error.
			errors = append(errors, fmt.Sprintf("environment variable %q is not set", key))
			return match

		case "secrets":
			if v, ok := s.Secrets[key]; ok {
				return v
			}
			errors = append(errors, fmt.Sprintf("unresolved secret %q", key))
			return match

		case "steps":
			// key is "ID.FIELD" -- split once more.
			stepParts := strings.SplitN(key, ".", 2)
			if len(stepParts) < 2 {
				errors = append(errors, fmt.Sprintf("invalid step reference %q: expected steps.ID.key", expr))
				return match
			}
			stepID, field := stepParts[0], stepParts[1]
			if out, ok := s.Steps[stepID]; ok {
				if v, ok := out.Values[field]; ok {
					return v
				}
				errors = append(errors, fmt.Sprintf("step %q has no output %q", stepID, field))
				return match
			}
			errors = append(errors, fmt.Sprintf("step %q not found (referenced by {{ %s }})", stepID, expr))
			return match

		case "workflow":
			switch key {
			case "name":
				return s.Workflow.Name
			default:
				errors = append(errors, fmt.Sprintf("unknown workflow property %q", key))
				return match
			}

		case "job":
			switch key {
			case "name":
				return s.Job.Name
			default:
				errors = append(errors, fmt.Sprintf("unknown job property %q", key))
				return match
			}

		default:
			errors = append(errors, fmt.Sprintf("unknown template namespace %q in {{ %s }}", namespace, expr))
			return match
		}
	})

	if len(errors) > 0 {
		return result, fmt.Errorf("template interpolation: %s", strings.Join(errors, "; "))
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Step results
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Services: background containers that outlive individual step execution
// ---------------------------------------------------------------------------

// ServiceHandle is an opaque reference to a running background container.
// The ContainerRuntime creates it; the Runtime holds it for later cleanup.
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
// The Runtime owns one registry per RunWorkflow call.
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

// ---------------------------------------------------------------------------
// ContainerRuntime: the abstraction over Docker/Podman/etc.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// StepRunner: the unified interface for executable steps
// ---------------------------------------------------------------------------

// StepRunner is what the runtime operates on. Every schema.Step becomes
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
// The runtime calls Next repeatedly, executing what comes back,
// then feeding results into the next call.
type StepIterator interface {
	// Next returns the next batch of steps to execute.
	// Pass nil on the first call; pass previous results on subsequent calls.
	// When done, returns (nil, true, nil).
	// Multiple returned runners should be executed in parallel.
	Next(prev []*StepResult) (runners []StepRunner, done bool, err error)
}

// ---------------------------------------------------------------------------
// StepFactory: converts schema.Step -> StepRunner
// ---------------------------------------------------------------------------

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

// serviceRunner starts a background container via the ContainerRuntime and
// registers it with the Runtime's ServiceRegistry. The step completes once
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
// then drive it the same way the top-level runtime does.
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

// ---------------------------------------------------------------------------
// Runtime: the orchestrator that drives execution
// ---------------------------------------------------------------------------

// Runtime orchestrates workflow execution. It converts schema steps into
// runners, drives iteration for composites, and executes leaf steps.
type Runtime struct {
	factory  StepFactory
	rt       ContainerRuntime
	services *ServiceRegistry
	logger   *log.Logger
}

// NewRuntime creates a Runtime backed by the given ContainerRuntime.
func NewRuntime(crt ContainerRuntime, logger *log.Logger) *Runtime {
	if logger == nil {
		logger = log.Default()
	}
	services := &ServiceRegistry{}
	return &Runtime{
		factory:  newStepFactory(crt, services, logger),
		rt:       crt,
		services: services,
		logger:   logger,
	}
}

// Services returns the service registry so callers can inspect running services.
func (rt *Runtime) Services() *ServiceRegistry {
	return rt.services
}

// Shutdown stops all running background services in reverse start order.
// The caller decides when to call this -- the Runtime never calls it automatically.
func (rt *Runtime) Shutdown(ctx context.Context) error {
	all := rt.services.All()
	var firstErr error
	// Stop in reverse order (most recently started first).
	for i := len(all) - 1; i >= 0; i-- {
		h := all[i]
		rt.logger.Printf("stopping service: %s", h.Name)
		if err := rt.rt.StopService(ctx, h); err != nil {
			rt.logger.Printf("  error stopping %s: %v", h.Name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// RunWorkflow is the top-level entry point. It determines the flow type
// of the given OCW schema (or job) and executes it.
func (rt *Runtime) RunWorkflow(ctx context.Context, ocw *schema.OCW, jobName string) (*StepResult, error) {
	scope := NewScope()
	scope.Logger = rt.logger
	scope.Workflow = WorkflowMeta{Name: ocw.Name}

	// Seed scope with workflow-level env and secrets.
	for k, v := range ocw.Env {
		scope.Env[k] = v.Value
	}
	for k, v := range ocw.Secrets {
		if v.Plain != "" {
			scope.Secrets[k] = v.Plain
		}
	}

	if jobName != "" {
		return rt.runJob(ctx, ocw, jobName, scope)
	}
	return rt.runTopLevel(ctx, ocw, scope)
}

func (rt *Runtime) runTopLevel(ctx context.Context, ocw *schema.OCW, scope *Scope) (*StepResult, error) {
	switch GetFlowType(ocw) {
	case "parallel":
		return rt.runSteps(ctx, rt.buildRunners(ocw.Parallel), scope, true)
	case "sequence":
		return rt.runSteps(ctx, rt.buildRunners(ocw.Sequence), scope, false)
	case "switch":
		return rt.runSwitch(ctx, ocw.Switch, ocw.Case, ocw.Default, scope)
	default:
		return nil, fmt.Errorf("workflow has no flow control (parallel, sequence, or switch)")
	}
}

func (rt *Runtime) runJob(ctx context.Context, ocw *schema.OCW, jobName string, scope *Scope) (*StepResult, error) {
	job := GetJob(ocw, jobName)
	if job == nil {
		return nil, fmt.Errorf("job %q not found", jobName)
	}

	scope.Job = JobMeta{Name: job.Name}

	switch GetJobFlowType(job) {
	case "parallel":
		return rt.runSteps(ctx, rt.buildRunners(job.Parallel), scope, true)
	case "sequence":
		return rt.runSteps(ctx, rt.buildRunners(job.Sequence), scope, false)
	case "switch":
		return rt.runSwitch(ctx, job.Switch, job.Case, job.Default, scope)
	case "step":
		runner := rt.factory(*job.Step)
		return rt.runRunner(ctx, runner, scope)
	default:
		return nil, fmt.Errorf("job %q has no flow control", jobName)
	}
}

// runSwitch handles the top-level / job-level switch where Switch, Case, and
// Default are separate fields (not wrapped in a SwitchStep).
func (rt *Runtime) runSwitch(ctx context.Context, switchExpr *string, cases map[string]schema.Step, def *schema.Step, scope *Scope) (*StepResult, error) {
	if switchExpr == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	value, err := scope.Interpolate(*switchExpr)
	if err != nil {
		return nil, fmt.Errorf("switch expression: %w", err)
	}

	if branch, ok := cases[value]; ok {
		runner := rt.factory(branch)
		return rt.runRunner(ctx, runner, scope)
	}
	if def != nil {
		runner := rt.factory(*def)
		return rt.runRunner(ctx, runner, scope)
	}
	rt.logger.Printf("switch: no case matched %q and no default", value)
	return &StepResult{Status: StatusSkipped}, nil
}

// buildRunners converts a slice of schema steps to runners.
func (rt *Runtime) buildRunners(steps []schema.Step) []StepRunner {
	runners := make([]StepRunner, len(steps))
	for i, s := range steps {
		runners[i] = rt.factory(s)
	}
	return runners
}

// runSteps executes a batch of runners either sequentially or in parallel.
func (rt *Runtime) runSteps(ctx context.Context, runners []StepRunner, scope *Scope, parallel bool) (*StepResult, error) {
	if parallel {
		return rt.runParallel(ctx, runners, scope)
	}
	return rt.runSequential(ctx, runners, scope)
}

func (rt *Runtime) runSequential(ctx context.Context, runners []StepRunner, scope *Scope) (*StepResult, error) {
	for _, runner := range runners {
		result, err := rt.runRunner(ctx, runner, scope)
		if err != nil {
			return result, err
		}
		if result.Status == StatusFailed {
			return result, result.Err
		}
		scope.Merge(result.ID, result.Output)
	}
	return &StepResult{Status: StatusSuccess}, nil
}

func (rt *Runtime) runParallel(ctx context.Context, runners []StepRunner, scope *Scope) (*StepResult, error) {
	type indexedResult struct {
		index  int
		result *StepResult
		err    error
	}

	results := make(chan indexedResult, len(runners))
	var wg sync.WaitGroup

	for i, runner := range runners {
		wg.Add(1)
		go func(idx int, r StepRunner, s *Scope) {
			defer wg.Done()
			res, err := rt.runRunner(ctx, r, s)
			results <- indexedResult{index: idx, result: res, err: err}
		}(i, runner, scope.Clone())
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for ir := range results {
		if ir.err != nil && firstErr == nil {
			firstErr = ir.err
		}
		if ir.result != nil && ir.result.Status == StatusFailed && firstErr == nil {
			firstErr = ir.result.Err
		}
	}
	if firstErr != nil {
		return &StepResult{Status: StatusFailed, Err: firstErr}, firstErr
	}
	return &StepResult{Status: StatusSuccess}, nil
}

// needsChecker is implemented by runners that have service dependencies.
type needsChecker interface {
	Needs() []string
}

// checkNeeds verifies that all services listed in a step's needs are
// registered and healthy. Returns an error if any dependency is missing.
func (rt *Runtime) checkNeeds(runner StepRunner) error {
	nc, ok := runner.(needsChecker)
	if !ok {
		return nil
	}
	for _, id := range nc.Needs() {
		h := rt.services.Get(id)
		if h == nil {
			return fmt.Errorf("step %q needs service %q, but it is not running", runner.Name(), id)
		}
		if !h.Healthy {
			return fmt.Errorf("step %q needs service %q, but it is not healthy", runner.Name(), id)
		}
	}
	return nil
}

// runRunner is the recursive core. It inspects the runner type and
// either executes it directly (simple) or drives its iterator (composite).
func (rt *Runtime) runRunner(ctx context.Context, runner StepRunner, scope *Scope) (*StepResult, error) {
	if err := ctx.Err(); err != nil {
		return &StepResult{Status: StatusFailed, Err: err}, err
	}

	if err := rt.checkNeeds(runner); err != nil {
		return &StepResult{ID: runner.ID(), Status: StatusFailed, Err: err}, err
	}

	rt.logger.Printf("step: %s", runnerLabel(runner))

	switch r := runner.(type) {
	case SimpleRunner:
		result, err := r.Execute(ctx, scope)
		if err != nil {
			return &StepResult{ID: runner.ID(), Status: StatusFailed, Err: err}, err
		}
		result.ID = runner.ID()
		return result, nil

	case CompositeRunner:
		iter := r.Iterator(scope)
		var prev []*StepResult

		for {
			batch, done, err := iter.Next(prev)
			if err != nil {
				return &StepResult{ID: runner.ID(), Status: StatusFailed, Err: err}, err
			}
			if done {
				break
			}

			if len(batch) == 1 {
				result, err := rt.runRunner(ctx, batch[0], scope)
				if err != nil {
					return result, err
				}
				if result.Status == StatusFailed {
					return result, result.Err
				}
				scope.Merge(result.ID, result.Output)
				prev = []*StepResult{result}
			} else if len(batch) > 1 {
				result, err := rt.runParallel(ctx, batch, scope)
				if err != nil {
					return result, err
				}
				prev = []*StepResult{result}
			}
		}

		return &StepResult{ID: runner.ID(), Status: StatusSuccess}, nil

	default:
		return nil, fmt.Errorf("runner %q does not implement SimpleRunner or CompositeRunner", runner.ID())
	}
}

func runnerLabel(r StepRunner) string {
	name := r.Name()
	id := r.ID()
	if name != "" && id != "" {
		return fmt.Sprintf("%s (%s)", name, id)
	}
	if name != "" {
		return name
	}
	if id != "" {
		return id
	}
	return "(anonymous)"
}

// ---------------------------------------------------------------------------
// DummyRuntime: a ContainerRuntime that logs but does nothing
// ---------------------------------------------------------------------------

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
