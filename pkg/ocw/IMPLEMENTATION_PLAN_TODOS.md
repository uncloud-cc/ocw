# OCW Runtime Implementation - TODOs and Notes

This document tracks open questions, deferred items, and implementation notes for the OCW runtime.

## Phase Status

### ✅ Phase 1: Foundation (COMPLETE)
**Date**: 2024-04-19
**Status**: All interfaces, types, and tests implemented. Ready for Phase 2.

**Created Files**:
- `pkg/container/types.go` - ID types, ExitResult, ContainerInfo, PortBinding, Streams + typed constants
- `pkg/container/options.go` - PullOptions, CreateOptions, BuildOptions, NetworkOptions, LogOptions, AttachOptions, Mount, PortMapping, HealthCheckConfig, BuildSecret
- `pkg/container/runtime.go` - Runtime interface (20 methods: Pull, Build, Create, Start, Stop, Remove, Wait, Inspect, Logs, Attach, Exec, CreateVolume, RemoveVolume, CreateNetwork, RemoveNetwork, ConnectNetwork)
- `pkg/container/errors.go` - Typed errors (ErrImageNotFound, ErrContainerNotFound, etc.) + ContainerError wrapper
- `pkg/container/mock/runtime.go` - Mock runtime with call tracking for testing
- `pkg/steps/step.go` - Step, SimpleStep, CompositeStep interfaces
- `pkg/steps/iterator.go` - StepIterator interface
- `pkg/steps/result.go` - Result type with Success(), SuccessWithOutputs(), Failed(), Merge() helpers
- `pkg/steps/executor.go` - Executor interface, Logger interface, ResolvedVolume
- `pkg/steps/step_context.go` - StepContext for template interpolation (renamed from Scope)
- `pkg/steps/interpolate.go` - Interpolate(), InterpolateMap(), InterpolateSlice() for {{ env.X }}, {{ secrets.X }}, {{ inputs.X }}, {{ steps.X.Y }}, {{ config.X }}
- `pkg/steps/outputs.go` - OutputStore (thread-safe)
- `pkg/steps/errors.go` - StepError (consolidated single source of truth)

**Test Coverage**:
- `pkg/container`: **100.0%** coverage
  - `types_test.go` - ID types, ExitResult, ContainerInfo, typed constants
  - `options_test.go` - All option structs
  - `errors_test.go` - Typed errors, ContainerError wrapping/unwrapping
  - `mock/runtime_test.go` - Mock implementation verification
- `pkg/steps`: **94.7%** coverage
  - `step_test.go` - Interface compliance tests (Step, SimpleStep, CompositeStep, StepIterator)
  - `result_test.go` - Success, Failed, Merge functions
  - `step_context_test.go` - StepContext creation, cloning, WithStepOutputs
  - `interpolate_test.go` - 27 tests covering all interpolation patterns and error cases
  - `outputs_test.go` - OutputStore Get, GetAll, Set, Snapshot, thread safety
  - `errors_test.go` - StepError variations, unwrapping
  - `executor_test.go` - ResolvedVolume, Logger mock, Executor interface compliance

### Phase 2: Simple Steps (PENDING)
**Status**: Ready to start

**To Implement**:
- `pkg/steps/simple/run/` - Run step implementation + parser + tests
- `pkg/steps/simple/build/` - Build step implementation + parser + tests

### Phase 3: Composite Steps (PENDING)
**To Implement**:
- `pkg/steps/composite/sequence/` - Sequence step + iterator + parser + tests
- `pkg/steps/composite/parallel/` - Parallel step + iterator + parser + tests
- `pkg/steps/composite/switchstep/` - Switch step + iterator + parser + tests

### Phase 4: Runtime Core (PENDING)
**To Implement**:
- `pkg/ocw/runtime.go` - Main Runtime struct and orchestration
- `pkg/ocw/executor.go` - ExecutionContext implementation
- `pkg/ocw/job.go` - Job execution and step parsing
- `pkg/ocw/services.go` - Background service tracking & health checks
- `pkg/ocw/errors.go` - Runtime error types

### Phase 5: Advanced (PENDING)
**To Implement**:
- `pkg/steps/composite/workflow/` - External workflow invocation
- Watch mode integration (deferred from Phase 1)

## Deferred Items

### Watch Mode
The `schema.Watch` type exists in `pkg/schema/watch.go` but is not currently used.

**Decision**: Deferred to Phase 5 after Runtime Core is stable.

**When to implement**: After Phase 4. Watch mode requires:
- File system watching infrastructure
- Step restart logic with cleanup
- State management for background services during reload
- Integration with the iterator pattern for re-execution

**Related files**:
- `pkg/schema/watch.go` - Schema definition
- Future: `pkg/ocw/watch.go` or similar for watch orchestration

## Design Decisions

### StepError Location
**Decision**: Consolidated into `pkg/steps/errors.go` (single source of truth).

Original plan defined `StepError` in two places:
1. Referenced in composite step iterators
2. Defined in runtime errors

**Resolution**: Define once in `pkg/steps/errors.go` and use throughout.

### Naming Conventions
- `StepContext` (renamed from `Scope`) to avoid confusion with Go's `context.Context`
- `step_context.go` filename instead of generic names like `scope.go` or `state.go`
- Deep nesting for steps: `pkg/steps/simple/run/`, `pkg/steps/composite/sequence/` (as requested)

### Testing Structure
- **Unit tests**: `xxx_test.go` files in same directory as code
- **Integration tests**: `test/integration/` directory (to be created as needed)
- **E2E tests**: `test/e2e/` directory (to be created as needed)

## Type References

The implementation references these schema types (already available):
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
