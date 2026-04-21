# OCW Workflow Engine Design

The workflow engine executes OCW workflows by iterating through steps, interpolating inputs from context, executing them (potentially in parallel), and merging outputs back into context.

## Core State

```
Pending []Step      // Steps yet to be executed
Current []Step      // Steps being executed (after interpolation)
Context *StepContext // Current input/output state
```

## Execution Model

The engine's `Execute()` method runs a single iteration:

1. **Interpolate**: Go through all steps in `Current` and resolve any `{{ expr }}` templates based on `StepContext`
2. **Execute**: Run `Execute()` on all steps in `Current` in parallel
3. **Merge**: Collect results and step outputs, merge them into `StepContext`
4. **Advance**: Move next steps from `Pending` to `Current`

## Step Interface

Steps are a simple interface that different step types implement:

```go
type Step interface {
    // Type returns the step type ("run", "build", "sequence", etc.)
    Type() string
    
    // ID returns the step identifier for output tracking
    ID() string
    
    // Execute runs the step and returns results
    // Leaf steps (run, build) execute and return outputs
    // Composite steps (sequence, parallel, switch) return child steps
    Execute(ctx context.Context, stepCtx *StepContext) (*StepResult, error)
}
```

## Step Types

**Leaf Steps** - Execute actual work:
- `run`: Execute a container, return outputs
- `build`: Build an image, return image reference

**Composite Steps** - Return child steps:
- `sequence`: Return children to be executed sequentially
- `parallel`: Return children to be executed in parallel
- `switch`: Evaluate condition, return matching branch
- `workflow`: Load external workflow, return its entry steps

## Step Result

```go
type StepResult struct {
    Outputs  map[string]string  // Key-value outputs to merge into context
    Children []Step             // Child steps (for composite steps)
    Parallel bool               // Whether children should run in parallel
}
```

## Engine Loop

```go
engine := workflow.NewEngine(schema, jobName, ctx)

for !engine.Done() {
    if err := engine.Execute(ctx); err != nil {
        return err
    }
}
```

The caller doesn't need to manage step execution - the engine handles:
- Interpolation of templates
- Parallel execution of steps
- Merging outputs into context
- Advancing to next steps

## Template Resolution

Templates (`{{ expr }}`) are resolved during the interpolation phase:

- `{{ env.VAR }}` - Environment variable
- `{{ secrets.NAME }}` - Secret value
- `{{ steps.ID.key }}` - Output from previous step
- `{{ inputs.NAME }}` - Workflow input

## File Structure

```
pkg/ocw/workflow/
├── engine.go           # Engine struct and Execute() loop
├── context.go          # StepContext (env, secrets, steps outputs)
├── step.go             # Step interface and StepResult
├── step_run.go         # run step implementation
├── step_build.go       # build step implementation
├── step_sequence.go    # sequence step implementation
├── step_parallel.go    # parallel step implementation
├── step_switch.go      # switch step implementation
├── step_workflow.go    # workflow step implementation
├── interpolate.go      # Template resolution
└── runtime.go          # ContainerRuntime interface
```
