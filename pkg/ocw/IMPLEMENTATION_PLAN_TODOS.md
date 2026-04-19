# OCW Runtime Implementation - TODOs and Notes

This document tracks open questions, deferred items, and implementation notes for the OCW runtime.

## Phase 1: Foundation - Open Items

### Watch Mode (Deferred)

The `schema.Watch` type exists in `pkg/schema/watch.go` but is not currently used in the runtime implementation plan. 

**Decision**: Deferred to a later phase. The basic runtime will execute steps without watch/reload capabilities initially.

**When to implement**: After Phase 4 (Runtime Core) is complete and stable. Watch mode requires:
- File system watching infrastructure
- Step restart logic with cleanup
- State management for background services during reload
- Integration with the iterator pattern for re-execution

**Related files**:
- `pkg/schema/watch.go` - Schema definition
- Future: `pkg/ocw/watch.go` or similar for watch orchestration

## Consolidation Notes

### StepError Location

**Decision**: Consolidated into `pkg/steps/errors.go` (single source of truth).

The original plan defined `StepError` in two places:
1. Referenced in `pkg/steps/composite/sequence/iterator.go` and `parallel/iterator.go`
2. Defined in `pkg/ocw/errors.go`

**Resolution**: Define once in `pkg/steps/errors.go` and use throughout the codebase.

## Testing Structure

**Unit tests**: `xxx_test.go` files in the same directory as the code they test.

**Integration tests**: `test/integration/` directory.

**E2E tests**: `test/e2e/` directory.

## Type References

The implementation references several schema types. Ensure these are available:

- `schema.RunStep` - `pkg/schema/run_step.go`
- `schema.BuildStep` - `pkg/schema/build_step.go`
- `schema.ParallelStep` - `pkg/schema/steps.go`
- `schema.SequenceStep` - `pkg/schema/steps.go`
- `schema.SwitchStep` - `pkg/schema/steps.go`
- `schema.WorkflowStep` - `pkg/schema/workflows.go`
- `schema.Step` - `pkg/schema/steps.go`
- `schema.HealthCheck` - `pkg/schema/run_step.go`
- `schema.VolumeRefs` - `pkg/schema/volumes.go`
- `schema.VolumeRef` - `pkg/schema/volumes.go`
- `schema.NumberOrString` - `pkg/schema/steps.go`

## Interface Assumptions

1. **Runtime interface**: All 20+ methods are implemented (full interface as specified in IMPLEMENTATION_PLAN.md)
2. **Step interfaces**: SimpleStep has `Execute()`, CompositeStep has `Iterator()`
3. **StepIterator**: Returns multiple steps for parallel execution
4. **Executor interface**: Provides access to container runtime, outputs, logger, and service registration

## Implementation Order

Following the plan in IMPLEMENTATION_PLAN.md:

1. ✅ Phase 1: Foundation (in progress)
   - pkg/container/ (types, options, runtime, errors)
   - pkg/steps/ (step, iterator, result, executor, scope, outputs, interpolate, errors)

2. Phase 2: Simple Steps (pending)
   - pkg/steps/simple/run/
   - pkg/steps/simple/build/

3. Phase 3: Composite Steps (pending)
   - pkg/steps/composite/sequence/
   - pkg/steps/composite/parallel/
   - pkg/steps/composite/switchstep/

4. Phase 4: Runtime Core (pending)
   - pkg/ocw/ (runtime, executor, job, context, outputs, services, errors)

5. Phase 5: Advanced (pending)
   - pkg/steps/composite/workflow/
   - Watch mode integration
