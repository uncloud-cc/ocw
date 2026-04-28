package ocw

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// Engine: the orchestrator that drives execution
// ---------------------------------------------------------------------------

// Engine orchestrates workflow execution. It converts schema steps into
// runners, drives iteration for composites, and executes leaf steps.
type Engine struct {
	factory  StepFactory
	rt       ContainerRuntime
	services *ServiceRegistry
	logger   *log.Logger
}

// NewEngine creates an Engine backed by the given ContainerRuntime.
func NewEngine(crt ContainerRuntime, logger *log.Logger) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	services := &ServiceRegistry{}
	return &Engine{
		factory:  newStepFactory(crt, services, logger),
		rt:       crt,
		services: services,
		logger:   logger,
	}
}

// Services returns the service registry so callers can inspect running services.
func (e *Engine) Services() *ServiceRegistry {
	return e.services
}

// Shutdown stops all running background services in reverse start order.
// The caller decides when to call this -- the Engine never calls it automatically.
func (e *Engine) Shutdown(ctx context.Context) error {
	all := e.services.All()
	var firstErr error
	// Stop in reverse order (most recently started first).
	for i := len(all) - 1; i >= 0; i-- {
		h := all[i]
		e.logger.Printf("stopping service: %s", h.Name)
		if err := e.rt.StopService(ctx, h); err != nil {
			e.logger.Printf("  error stopping %s: %v", h.Name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// RunWorkflow is the top-level entry point. It determines the flow type
// of the given OCW schema (or job) and executes it.
func (e *Engine) RunWorkflow(ctx context.Context, ocw *schema.OCW, jobName string) (*StepResult, error) {
	scope := NewScope()
	scope.Logger = e.logger
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
		return e.runJob(ctx, ocw, jobName, scope)
	}
	return e.runTopLevel(ctx, ocw, scope)
}

func (e *Engine) runTopLevel(ctx context.Context, ocw *schema.OCW, scope *Scope) (*StepResult, error) {
	switch GetFlowType(ocw) {
	case "parallel":
		return e.runSteps(ctx, e.buildRunners(ocw.Parallel), scope, true)
	case "sequence":
		return e.runSteps(ctx, e.buildRunners(ocw.Sequence), scope, false)
	case "switch":
		return e.runSwitch(ctx, ocw.Switch, ocw.Case, ocw.Default, scope)
	default:
		return nil, fmt.Errorf("workflow has no flow control (parallel, sequence, or switch)")
	}
}

func (e *Engine) runJob(ctx context.Context, ocw *schema.OCW, jobName string, scope *Scope) (*StepResult, error) {
	job := GetJob(ocw, jobName)
	if job == nil {
		return nil, fmt.Errorf("job %q not found", jobName)
	}

	scope.Job = JobMeta{Name: job.Name}

	switch GetJobFlowType(job) {
	case "parallel":
		return e.runSteps(ctx, e.buildRunners(job.Parallel), scope, true)
	case "sequence":
		return e.runSteps(ctx, e.buildRunners(job.Sequence), scope, false)
	case "switch":
		return e.runSwitch(ctx, job.Switch, job.Case, job.Default, scope)
	case "step":
		runner := e.factory(*job.Step)
		return e.runRunner(ctx, runner, scope)
	default:
		return nil, fmt.Errorf("job %q has no flow control", jobName)
	}
}

// runSwitch handles the top-level / job-level switch where Switch, Case, and
// Default are separate fields (not wrapped in a SwitchStep).
func (e *Engine) runSwitch(ctx context.Context, switchExpr *string, cases map[string]schema.Step, def *schema.Step, scope *Scope) (*StepResult, error) {
	if switchExpr == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	value, err := scope.Interpolate(*switchExpr)
	if err != nil {
		return nil, fmt.Errorf("switch expression: %w", err)
	}

	if branch, ok := cases[value]; ok {
		runner := e.factory(branch)
		return e.runRunner(ctx, runner, scope)
	}
	if def != nil {
		runner := e.factory(*def)
		return e.runRunner(ctx, runner, scope)
	}
	e.logger.Printf("switch: no case matched %q and no default", value)
	return &StepResult{Status: StatusSkipped}, nil
}

// buildRunners converts a slice of schema steps to runners.
func (e *Engine) buildRunners(steps []schema.Step) []StepRunner {
	runners := make([]StepRunner, len(steps))
	for i, s := range steps {
		runners[i] = e.factory(s)
	}
	return runners
}

// runSteps executes a batch of runners either sequentially or in parallel.
func (e *Engine) runSteps(ctx context.Context, runners []StepRunner, scope *Scope, parallel bool) (*StepResult, error) {
	if parallel {
		return e.runParallel(ctx, runners, scope)
	}
	return e.runSequential(ctx, runners, scope)
}

func (e *Engine) runSequential(ctx context.Context, runners []StepRunner, scope *Scope) (*StepResult, error) {
	for _, runner := range runners {
		result, err := e.runRunner(ctx, runner, scope)
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

func (e *Engine) runParallel(ctx context.Context, runners []StepRunner, scope *Scope) (*StepResult, error) {
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
			res, err := e.runRunner(ctx, r, s)
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
func (e *Engine) checkNeeds(runner StepRunner) error {
	nc, ok := runner.(needsChecker)
	if !ok {
		return nil
	}
	for _, id := range nc.Needs() {
		h := e.services.Get(id)
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
func (e *Engine) runRunner(ctx context.Context, runner StepRunner, scope *Scope) (*StepResult, error) {
	if err := ctx.Err(); err != nil {
		return &StepResult{Status: StatusFailed, Err: err}, err
	}

	if err := e.checkNeeds(runner); err != nil {
		return &StepResult{ID: runner.ID(), Status: StatusFailed, Err: err}, err
	}

	e.logger.Printf("step: %s", runnerLabel(runner))

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
				result, err := e.runRunner(ctx, batch[0], scope)
				if err != nil {
					return result, err
				}
				if result.Status == StatusFailed {
					return result, result.Err
				}
				scope.Merge(result.ID, result.Output)
				prev = []*StepResult{result}
			} else if len(batch) > 1 {
				result, err := e.runParallel(ctx, batch, scope)
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
