package workflow

import (
	"context"
	"fmt"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// Engine drives workflow execution.
// It maintains pending/current step queues and executes steps through
// an interpolate -> execute -> merge cycle.
type Engine struct {
	ocw     *schema.OCW
	jobName string
	ctx     *StepContext

	// pending holds steps that have not yet been executed
	pending []Step

	// current holds steps that are ready to execute (after interpolation)
	current []Step
}

// WorkflowMeta contains metadata about the current workflow.
type WorkflowMeta struct {
	Name string
	ID   string
	Path string // File path (for workflow step resolution)
}

// ServiceInfo tracks a running background service.
type ServiceInfo struct {
	StepID      string
	ContainerID string // Set by caller after container starts
	Healthy     bool   // Set by caller after healthcheck passes
	Ports       []PortMapping
}

// PortMapping maps container port to host port.
type PortMapping struct {
	ContainerPort int
	HostPort      int
	Protocol      string
}

// New creates a workflow engine for the given schema and job.
// If jobName is empty, uses the default flow (direct parallel/sequence/switch).
func New(ocw *schema.OCW, jobName string, ctx *StepContext) (*Engine, error) {
	if ocw == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// Validate schema
	if err := ocw.Validate(); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	// Find entry point
	entrySteps, err := findEntryPoint(ocw, jobName)
	if err != nil {
		return nil, fmt.Errorf("failed to find entry point: %w", err)
	}

	// Create initial steps via factory
	if ctx.Factory == nil {
		return nil, fmt.Errorf("StepContext.Factory is nil")
	}
	current := make([]Step, len(entrySteps))
	for i, s := range entrySteps {
		step, err := ctx.Factory.Create(&s, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry step %d: %w", i, err)
		}
		current[i] = step
	}

	// Initialize context workflow metadata
	ctx.Workflow = WorkflowMeta{
		Name: string(ocw.Name),
		ID:   string(ocw.ID),
	}

	return &Engine{
		ocw:     ocw,
		jobName: jobName,
		ctx:     ctx,
		current: current,
		pending: []Step{},
	}, nil
}

// Execute runs one iteration of the engine loop:
//  1. Interpolate: resolve {{ expr }} templates in current steps using StepContext
//  2. Execute: run all current steps in parallel
//  3. Merge: collect outputs from all steps and merge into StepContext
//  4. Advance: move next steps from pending to current (or from step children)
//
// Returns error if any step fails.
func (e *Engine) Execute(ctx context.Context) error {
	if len(e.current) == 0 {
		// Nothing to execute
		return nil
	}

	// Execute all steps in current concurrently.
	// Each step is interpolated just before execution so prior steps' outputs
	// are available in the context for template resolution.
	originalCurrent := e.current

	// Concurrent execution setup
	var wg sync.WaitGroup
	type stepResult struct {
		index  int
		result *StepResult
		err    error
	}
	results := make([]stepResult, len(originalCurrent))
	var mu sync.Mutex
	var firstErr error

	for i, step := range originalCurrent {
		wg.Add(1)
		go func(idx int, s Step) {
			defer wg.Done()

			opts := ExecuteOptions{
				Logger: nil, // TODO: accept logger from caller
			}

			// Interpolate templates immediately before execution.
			// This ensures prior steps' outputs are available for template resolution.
			// Safe to do concurrently since we're only reading from e.ctx.Steps.
			if err := Interpolate(s, e.ctx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("interpolate step %s: %w", s.Name(), err)
				}
				mu.Unlock()
				results[idx] = stepResult{index: idx, err: err}
				return
			}

			result, err := s.Execute(ctx, e.ctx, opts)

			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("execute step %s: %w", s.Name(), err)
			}
			// Merge outputs while holding lock to ensure safe concurrent writes
			if result != nil {
				if result.StepID != "" && len(result.Outputs) > 0 {
					if e.ctx.Steps == nil {
						e.ctx.Steps = make(map[string]map[string]string)
					}
					e.ctx.Steps[result.StepID] = result.Outputs
				}
				if result.Service != nil {
					if e.ctx.Services == nil {
						e.ctx.Services = make(map[string]*ServiceInfo)
					}
					e.ctx.Services[result.StepID] = result.Service
				}
			}
			mu.Unlock()

			results[idx] = stepResult{index: idx, result: result, err: err}
		}(i, step)
	}

	wg.Wait()

	// Return first error encountered
	if firstErr != nil {
		return firstErr
	}

	// Handle children from composite steps (only first composite step wins)
	// Process in order to maintain deterministic behavior
	for i, sr := range results {
		if sr.err != nil {
			continue // Already handled above
		}

		result := sr.result
		if result == nil || len(result.Children) == 0 {
			continue
		}

		if result.Parallel {
			// Parallel children go directly to current for next iteration
			// Append any remaining original current steps after the parallel composite
			if i+1 < len(originalCurrent) {
				e.current = append(result.Children, originalCurrent[i+1:]...)
			} else {
				e.current = result.Children
			}
			return nil // Stop processing other results, we'll handle them in next iteration
		} else {
			// Sequential: execute children immediately within this call
			// Put remaining children in pending, then recursively execute first child
			var newPending []Step
			if len(result.Children) > 1 {
				newPending = append(newPending, result.Children[1:]...)
			}
			// Any remaining original current steps after this one should also go to pending
			if i+1 < len(originalCurrent) {
				newPending = append(newPending, originalCurrent[i+1:]...)
			}
			if len(newPending) > 0 {
				e.pending = append(newPending, e.pending...)
			}
			// Set up first child to execute immediately
			e.current = []Step{result.Children[0]}
			// Recursively execute the sequential child (without returning)
			return e.Execute(ctx)
		}
	}

	// All steps completed with no children produced
	e.current = []Step{}

	// If there's nothing in current but we have pending, move first pending to current
	if len(e.current) == 0 && len(e.pending) > 0 {
		e.current = []Step{e.pending[0]}
		e.pending = e.pending[1:]
		// Recursively execute the pending step
		return e.Execute(ctx)
	}

	return nil
}

// Done returns true when there are no more steps to execute.
func (e *Engine) Done() bool {
	return len(e.pending) == 0 && len(e.current) == 0
}

// Context returns the current step context for inspection.
func (e *Engine) Context() *StepContext {
	return e.ctx
}

// Current returns the steps that are ready for execution.
// Useful for inspection/debugging before calling Execute.
func (e *Engine) Current() []Step {
	return e.current
}

// Pending returns the steps waiting to be executed.
// Useful for inspection/debugging.
func (e *Engine) Pending() []Step {
	return e.pending
}

// -----------------------------------------------------------------------------
// Entry Point Resolution (stubs)
// -----------------------------------------------------------------------------

// findEntryPoint returns the starting steps for a workflow based on jobName.
// If jobName is empty, uses direct flow control (parallel/sequence/switch).
func findEntryPoint(ocwSchema *schema.OCW, jobName string) ([]schema.Step, error) {
	// If jobName is specified, look for that job
	if jobName != "" {
		job := ocw.GetJob(ocwSchema, jobName)
		if job == nil {
			return nil, fmt.Errorf("job not found: %s", jobName)
		}
		return getJobEntrySteps(job)
	}

	// No job specified - use direct flow
	flowType := ocw.GetFlowType(ocwSchema)
	switch flowType {
	case "parallel":
		return ocwSchema.Parallel, nil
	case "sequence":
		// Return a synthetic sequence step to ensure sequential execution
		return []schema.Step{{
			SequenceStep: &schema.SequenceStep{
				OptionalStepBase: schema.OptionalStepBase{
					Name: "root-sequence",
				},
				Sequence: ocwSchema.Sequence,
			},
		}}, nil
	case "switch":
		// Return a synthetic switch step
		return []schema.Step{{
			SwitchStep: &schema.SwitchStep{
				Switch:  *ocwSchema.Switch,
				Case:    ocwSchema.Case,
				Default: ocwSchema.Default,
			},
		}}, nil
	}

	// No direct flow - check for single job
	if len(ocwSchema.Jobs) == 1 {
		for _, job := range ocwSchema.Jobs {
			return getJobEntrySteps(&job)
		}
	}

	if len(ocwSchema.Jobs) > 1 {
		return nil, fmt.Errorf("multiple jobs defined, specify which to run: %v", ocw.GetJobNames(ocwSchema))
	}

	return nil, fmt.Errorf("no jobs or direct flow defined")
}

func getJobEntrySteps(job *schema.Job) ([]schema.Step, error) {
	// Check for switch-based job
	if job.Switch != nil {
		return []schema.Step{{
			SwitchStep: &schema.SwitchStep{
				Switch:  *job.Switch,
				Case:    job.Case,
				Default: job.Default,
			},
		}}, nil
	}

	// Get job steps using ocw helper
	steps := ocw.GetJobSteps(job)
	if len(steps) == 0 {
		return nil, fmt.Errorf("job has no steps")
	}

	// Wrap job steps in a sequence step to ensure sequential execution
	// Jobs should execute their steps sequentially by default
	return []schema.Step{{
		SequenceStep: &schema.SequenceStep{
			OptionalStepBase: schema.OptionalStepBase{
				Name: schema.Name(job.Name),
			},
			Sequence: steps,
		},
	}}, nil
}
