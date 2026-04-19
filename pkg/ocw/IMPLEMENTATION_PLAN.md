# OCW Runtime Implementation Plan

This document describes the architecture and implementation plan for the OCW runtime. The runtime is responsible for executing OCW workflows by orchestrating jobs, steps, and container operations.

## Architecture Overview

The runtime is structured into three distinct layers:

```
┌─────────────────────────────────────────────────────────────┐
│  CLI (external - injects container runtime)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: OCW Runtime (pkg/ocw/)                            │
│  - Orchestrates job execution                               │
│  - Drives step iteration                                    │
│  - Tracks background services                               │
│  - Handles cleanup                                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: Steps (pkg/steps/)                                │
│  - Simple steps: run, build (execute directly)              │
│  - Composite steps: sequence, parallel, switch (iterators)  │
│  - Parsers (schema → executable step with interpolation)    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Container Runtime (pkg/container/)                │
│  - Abstract interface for container operations              │
│  - Types: Container, Image, Volume, Network                 │
│  - Implementations provided externally (Docker, Podman)     │
└─────────────────────────────────────────────────────────────┘
```

## Step Types: Simple vs Composite

Steps are divided into two categories:

### Simple Steps (Leaf Nodes)
Simple steps do actual work by interacting with the container runtime. They implement an `Execute()` method that runs to completion.

- **`run`** - Execute a container
- **`build`** - Build an image

### Composite Steps (Control Flow)
Composite steps contain other steps and control how they execute. They expose an **iterator** that yields the next step(s) to run. The runtime drives the iteration.

- **`sequence`** - Run children in order
- **`parallel`** - Run children concurrently
- **`switch`** - Conditional branching
- **`workflow`** - External workflow invocation (future)

This separation allows the runtime to:
- Have full visibility into execution flow
- Execute steps uniformly (simple or composite)
- Support future features like pause/resume, step-by-step debugging
- Handle loops naturally without recursion

## Directory Structure

```
pkg/
├── container/                  # Layer 1: Container Runtime Interface
│   ├── runtime.go             # Runtime interface definition
│   ├── types.go               # Container, Image, Volume, Network types
│   ├── options.go             # CreateOptions, BuildOptions, PullOptions, etc.
│   └── errors.go              # Typed errors (ErrImageNotFound, ErrContainerFailed, etc.)
│
├── steps/                      # Layer 2: Step Definitions & Parsing
│   ├── step.go                # Step, SimpleStep, CompositeStep interfaces
│   ├── iterator.go            # StepIterator interface
│   ├── result.go              # Result type with outputs
│   ├── executor.go            # Executor interface (what simple steps receive)
│   ├── scope.go               # InterpolationScope for template resolution
│   ├── interpolate.go         # Template interpolation logic
│   │
│   ├── simple/                # Simple step implementations
│   │   ├── run/
│   │   │   ├── step.go        # RunStep implementation (SimpleStep)
│   │   │   └── parser.go      # Parses schema.RunStep → run.Step
│   │   └── build/
│   │       ├── step.go        # BuildStep implementation (SimpleStep)
│   │       └── parser.go      # Parses schema.BuildStep → build.Step
│   │
│   └── composite/             # Composite step implementations
│       ├── sequence/
│       │   ├── step.go        # SequenceStep (CompositeStep)
│       │   ├── iterator.go    # SequenceIterator
│       │   └── parser.go      # Parses schema.SequenceStep
│       ├── parallel/
│       │   ├── step.go        # ParallelStep (CompositeStep)
│       │   ├── iterator.go    # ParallelIterator
│       │   └── parser.go      # Parses schema.ParallelStep
│       ├── switchstep/        # "switch" is a reserved word
│       │   ├── step.go        # SwitchStep (CompositeStep)
│       │   ├── iterator.go    # SwitchIterator
│       │   └── parser.go      # Parses schema.SwitchStep
│       └── workflow/          # Future: external workflow invocation
│           ├── step.go
│           ├── iterator.go
│           └── parser.go
│
└── ocw/                        # Layer 3: OCW Runtime
    ├── runtime.go             # Main Runtime struct and Run method
    ├── executor.go            # Executor implementation
    ├── job.go                 # Job execution logic
    ├── context.go             # ExecutionContext (per-run shared state)
    ├── outputs.go             # OutputStore for {{ steps.x.y }} references
    ├── services.go            # Background service tracking & health checks
    └── errors.go              # Runtime error types
```

---

## Layer 1: Container Runtime Interface (`pkg/container/`)

This layer defines the abstract interface for container operations. The CLI will inject a concrete implementation (Docker, Podman, etc.).

### `pkg/container/types.go`

```go
package container

import "io"

// ContainerID is an opaque identifier for a container
type ContainerID string

// ImageID is an opaque identifier for an image  
type ImageID string

// VolumeID is an opaque identifier for a volume
type VolumeID string

// NetworkID is an opaque identifier for a network
type NetworkID string

// ExitResult contains the result of a container execution
type ExitResult struct {
    StatusCode int
    OOMKilled  bool
    Error      string
}

// ContainerInfo contains information about a running container
type ContainerInfo struct {
    ID      ContainerID
    Name    string
    Image   string
    Status  string // "created", "running", "paused", "exited", "dead"
    Health  string // "healthy", "unhealthy", "starting", "" (no healthcheck)
    Ports   []PortBinding
}

// PortBinding represents a port mapping
type PortBinding struct {
    ContainerPort int
    HostPort      int
    Protocol      string // "tcp", "udp"
}

// Streams represents attached container I/O streams
type Streams struct {
    Stdin  io.WriteCloser
    Stdout io.ReadCloser
    Stderr io.ReadCloser
}
```

### `pkg/container/options.go`

```go
package container

import "time"

// PullOptions configures image pulling
type PullOptions struct {
    Platform string // e.g., "linux/amd64"
    Quiet    bool
}

// CreateOptions configures container creation
type CreateOptions struct {
    Image       string
    Name        string            // Optional container name
    Cmd         []string          // Command to run
    Entrypoint  []string          // Override entrypoint
    Env         map[string]string // Environment variables
    WorkingDir  string
    Mounts      []Mount
    Ports       []PortMapping
    Network     NetworkID         // Network to attach to
    NetworkMode string            // "bridge", "host", "none", or network name
    
    // Resource limits
    CPUs        float64  // Number of CPUs
    Memory      int64    // Memory limit in bytes
    GPUs        string   // GPU configuration ("all" or count)
    
    // Health check
    HealthCheck *HealthCheckConfig
    
    // TTY/Interactive
    TTY         bool
    OpenStdin   bool
    
    // Labels for identification
    Labels      map[string]string
}

// Mount represents a volume/bind mount
type Mount struct {
    Type        string // "bind", "volume"
    Source      string // Host path or volume name
    Target      string // Container path
    ReadOnly    bool
}

// PortMapping represents a port exposure
type PortMapping struct {
    ContainerPort int
    HostPort      int    // 0 = auto-assign
    Protocol      string // "tcp", "udp" (default: "tcp")
}

// HealthCheckConfig configures container health checking
type HealthCheckConfig struct {
    Test        []string      // Command to run
    Interval    time.Duration
    Timeout     time.Duration
    Retries     int
    StartPeriod time.Duration
}

// BuildOptions configures image building
type BuildOptions struct {
    ContextPath    string
    Dockerfile     string            // Path relative to context
    Tags           []string          // Image tags
    BuildArgs      map[string]string
    Target         string            // Multi-stage target
    Platform       []string          // Target platforms
    CacheFrom      []string
    CacheTo        []string
    NoCache        bool
    Pull           bool              // Always pull base images
    Push           bool              // Push after build
    Load           bool              // Load into local images
    Labels         map[string]string
    Secrets        []BuildSecret
    
    // Progress reporting
    ProgressWriter io.Writer
}

// BuildSecret represents a secret available during build
type BuildSecret struct {
    ID     string
    Source string // File path or env var name
    IsEnv  bool   // True if Source is an env var name
}

// NetworkOptions configures network creation
type NetworkOptions struct {
    Driver   string            // "bridge", "overlay", etc.
    Labels   map[string]string
    Internal bool              // No external connectivity
}

// LogOptions configures log retrieval
type LogOptions struct {
    Follow     bool
    Timestamps bool
    Since      time.Time
    Until      time.Time
    Tail       string // "all" or number
}

// AttachOptions configures container attachment
type AttachOptions struct {
    Stdin  bool
    Stdout bool
    Stderr bool
    Stream bool
}
```

### `pkg/container/runtime.go`

```go
package container

import (
    "context"
    "io"
    "time"
)

// Runtime is the interface for container operations.
// Implementations are provided externally (Docker, Podman, etc.).
type Runtime interface {
    // Image operations
    Pull(ctx context.Context, image string, opts PullOptions) error
    Build(ctx context.Context, opts BuildOptions) (ImageID, error)
    ImageExists(ctx context.Context, image string) (bool, error)
    
    // Container lifecycle
    Create(ctx context.Context, opts CreateOptions) (ContainerID, error)
    Start(ctx context.Context, id ContainerID) error
    Stop(ctx context.Context, id ContainerID, timeout time.Duration) error
    Remove(ctx context.Context, id ContainerID, force bool) error
    Kill(ctx context.Context, id ContainerID, signal string) error
    
    // Container inspection
    Wait(ctx context.Context, id ContainerID) (ExitResult, error)
    Inspect(ctx context.Context, id ContainerID) (ContainerInfo, error)
    
    // Logs and I/O
    Logs(ctx context.Context, id ContainerID, opts LogOptions) (io.ReadCloser, error)
    Attach(ctx context.Context, id ContainerID, opts AttachOptions) (Streams, error)
    
    // Exec (for health checks)
    Exec(ctx context.Context, id ContainerID, cmd []string) (ExitResult, error)
    
    // Volumes
    CreateVolume(ctx context.Context, name string) (VolumeID, error)
    RemoveVolume(ctx context.Context, id VolumeID) error
    
    // Networks
    CreateNetwork(ctx context.Context, name string, opts NetworkOptions) (NetworkID, error)
    RemoveNetwork(ctx context.Context, id NetworkID) error
    ConnectNetwork(ctx context.Context, networkID NetworkID, containerID ContainerID) error
}
```

### `pkg/container/errors.go`

```go
package container

import (
    "errors"
    "fmt"
)

var (
    // ErrImageNotFound is returned when an image cannot be found
    ErrImageNotFound = errors.New("image not found")
    
    // ErrContainerNotFound is returned when a container cannot be found
    ErrContainerNotFound = errors.New("container not found")
    
    // ErrContainerNotRunning is returned when trying to stop a non-running container
    ErrContainerNotRunning = errors.New("container is not running")
    
    // ErrBuildFailed is returned when an image build fails
    ErrBuildFailed = errors.New("build failed")
    
    // ErrNetworkNotFound is returned when a network cannot be found
    ErrNetworkNotFound = errors.New("network not found")
    
    // ErrVolumeNotFound is returned when a volume cannot be found
    ErrVolumeNotFound = errors.New("volume not found")
)

// ContainerError wraps an error with container context
type ContainerError struct {
    ContainerID ContainerID
    Operation   string // "create", "start", "stop", etc.
    Err         error
}

func (e *ContainerError) Error() string {
    return fmt.Sprintf("%s container %s: %v", e.Operation, e.ContainerID, e.Err)
}

func (e *ContainerError) Unwrap() error {
    return e.Err
}
```

---

## Layer 2: Steps (`pkg/steps/`)

This layer defines the step interfaces and provides implementations. There are two kinds of steps:

1. **Simple steps** implement `Execute()` and do actual work
2. **Composite steps** implement `Iterator()` and yield child steps

### `pkg/steps/step.go`

```go
package steps

import "context"

// Step is the base interface for all steps.
// Use type assertions to determine if a step is Simple or Composite.
type Step interface {
    // ID returns the step's identifier (for output references).
    // Returns empty string if the step has no ID.
    ID() string
    
    // Name returns the step's display name.
    Name() string
}

// SimpleStep executes directly against the container runtime.
// These are leaf nodes that do actual work (run containers, build images).
type SimpleStep interface {
    Step
    
    // Execute runs the step and returns its result.
    // The Executor provides access to the container runtime and shared state.
    Execute(ctx context.Context, exec Executor) (*Result, error)
}

// CompositeStep contains other steps and controls their execution flow.
// These are control flow nodes (sequence, parallel, switch).
type CompositeStep interface {
    Step
    
    // Children returns all child steps (for validation, visualization, etc.)
    Children() []Step
    
    // Iterator returns a fresh iterator for executing this composite step.
    // The scope provides the interpolation context for child steps.
    Iterator(scope *Scope) StepIterator
}
```

### `pkg/steps/iterator.go`

```go
package steps

// StepIterator yields steps to execute from a composite step.
// The runtime drives iteration by calling Next() repeatedly.
type StepIterator interface {
    // Next returns the next step(s) to execute.
    //
    // Parameters:
    //   - lastResults: Results from the previous Next() call's steps.
    //                  Nil on first call.
    //
    // Returns:
    //   - steps: The next step(s) to execute. Multiple steps means parallel execution.
    //   - done: True if iteration is complete (steps will be empty).
    //   - err: Error if iteration cannot continue.
    //
    // The runtime calls Next() in a loop:
    //   1. Call Next(nil) to get first step(s)
    //   2. Execute returned steps
    //   3. Call Next(results) with execution results
    //   4. Repeat until done=true or err!=nil
    Next(lastResults []*Result) (steps []Step, done bool, err error)
    
    // Result returns the final combined result after iteration completes.
    // Only valid to call after Next() returns done=true.
    Result() *Result
}
```

### `pkg/steps/result.go`

```go
package steps

// Result contains the outcome of a step execution
type Result struct {
    // Outputs are key-value pairs that can be referenced by subsequent steps
    // via {{ steps.<id>.<key> }} syntax
    Outputs map[string]string
    
    // ExitCode is the container's exit code (for run steps)
    // 0 indicates success, non-zero indicates failure
    ExitCode int
    
    // ContainerID is set for background steps, allowing the runtime
    // to track and clean up the container
    ContainerID string
    
    // IsBackground indicates this step started a background service
    IsBackground bool
}

// Success creates a successful result with no outputs
func Success() *Result {
    return &Result{ExitCode: 0, Outputs: make(map[string]string)}
}

// SuccessWithOutputs creates a successful result with outputs
func SuccessWithOutputs(outputs map[string]string) *Result {
    return &Result{ExitCode: 0, Outputs: outputs}
}

// Failed creates a failed result with the given exit code
func Failed(exitCode int) *Result {
    return &Result{ExitCode: exitCode, Outputs: make(map[string]string)}
}

// Merge combines multiple results into one (for parallel steps)
func Merge(results []*Result) *Result {
    merged := &Result{
        Outputs:  make(map[string]string),
        ExitCode: 0,
    }
    for _, r := range results {
        if r == nil {
            continue
        }
        for k, v := range r.Outputs {
            merged.Outputs[k] = v
        }
        if r.ExitCode != 0 {
            merged.ExitCode = r.ExitCode
        }
    }
    return merged
}
```

### `pkg/steps/executor.go`

```go
package steps

import (
    "context"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
)

// Executor provides the execution environment for simple steps.
// It is passed to SimpleStep.Execute() and provides access to:
// - The container runtime
// - Output storage for step communication  
// - Logging
// - Service registration for background containers
type Executor interface {
    // Container returns the container runtime
    Container() container.Runtime
    
    // Outputs returns the output store for reading/writing step outputs
    Outputs() *OutputStore
    
    // Logger returns the logger for this execution
    Logger() Logger
    
    // WorkDir returns the workflow's working directory (where the OCW file is)
    WorkDir() string
    
    // ResolvedVolumes returns the resolved volume definitions
    // Key is volume name, value is the resolved host path and mount settings
    ResolvedVolumes() map[string]ResolvedVolume
    
    // RegisterService registers a background container as a service.
    // The runtime will track its health and clean it up when the job completes.
    RegisterService(id string, containerID container.ContainerID, healthCheck *schema.HealthCheck)
    
    // WaitForServices waits for the specified service IDs to become healthy.
    // Used to implement the "needs" field on steps.
    WaitForServices(ctx context.Context, serviceIDs []string) error
}

// ResolvedVolume contains a fully resolved volume ready for mounting
type ResolvedVolume struct {
    HostPath  string
    MountPath string // Default mount path (can be overridden per-step)
    ReadOnly  bool
}

// Logger provides logging capabilities for steps
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    
    // WithStep returns a logger scoped to a specific step
    WithStep(stepID, stepName string) Logger
}
```

### `pkg/steps/outputs.go`

```go
package steps

import "sync"

// OutputStore provides thread-safe storage for step outputs.
// Outputs are stored as map[stepID]map[key]value.
type OutputStore struct {
    mu      sync.RWMutex
    outputs map[string]map[string]string
}

// NewOutputStore creates a new output store
func NewOutputStore() *OutputStore {
    return &OutputStore{
        outputs: make(map[string]map[string]string),
    }
}

// Get retrieves an output value: store.Get("stepID", "key")
func (s *OutputStore) Get(stepID, key string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    stepOutputs, ok := s.outputs[stepID]
    if !ok {
        return "", false
    }
    
    value, ok := stepOutputs[key]
    return value, ok
}

// GetAll retrieves all outputs for a step
func (s *OutputStore) GetAll(stepID string) (map[string]string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    stepOutputs, ok := s.outputs[stepID]
    if !ok {
        return nil, false
    }
    
    // Return a copy
    result := make(map[string]string, len(stepOutputs))
    for k, v := range stepOutputs {
        result[k] = v
    }
    return result, true
}

// Set stores outputs for a step
func (s *OutputStore) Set(stepID string, outputs map[string]string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.outputs[stepID] = outputs
}

// Snapshot returns a copy of all outputs (for building scope)
func (s *OutputStore) Snapshot() map[string]map[string]string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    result := make(map[string]map[string]string, len(s.outputs))
    for stepID, outputs := range s.outputs {
        stepCopy := make(map[string]string, len(outputs))
        for k, v := range outputs {
            stepCopy[k] = v
        }
        result[stepID] = stepCopy
    }
    return result
}
```

### `pkg/steps/scope.go`

```go
package steps

// Scope contains the data available for template interpolation.
// It is built up as steps execute and passed to parsers.
type Scope struct {
    // Env contains environment variables (workflow + job + step level merged)
    Env map[string]string
    
    // Secrets contains resolved secret values
    Secrets map[string]string
    
    // Inputs contains workflow inputs (from CLI or parent workflow)
    Inputs map[string]string
    
    // Steps contains outputs from previous steps, keyed by step ID
    // Access pattern: Steps["stepID"]["outputKey"]
    Steps map[string]map[string]string
    
    // Config contains workflow configuration values
    Config map[string]any
}

// NewScope creates a new empty scope
func NewScope() *Scope {
    return &Scope{
        Env:     make(map[string]string),
        Secrets: make(map[string]string),
        Inputs:  make(map[string]string),
        Steps:   make(map[string]map[string]string),
        Config:  make(map[string]any),
    }
}

// Clone creates a deep copy of the scope
func (s *Scope) Clone() *Scope {
    clone := NewScope()
    
    for k, v := range s.Env {
        clone.Env[k] = v
    }
    for k, v := range s.Secrets {
        clone.Secrets[k] = v
    }
    for k, v := range s.Inputs {
        clone.Inputs[k] = v
    }
    for stepID, outputs := range s.Steps {
        clone.Steps[stepID] = make(map[string]string, len(outputs))
        for k, v := range outputs {
            clone.Steps[stepID][k] = v
        }
    }
    for k, v := range s.Config {
        clone.Config[k] = v
    }
    
    return clone
}

// WithStepOutputs returns a new scope with the given step's outputs added
func (s *Scope) WithStepOutputs(stepID string, outputs map[string]string) *Scope {
    clone := s.Clone()
    clone.Steps[stepID] = make(map[string]string, len(outputs))
    for k, v := range outputs {
        clone.Steps[stepID][k] = v
    }
    return clone
}
```

### `pkg/steps/interpolate.go`

```go
package steps

import (
    "fmt"
    "regexp"
    "strings"
)

// templatePattern matches {{ expression }} patterns
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate resolves template expressions in a string.
// Supported syntax:
//   - {{ env.VAR_NAME }}        - Environment variable
//   - {{ secrets.SECRET_NAME }} - Secret value
//   - {{ inputs.INPUT_NAME }}   - Workflow input
//   - {{ steps.STEP_ID.KEY }}   - Output from a previous step
//   - {{ config.ns.key }}       - Configuration value
//
// Returns the interpolated string and any error encountered.
func Interpolate(template string, scope *Scope) (string, error) {
    var firstErr error
    
    result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
        if firstErr != nil {
            return match
        }
        
        // Extract expression from {{ expr }}
        submatch := templatePattern.FindStringSubmatch(match)
        if len(submatch) < 2 {
            return match
        }
        expr := strings.TrimSpace(submatch[1])
        
        value, err := resolveExpression(expr, scope)
        if err != nil {
            firstErr = &InterpolationError{
                Template:   template,
                Expression: expr,
                Reason:     err.Error(),
            }
            return match
        }
        
        return value
    })
    
    return result, firstErr
}

func resolveExpression(expr string, scope *Scope) (string, error) {
    parts := strings.Split(expr, ".")
    if len(parts) < 2 {
        return "", fmt.Errorf("invalid expression: %s", expr)
    }
    
    switch parts[0] {
    case "env":
        if len(parts) != 2 {
            return "", fmt.Errorf("env requires exactly one key: env.VAR_NAME")
        }
        if val, ok := scope.Env[parts[1]]; ok {
            return val, nil
        }
        return "", fmt.Errorf("env variable %q not found", parts[1])
        
    case "secrets":
        if len(parts) != 2 {
            return "", fmt.Errorf("secrets requires exactly one key: secrets.NAME")
        }
        if val, ok := scope.Secrets[parts[1]]; ok {
            return val, nil
        }
        return "", fmt.Errorf("secret %q not found", parts[1])
        
    case "inputs":
        if len(parts) != 2 {
            return "", fmt.Errorf("inputs requires exactly one key: inputs.NAME")
        }
        if val, ok := scope.Inputs[parts[1]]; ok {
            return val, nil
        }
        return "", fmt.Errorf("input %q not found", parts[1])
        
    case "steps":
        if len(parts) != 3 {
            return "", fmt.Errorf("steps requires step ID and key: steps.STEP_ID.KEY")
        }
        stepID, key := parts[1], parts[2]
        if outputs, ok := scope.Steps[stepID]; ok {
            if val, ok := outputs[key]; ok {
                return val, nil
            }
            return "", fmt.Errorf("step %q has no output %q", stepID, key)
        }
        return "", fmt.Errorf("step %q not found", stepID)
        
    case "config":
        // Navigate nested config: config.namespace.key
        var current any = scope.Config
        for i := 1; i < len(parts); i++ {
            if m, ok := current.(map[string]any); ok {
                current = m[parts[i]]
            } else {
                return "", fmt.Errorf("config path %q not found", strings.Join(parts[:i+1], "."))
            }
        }
        if current == nil {
            return "", fmt.Errorf("config %q not found", expr)
        }
        return fmt.Sprintf("%v", current), nil
        
    default:
        return "", fmt.Errorf("unknown reference type: %s", parts[0])
    }
}

// InterpolateMap interpolates all values in a map
func InterpolateMap(m map[string]string, scope *Scope) (map[string]string, error) {
    result := make(map[string]string, len(m))
    for k, v := range m {
        interpolated, err := Interpolate(v, scope)
        if err != nil {
            return nil, fmt.Errorf("interpolating %q: %w", k, err)
        }
        result[k] = interpolated
    }
    return result, nil
}

// InterpolateSlice interpolates all values in a slice
func InterpolateSlice(s []string, scope *Scope) ([]string, error) {
    result := make([]string, len(s))
    for i, v := range s {
        interpolated, err := Interpolate(v, scope)
        if err != nil {
            return nil, fmt.Errorf("interpolating index %d: %w", i, err)
        }
        result[i] = interpolated
    }
    return result, nil
}

// InterpolationError represents a template interpolation failure
type InterpolationError struct {
    Template   string
    Expression string
    Reason     string
}

func (e *InterpolationError) Error() string {
    return fmt.Sprintf("interpolation failed for %q: %s (in template: %s)", e.Expression, e.Reason, e.Template)
}
```

---

## Simple Step: Run (`pkg/steps/simple/run/`)

### `pkg/steps/simple/run/step.go`

```go
package run

import (
    "context"
    "fmt"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes a container. Implements SimpleStep.
type Step struct {
    id          string
    name        string
    
    // Container config (all values already interpolated)
    image       string
    cmd         []string
    entrypoint  []string
    env         map[string]string
    workdir     string
    mounts      []container.Mount
    ports       []container.PortMapping
    
    // Resource limits
    cpus        float64
    memory      int64
    gpus        string
    platform    string
    
    // Execution mode
    background  bool
    healthCheck *container.HealthCheckConfig
    
    // Pull policy
    pullPolicy  string // "always", "missing", "never"
    
    // Output control
    quiet       bool
    tty         bool
    
    // Dependencies
    needs       []string
}

func (s *Step) ID() string   { return s.id }
func (s *Step) Name() string { return s.name }

func (s *Step) Execute(ctx context.Context, exec steps.Executor) (*steps.Result, error) {
    logger := exec.Logger().WithStep(s.id, s.name)
    
    // 1. Wait for dependencies (needs)
    if len(s.needs) > 0 {
        logger.Info("waiting for services", "needs", s.needs)
        if err := exec.WaitForServices(ctx, s.needs); err != nil {
            return nil, fmt.Errorf("waiting for services: %w", err)
        }
    }
    
    // 2. Pull image if needed
    if err := s.ensureImage(ctx, exec); err != nil {
        return nil, err
    }
    
    // 3. Create container
    logger.Info("creating container", "image", s.image)
    containerID, err := exec.Container().Create(ctx, s.createOptions())
    if err != nil {
        return nil, fmt.Errorf("creating container: %w", err)
    }
    
    // 4. Start container
    logger.Info("starting container", "id", containerID)
    if err := exec.Container().Start(ctx, containerID); err != nil {
        return nil, fmt.Errorf("starting container: %w", err)
    }
    
    // 5. Handle background vs foreground
    if s.background {
        logger.Info("started background service", "id", containerID)
        exec.RegisterService(s.id, containerID, nil) // healthCheck converted separately
        
        return &steps.Result{
            ContainerID:  string(containerID),
            IsBackground: true,
            Outputs:      s.buildServiceOutputs(containerID, exec),
        }, nil
    }
    
    // 6. Wait for completion
    logger.Info("waiting for container to complete")
    result, err := exec.Container().Wait(ctx, containerID)
    if err != nil {
        return nil, fmt.Errorf("waiting for container: %w", err)
    }
    
    // 7. Cleanup
    if err := exec.Container().Remove(ctx, containerID, false); err != nil {
        logger.Warn("failed to remove container", "id", containerID, "error", err)
    }
    
    // 8. Return result
    if result.StatusCode != 0 {
        logger.Info("container exited with error", "exitCode", result.StatusCode)
        return steps.Failed(result.StatusCode), nil
    }
    
    logger.Info("container completed successfully")
    return steps.Success(), nil
}

func (s *Step) ensureImage(ctx context.Context, exec steps.Executor) error {
    switch s.pullPolicy {
    case "always":
        return exec.Container().Pull(ctx, s.image, container.PullOptions{
            Platform: s.platform,
            Quiet:    s.quiet,
        })
    case "never":
        return nil
    default: // "missing" or empty
        exists, err := exec.Container().ImageExists(ctx, s.image)
        if err != nil {
            return fmt.Errorf("checking image: %w", err)
        }
        if !exists {
            return exec.Container().Pull(ctx, s.image, container.PullOptions{
                Platform: s.platform,
                Quiet:    s.quiet,
            })
        }
        return nil
    }
}

func (s *Step) createOptions() container.CreateOptions {
    return container.CreateOptions{
        Image:       s.image,
        Cmd:         s.cmd,
        Entrypoint:  s.entrypoint,
        Env:         s.env,
        WorkingDir:  s.workdir,
        Mounts:      s.mounts,
        Ports:       s.ports,
        CPUs:        s.cpus,
        Memory:      s.memory,
        GPUs:        s.gpus,
        HealthCheck: s.healthCheck,
        TTY:         s.tty,
    }
}

func (s *Step) buildServiceOutputs(id container.ContainerID, exec steps.Executor) map[string]string {
    // For background services, output connection info
    // This can be expanded based on exposed ports
    outputs := map[string]string{
        "container_id": string(id),
    }
    
    // If ports are exposed, add host/port info
    if len(s.ports) > 0 {
        outputs["host"] = "localhost" // Or container name for internal networking
        outputs["port"] = fmt.Sprintf("%d", s.ports[0].HostPort)
    }
    
    return outputs
}
```

### `pkg/steps/simple/run/parser.go`

```go
package run

import (
    "fmt"
    "strconv"
    "strings"
    "time"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.RunStep into an executable run.Step.
// This is where template interpolation happens.
func Parse(rs *schema.RunStep, scope *steps.Scope, volumes map[string]steps.ResolvedVolume) (*Step, error) {
    // Interpolate image
    image, err := steps.Interpolate(rs.Image, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating image: %w", err)
    }
    
    // Interpolate command
    var cmd []string
    if rs.Cmd != "" {
        cmdStr, err := steps.Interpolate(rs.Cmd, scope)
        if err != nil {
            return nil, fmt.Errorf("interpolating cmd: %w", err)
        }
        cmd = parseCommand(cmdStr)
    }
    
    // Interpolate args
    args, err := steps.InterpolateSlice(rs.Args, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating args: %w", err)
    }
    if len(args) > 0 {
        cmd = append(cmd, args...)
    }
    
    // Interpolate entrypoint
    var entrypoint []string
    if rs.Entrypoint != "" {
        epStr, err := steps.Interpolate(rs.Entrypoint, scope)
        if err != nil {
            return nil, fmt.Errorf("interpolating entrypoint: %w", err)
        }
        entrypoint = parseCommand(epStr)
    }
    
    // Interpolate workdir
    workdir, err := steps.Interpolate(rs.Workdir, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating workdir: %w", err)
    }
    
    // Parse environment variables
    env, err := parseEnv(rs, scope)
    if err != nil {
        return nil, fmt.Errorf("parsing env: %w", err)
    }
    
    // Resolve volume mounts
    mounts, err := resolveMounts(rs.Volumes, volumes)
    if err != nil {
        return nil, fmt.Errorf("resolving mounts: %w", err)
    }
    
    // Parse port exposures
    ports := parsePorts(rs.Expose)
    
    // Parse health check
    var healthCheck *container.HealthCheckConfig
    if rs.HealthCheck != nil {
        healthCheck, err = parseHealthCheck(rs.HealthCheck, scope)
        if err != nil {
            return nil, fmt.Errorf("parsing health check: %w", err)
        }
    }
    
    return &Step{
        id:          string(rs.ID),
        name:        string(rs.Name),
        image:       image,
        cmd:         cmd,
        entrypoint:  entrypoint,
        env:         env,
        workdir:     workdir,
        mounts:      mounts,
        ports:       ports,
        cpus:        parseCPUs(rs.CPUs),
        memory:      parseMemory(rs.Memory),
        gpus:        parseGPUs(rs.GPUs),
        platform:    rs.Platform,
        background:  rs.Background,
        healthCheck: healthCheck,
        pullPolicy:  string(rs.Pull),
        quiet:       rs.Quiet,
        tty:         rs.TTY,
        needs:       rs.Needs,
    }, nil
}

// parseCommand splits a shell-style command into arguments
func parseCommand(cmd string) []string {
    if cmd == "" {
        return nil
    }
    // Simple split - a full implementation would handle quoting
    return strings.Fields(cmd)
}

// parseEnv combines step-level env with schema env
func parseEnv(rs *schema.RunStep, scope *steps.Scope) (map[string]string, error) {
    env := make(map[string]string)
    
    // Start with scope env
    for k, v := range scope.Env {
        env[k] = v
    }
    
    // Add step-level env (from RunEnv field)
    if rs.RunEnv != nil {
        if rs.RunEnv.Map != nil {
            for k, v := range rs.RunEnv.Map {
                interpolated, err := steps.Interpolate(v, scope)
                if err != nil {
                    return nil, fmt.Errorf("interpolating env %q: %w", k, err)
                }
                env[k] = interpolated
            }
        }
        if rs.RunEnv.Slice != nil {
            for _, item := range rs.RunEnv.Slice {
                parts := strings.SplitN(item, "=", 2)
                if len(parts) == 2 {
                    interpolated, err := steps.Interpolate(parts[1], scope)
                    if err != nil {
                        return nil, fmt.Errorf("interpolating env %q: %w", parts[0], err)
                    }
                    env[parts[0]] = interpolated
                }
            }
        }
    }
    
    return env, nil
}

// resolveMounts converts volume references to container mounts
func resolveMounts(refs schema.VolumeRefs, volumes map[string]steps.ResolvedVolume) ([]container.Mount, error) {
    var mounts []container.Mount
    
    for _, ref := range refs {
        vol, ok := volumes[ref.Name]
        if !ok {
            return nil, fmt.Errorf("volume %q not found", ref.Name)
        }
        
        mountPath := vol.MountPath
        if ref.MountPath != "" {
            mountPath = ref.MountPath
        }
        
        readOnly := vol.ReadOnly
        if ref.ReadOnly != nil {
            readOnly = *ref.ReadOnly
        }
        
        mounts = append(mounts, container.Mount{
            Type:     "bind",
            Source:   vol.HostPath,
            Target:   mountPath,
            ReadOnly: readOnly,
        })
    }
    
    return mounts, nil
}

// parsePorts converts expose config to port mappings
func parsePorts(expose *schema.Expose) []container.PortMapping {
    if expose == nil || len(expose.Ports) == 0 {
        return nil
    }
    
    ports := make([]container.PortMapping, len(expose.Ports))
    for i, p := range expose.Ports {
        protocol := p.Protocol
        if protocol == "" || protocol == "http" || protocol == "https" {
            protocol = "tcp"
        }
        ports[i] = container.PortMapping{
            ContainerPort: p.ContainerPort,
            HostPort:      p.HostPort,
            Protocol:      protocol,
        }
    }
    return ports
}

// parseHealthCheck converts schema health check to container config
func parseHealthCheck(hc *schema.HealthCheck, scope *steps.Scope) (*container.HealthCheckConfig, error) {
    cmd, err := steps.Interpolate(hc.Cmd, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating health check cmd: %w", err)
    }
    
    return &container.HealthCheckConfig{
        Test:        []string{"CMD-SHELL", cmd},
        Interval:    parseDuration(hc.Interval, 30*time.Second),
        Timeout:     parseDuration(hc.Timeout, 30*time.Second),
        Retries:     hc.Retries,
        StartPeriod: parseDuration(hc.StartPeriod, 0),
    }, nil
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
    if s == "" {
        return defaultVal
    }
    d, err := time.ParseDuration(s)
    if err != nil {
        return defaultVal
    }
    return d
}

func parseCPUs(n *schema.NumberOrString) float64 {
    if n == nil {
        return 0
    }
    if n.Number != nil {
        return *n.Number
    }
    if n.String != nil {
        f, _ := strconv.ParseFloat(*n.String, 64)
        return f
    }
    return 0
}

func parseMemory(mem string) int64 {
    if mem == "" {
        return 0
    }
    // Parse values like "512m", "2g", "1024k"
    mem = strings.ToLower(strings.TrimSpace(mem))
    
    multiplier := int64(1)
    if strings.HasSuffix(mem, "k") {
        multiplier = 1024
        mem = mem[:len(mem)-1]
    } else if strings.HasSuffix(mem, "m") {
        multiplier = 1024 * 1024
        mem = mem[:len(mem)-1]
    } else if strings.HasSuffix(mem, "g") {
        multiplier = 1024 * 1024 * 1024
        mem = mem[:len(mem)-1]
    }
    
    val, _ := strconv.ParseInt(mem, 10, 64)
    return val * multiplier
}

func parseGPUs(n *schema.NumberOrString) string {
    if n == nil {
        return ""
    }
    if n.Number != nil {
        return fmt.Sprintf("%d", int(*n.Number))
    }
    if n.String != nil {
        return *n.String
    }
    return ""
}
```

---

## Simple Step: Build (`pkg/steps/simple/build/`)

Similar structure to run. Implement `step.go` with an `Execute()` method that calls `exec.Container().Build()`, and `parser.go` that converts `schema.BuildStep` to the executable step.

---

## Composite Step: Sequence (`pkg/steps/composite/sequence/`)

### `pkg/steps/composite/sequence/step.go`

```go
package sequence

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes child steps sequentially. Implements CompositeStep.
type Step struct {
    id       string
    name     string
    children []steps.Step
}

// New creates a new sequence step
func New(id, name string, children []steps.Step) *Step {
    return &Step{
        id:       id,
        name:     name,
        children: children,
    }
}

func (s *Step) ID() string           { return s.id }
func (s *Step) Name() string         { return s.name }
func (s *Step) Children() []steps.Step { return s.children }

func (s *Step) Iterator(scope *steps.Scope) steps.StepIterator {
    return &Iterator{
        steps:   s.children,
        scope:   scope,
        index:   0,
        outputs: make(map[string]string),
    }
}
```

### `pkg/steps/composite/sequence/iterator.go`

```go
package sequence

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator yields steps one at a time for sequential execution.
type Iterator struct {
    steps   []steps.Step
    scope   *steps.Scope
    index   int
    outputs map[string]string
    done    bool
}

func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
    // Process result from previous step
    if lastResults != nil && len(lastResults) > 0 {
        last := lastResults[0]
        
        // Check for failure
        if last.ExitCode != 0 {
            it.done = true
            return nil, true, &steps.StepError{
                ExitCode: last.ExitCode,
            }
        }
        
        // Store outputs from previous step
        prevStep := it.steps[it.index-1]
        if prevStep.ID() != "" {
            it.scope = it.scope.WithStepOutputs(prevStep.ID(), last.Outputs)
            for k, v := range last.Outputs {
                it.outputs[k] = v
            }
        }
    }
    
    // Check if we're done
    if it.index >= len(it.steps) {
        it.done = true
        return nil, true, nil
    }
    
    // Return next step
    step := it.steps[it.index]
    it.index++
    
    return []steps.Step{step}, false, nil
}

func (it *Iterator) Result() *steps.Result {
    return steps.SuccessWithOutputs(it.outputs)
}

// Scope returns the current scope (with accumulated outputs)
// This is used by the runtime to pass the updated scope to child step parsers
func (it *Iterator) Scope() *steps.Scope {
    return it.scope
}
```

### `pkg/steps/composite/sequence/parser.go`

```go
package sequence

import (
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.SequenceStep into an executable sequence.Step.
// Note: Child steps are NOT parsed here - they are parsed lazily by the runtime
// as the iterator yields them, allowing proper scope accumulation.
func Parse(ss *schema.SequenceStep, childSteps []steps.Step) (*Step, error) {
    return New(
        string(ss.ID),
        string(ss.Name),
        childSteps,
    ), nil
}
```

---

## Composite Step: Parallel (`pkg/steps/composite/parallel/`)

### `pkg/steps/composite/parallel/step.go`

```go
package parallel

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes child steps concurrently. Implements CompositeStep.
type Step struct {
    id       string
    name     string
    children []steps.Step
}

// New creates a new parallel step
func New(id, name string, children []steps.Step) *Step {
    return &Step{
        id:       id,
        name:     name,
        children: children,
    }
}

func (s *Step) ID() string           { return s.id }
func (s *Step) Name() string         { return s.name }
func (s *Step) Children() []steps.Step { return s.children }

func (s *Step) Iterator(scope *steps.Scope) steps.StepIterator {
    return &Iterator{
        steps:   s.children,
        scope:   scope,
        started: false,
    }
}
```

### `pkg/steps/composite/parallel/iterator.go`

```go
package parallel

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator yields all steps at once for parallel execution.
type Iterator struct {
    steps   []steps.Step
    scope   *steps.Scope
    started bool
    results []*steps.Result
}

func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
    if !it.started {
        // First call - return all steps for parallel execution
        it.started = true
        return it.steps, false, nil
    }
    
    // Second call - all parallel steps completed
    it.results = lastResults
    
    // Check for any failures
    for i, r := range lastResults {
        if r != nil && r.ExitCode != 0 {
            return nil, true, &steps.StepError{
                StepID:   it.steps[i].ID(),
                StepName: it.steps[i].Name(),
                ExitCode: r.ExitCode,
            }
        }
    }
    
    return nil, true, nil
}

func (it *Iterator) Result() *steps.Result {
    return steps.Merge(it.results)
}

func (it *Iterator) Scope() *steps.Scope {
    return it.scope
}
```

---

## Composite Step: Switch (`pkg/steps/composite/switchstep/`)

### `pkg/steps/composite/switchstep/step.go`

```go
package switchstep

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Branch represents a case branch
type Branch struct {
    Value string       // The case value to match
    Steps []steps.Step // Steps to execute if matched
}

// Step conditionally executes a branch based on a value. Implements CompositeStep.
type Step struct {
    id           string
    name         string
    value        string    // The resolved switch value
    branches     []Branch  // Case branches
    defaultSteps []steps.Step // Default branch (may be nil)
}

// New creates a new switch step
func New(id, name, value string, branches []Branch, defaultSteps []steps.Step) *Step {
    return &Step{
        id:           id,
        name:         name,
        value:        value,
        branches:     branches,
        defaultSteps: defaultSteps,
    }
}

func (s *Step) ID() string   { return s.id }
func (s *Step) Name() string { return s.name }

func (s *Step) Children() []steps.Step {
    // Return all possible children for validation/visualization
    var all []steps.Step
    for _, b := range s.branches {
        all = append(all, b.Steps...)
    }
    all = append(all, s.defaultSteps...)
    return all
}

func (s *Step) Iterator(scope *steps.Scope) steps.StepIterator {
    // Find matching branch
    var selectedSteps []steps.Step
    for _, branch := range s.branches {
        if branch.Value == s.value {
            selectedSteps = branch.Steps
            break
        }
    }
    
    // Fall back to default if no match
    if selectedSteps == nil && s.defaultSteps != nil {
        selectedSteps = s.defaultSteps
    }
    
    // If still no steps, return empty iterator
    if selectedSteps == nil {
        return &Iterator{done: true}
    }
    
    // Use a sequence iterator for the selected branch
    return &Iterator{
        steps:   selectedSteps,
        scope:   scope,
        index:   0,
        outputs: make(map[string]string),
    }
}
```

### `pkg/steps/composite/switchstep/iterator.go`

```go
package switchstep

import (
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Iterator executes the selected branch sequentially.
// (Similar to sequence iterator)
type Iterator struct {
    steps   []steps.Step
    scope   *steps.Scope
    index   int
    outputs map[string]string
    done    bool
}

func (it *Iterator) Next(lastResults []*steps.Result) ([]steps.Step, bool, error) {
    if it.done {
        return nil, true, nil
    }
    
    // Process result from previous step
    if lastResults != nil && len(lastResults) > 0 {
        last := lastResults[0]
        
        if last.ExitCode != 0 {
            it.done = true
            return nil, true, &steps.StepError{ExitCode: last.ExitCode}
        }
        
        prevStep := it.steps[it.index-1]
        if prevStep.ID() != "" {
            it.scope = it.scope.WithStepOutputs(prevStep.ID(), last.Outputs)
            for k, v := range last.Outputs {
                it.outputs[k] = v
            }
        }
    }
    
    if it.index >= len(it.steps) {
        it.done = true
        return nil, true, nil
    }
    
    step := it.steps[it.index]
    it.index++
    
    return []steps.Step{step}, false, nil
}

func (it *Iterator) Result() *steps.Result {
    return steps.SuccessWithOutputs(it.outputs)
}

func (it *Iterator) Scope() *steps.Scope {
    return it.scope
}
```

---

## Layer 3: OCW Runtime (`pkg/ocw/`)

The runtime drives step execution using the iterator pattern.

### `pkg/ocw/runtime.go`

```go
package ocw

import (
    "context"
    "fmt"
    "sync"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Runtime executes OCW workflows
type Runtime struct {
    container container.Runtime
    logger    steps.Logger
    config    Config
}

// Config contains runtime configuration
type Config struct {
    // CleanupPolicy controls what happens to resources after execution
    // "always" (default), "on-success", "never"
    CleanupPolicy string
    
    // KeepBackground keeps background containers running after workflow completes
    KeepBackground bool
}

// Option configures the runtime
type Option func(*Runtime)

// WithLogger sets the logger
func WithLogger(l steps.Logger) Option {
    return func(r *Runtime) { r.logger = l }
}

// WithConfig sets the runtime configuration
func WithConfig(c Config) Option {
    return func(r *Runtime) { r.config = c }
}

// New creates a new runtime with the given container runtime
func New(cr container.Runtime, opts ...Option) *Runtime {
    r := &Runtime{
        container: cr,
        config:    Config{CleanupPolicy: "always"},
    }
    for _, opt := range opts {
        opt(r)
    }
    if r.logger == nil {
        r.logger = &noopLogger{}
    }
    return r
}

// Run executes a job from the workflow
func (r *Runtime) Run(ctx context.Context, wf *schema.OCW, jobName string) error {
    // 1. Find the job
    job := wf.GetJob(jobName)
    if job == nil {
        if wf.HasDirectFlow() && jobName == "" {
            return r.runDirectFlow(ctx, wf)
        }
        return fmt.Errorf("job %q not found", jobName)
    }
    
    // 2. Create execution context
    execCtx := r.newExecutionContext(ctx, wf)
    defer execCtx.Cleanup()
    
    // 3. Execute the job
    return r.runJob(ctx, job, execCtx)
}

// runStep executes a single step, handling both simple and composite steps
func (r *Runtime) runStep(ctx context.Context, step steps.Step, scope *steps.Scope, execCtx *ExecutionContext) (*steps.Result, error) {
    switch s := step.(type) {
    case steps.SimpleStep:
        return s.Execute(ctx, execCtx)
        
    case steps.CompositeStep:
        return r.runComposite(ctx, s, scope, execCtx)
        
    default:
        return nil, fmt.Errorf("unknown step type: %T", step)
    }
}

// runComposite executes a composite step using its iterator
func (r *Runtime) runComposite(ctx context.Context, step steps.CompositeStep, scope *steps.Scope, execCtx *ExecutionContext) (*steps.Result, error) {
    iter := step.Iterator(scope)
    var lastResults []*steps.Result
    
    for {
        // Get next step(s) from iterator
        nextSteps, done, err := iter.Next(lastResults)
        if err != nil {
            return nil, err
        }
        if done {
            return iter.Result(), nil
        }
        
        // Execute the step(s)
        // Multiple steps means parallel execution
        if len(nextSteps) == 0 {
            continue
        } else if len(nextSteps) == 1 {
            // Single step - execute directly
            // Get current scope from iterator if it supports it
            currentScope := scope
            if scopeProvider, ok := iter.(interface{ Scope() *steps.Scope }); ok {
                currentScope = scopeProvider.Scope()
            }
            
            result, err := r.runStep(ctx, nextSteps[0], currentScope, execCtx)
            if err != nil {
                return nil, err
            }
            lastResults = []*steps.Result{result}
        } else {
            // Multiple steps - execute in parallel
            lastResults, err = r.runParallel(ctx, nextSteps, scope, execCtx)
            if err != nil {
                return nil, err
            }
        }
    }
}

// runParallel executes multiple steps concurrently
func (r *Runtime) runParallel(ctx context.Context, stps []steps.Step, scope *steps.Scope, execCtx *ExecutionContext) ([]*steps.Result, error) {
    results := make([]*steps.Result, len(stps))
    errors := make([]error, len(stps))
    
    var wg sync.WaitGroup
    for i, step := range stps {
        wg.Add(1)
        go func(idx int, s steps.Step) {
            defer wg.Done()
            // Each parallel step gets a cloned scope
            results[idx], errors[idx] = r.runStep(ctx, s, scope.Clone(), execCtx)
        }(i, step)
    }
    wg.Wait()
    
    // Check for errors
    for i, err := range errors {
        if err != nil {
            return results, fmt.Errorf("parallel step %d (%s) failed: %w", i, stps[i].Name(), err)
        }
    }
    
    return results, nil
}

// runDirectFlow executes the workflow's direct flow control
func (r *Runtime) runDirectFlow(ctx context.Context, wf *schema.OCW) error {
    execCtx := r.newExecutionContext(ctx, wf)
    defer execCtx.Cleanup()
    
    scope := r.buildInitialScope(wf)
    
    switch wf.GetFlowType() {
    case "parallel":
        step, err := r.parseSchemaSteps(wf.Parallel, scope, execCtx, true)
        if err != nil {
            return err
        }
        _, err = r.runStep(ctx, step, scope, execCtx)
        return err
        
    case "sequence":
        step, err := r.parseSchemaSteps(wf.Sequence, scope, execCtx, false)
        if err != nil {
            return err
        }
        _, err = r.runStep(ctx, step, scope, execCtx)
        return err
        
    case "switch":
        // Handle switch at workflow level
        return r.runWorkflowSwitch(ctx, wf, scope, execCtx)
    }
    
    return fmt.Errorf("workflow has no steps")
}

// buildInitialScope creates the initial scope from workflow configuration
func (r *Runtime) buildInitialScope(wf *schema.OCW) *steps.Scope {
    scope := steps.NewScope()
    
    // Add environment variables
    for k, v := range wf.Env {
        scope.Env[k] = v.Value
    }
    
    // Add secrets
    for k, v := range wf.Secrets {
        if v.Secure != nil {
            scope.Secrets[k] = v.Secure.Secure // Would need decryption
        } else {
            scope.Secrets[k] = v.Plain
        }
    }
    
    // Add config
    for k, v := range wf.Config {
        scope.Config[k] = v
    }
    
    return scope
}
```

### `pkg/ocw/executor.go`

```go
package ocw

import (
    "context"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// ExecutionContext implements steps.Executor and holds state for a workflow run
type ExecutionContext struct {
    runtime  *Runtime
    workflow *schema.OCW
    workDir  string
    
    // Shared state
    outputs  *steps.OutputStore
    services *ServiceTracker
    
    // Resolved resources
    volumes map[string]steps.ResolvedVolume
    network container.NetworkID
    
    // Cleanup tracking
    cleanup []func() error
}

// Verify ExecutionContext implements Executor
var _ steps.Executor = (*ExecutionContext)(nil)

func (ec *ExecutionContext) Container() container.Runtime {
    return ec.runtime.container
}

func (ec *ExecutionContext) Outputs() *steps.OutputStore {
    return ec.outputs
}

func (ec *ExecutionContext) Logger() steps.Logger {
    return ec.runtime.logger
}

func (ec *ExecutionContext) WorkDir() string {
    return ec.workDir
}

func (ec *ExecutionContext) ResolvedVolumes() map[string]steps.ResolvedVolume {
    return ec.volumes
}

func (ec *ExecutionContext) RegisterService(id string, containerID container.ContainerID, hc *schema.HealthCheck) {
    ec.services.Register(id, containerID, hc)
    
    // Schedule cleanup unless KeepBackground is set
    if !ec.runtime.config.KeepBackground {
        ec.addCleanup(func() error {
            ec.runtime.container.Stop(context.Background(), containerID, 10*time.Second)
            return ec.runtime.container.Remove(context.Background(), containerID, true)
        })
    }
}

func (ec *ExecutionContext) WaitForServices(ctx context.Context, ids []string) error {
    return ec.services.WaitFor(ctx, ids, ec.runtime.container)
}

func (ec *ExecutionContext) Cleanup() {
    // Run cleanup functions in reverse order
    for i := len(ec.cleanup) - 1; i >= 0; i-- {
        if err := ec.cleanup[i](); err != nil {
            ec.runtime.logger.Warn("cleanup failed", "error", err)
        }
    }
}

func (ec *ExecutionContext) addCleanup(fn func() error) {
    ec.cleanup = append(ec.cleanup, fn)
}
```

### `pkg/ocw/job.go`

```go
package ocw

import (
    "context"
    "fmt"
    
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// runJob executes a job within an execution context
func (r *Runtime) runJob(ctx context.Context, job *schema.Job, ec *ExecutionContext) error {
    // Build initial scope from workflow and job
    scope := r.buildInitialScope(ec.workflow)
    
    // Execute based on flow type
    switch job.GetFlowType() {
    case "parallel":
        step, err := r.parseSchemaSteps(job.Parallel, scope, ec, true)
        if err != nil {
            return err
        }
        _, err = r.runStep(ctx, step, scope, ec)
        return err
        
    case "sequence":
        step, err := r.parseSchemaSteps(job.Sequence, scope, ec, false)
        if err != nil {
            return err
        }
        _, err = r.runStep(ctx, step, scope, ec)
        return err
        
    case "switch":
        return r.runJobSwitch(ctx, job, scope, ec)
        
    case "step":
        return r.runSingleStep(ctx, *job.Step, scope, ec)
        
    default:
        return fmt.Errorf("job has no steps")
    }
}

// parseSchemaSteps converts schema steps to executable steps
func (r *Runtime) parseSchemaSteps(schemaSteps []schema.Step, scope *steps.Scope, ec *ExecutionContext, parallel bool) (steps.Step, error) {
    children := make([]steps.Step, 0, len(schemaSteps))
    
    for _, ss := range schemaSteps {
        step, err := r.parseSchemaStep(ss, scope, ec)
        if err != nil {
            return nil, err
        }
        children = append(children, step)
    }
    
    if parallel {
        return parallel.New("", "", children), nil
    }
    return sequence.New("", "", children), nil
}

// parseSchemaStep converts a single schema step to an executable step
func (r *Runtime) parseSchemaStep(ss schema.Step, scope *steps.Scope, ec *ExecutionContext) (steps.Step, error) {
    switch {
    case ss.RunStep != nil:
        return run.Parse(ss.RunStep, scope, ec.volumes)
        
    case ss.BuildStep != nil:
        return build.Parse(ss.BuildStep, scope, ec.volumes)
        
    case ss.ParallelStep != nil:
        children, err := r.parseSchemaStepsSlice(ss.ParallelStep.Parallel, scope, ec)
        if err != nil {
            return nil, err
        }
        return parallel.New(string(ss.ParallelStep.ID), string(ss.ParallelStep.Name), children), nil
        
    case ss.SequenceStep != nil:
        children, err := r.parseSchemaStepsSlice(ss.SequenceStep.Sequence, scope, ec)
        if err != nil {
            return nil, err
        }
        return sequence.New(string(ss.SequenceStep.ID), string(ss.SequenceStep.Name), children), nil
        
    case ss.SwitchStep != nil:
        return r.parseSwitchStep(ss.SwitchStep, scope, ec)
        
    case ss.WorkflowStep != nil:
        // Future: implement workflow step
        return nil, fmt.Errorf("workflow steps not yet implemented")
        
    default:
        return nil, fmt.Errorf("unknown step type")
    }
}

func (r *Runtime) parseSchemaStepsSlice(schemaSteps []schema.Step, scope *steps.Scope, ec *ExecutionContext) ([]steps.Step, error) {
    children := make([]steps.Step, 0, len(schemaSteps))
    for _, ss := range schemaSteps {
        step, err := r.parseSchemaStep(ss, scope, ec)
        if err != nil {
            return nil, err
        }
        children = append(children, step)
    }
    return children, nil
}
```

### `pkg/ocw/services.go`

```go
package ocw

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
)

// ServiceTracker manages background services and their health
type ServiceTracker struct {
    mu       sync.RWMutex
    services map[string]*Service
}

// Service represents a tracked background container
type Service struct {
    ID          string
    ContainerID container.ContainerID
    HealthCheck *schema.HealthCheck
    Ready       chan struct{}
    Error       error
}

// NewServiceTracker creates a new service tracker
func NewServiceTracker() *ServiceTracker {
    return &ServiceTracker{
        services: make(map[string]*Service),
    }
}

// Register adds a service to track
func (st *ServiceTracker) Register(id string, containerID container.ContainerID, hc *schema.HealthCheck) {
    st.mu.Lock()
    defer st.mu.Unlock()
    
    svc := &Service{
        ID:          id,
        ContainerID: containerID,
        HealthCheck: hc,
        Ready:       make(chan struct{}),
    }
    st.services[id] = svc
    
    // If no health check, mark immediately ready
    if hc == nil {
        close(svc.Ready)
        return
    }
    
    // Start health check monitoring
    go st.monitorHealth(svc)
}

// WaitFor waits for the specified services to become healthy
func (st *ServiceTracker) WaitFor(ctx context.Context, ids []string, cr container.Runtime) error {
    st.mu.RLock()
    services := make([]*Service, 0, len(ids))
    for _, id := range ids {
        svc, ok := st.services[id]
        if !ok {
            st.mu.RUnlock()
            return fmt.Errorf("unknown service: %s", id)
        }
        services = append(services, svc)
    }
    st.mu.RUnlock()
    
    for _, svc := range services {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-svc.Ready:
            if svc.Error != nil {
                return fmt.Errorf("service %s unhealthy: %w", svc.ID, svc.Error)
            }
        }
    }
    
    return nil
}

func (st *ServiceTracker) monitorHealth(svc *Service) {
    // Simple implementation - a full version would:
    // 1. Wait for startPeriod
    // 2. Run health check at interval
    // 3. Track retries
    // 4. Set Error on permanent failure
    
    // For now, just close Ready after a short delay
    time.Sleep(100 * time.Millisecond)
    close(svc.Ready)
}
```

### `pkg/ocw/errors.go`

```go
package ocw

import "fmt"

// StepError represents a step execution failure
type StepError struct {
    StepID   string
    StepName string
    ExitCode int
    Err      error
}

func (e *StepError) Error() string {
    if e.StepName != "" {
        return fmt.Sprintf("step %q (id: %s) failed with exit code %d", e.StepName, e.StepID, e.ExitCode)
    }
    if e.StepID != "" {
        return fmt.Sprintf("step %s failed with exit code %d", e.StepID, e.ExitCode)
    }
    return fmt.Sprintf("step failed with exit code %d", e.ExitCode)
}

func (e *StepError) Unwrap() error {
    return e.Err
}

// JobError represents a job execution failure
type JobError struct {
    JobName string
    Err     error
}

func (e *JobError) Error() string {
    return fmt.Sprintf("job %q failed: %v", e.JobName, e.Err)
}

func (e *JobError) Unwrap() error {
    return e.Err
}
```

---

## Implementation Order

Implement in this order to allow incremental testing:

### Phase 1: Foundation
1. **`pkg/container/`** - All files (types, options, runtime interface, errors)
2. **`pkg/steps/step.go`** - Step, SimpleStep, CompositeStep interfaces
3. **`pkg/steps/iterator.go`** - StepIterator interface
4. **`pkg/steps/result.go`** - Result type
5. **`pkg/steps/scope.go`** - Scope type with Clone()
6. **`pkg/steps/outputs.go`** - OutputStore
7. **`pkg/steps/interpolate.go`** - Template interpolation
8. **`pkg/steps/executor.go`** - Executor interface

### Phase 2: Simple Steps
9. **`pkg/steps/simple/run/`** - Run step (most important, test first)
10. **`pkg/steps/simple/build/`** - Build step

### Phase 3: Composite Steps
11. **`pkg/steps/composite/sequence/`** - Sequence step + iterator
12. **`pkg/steps/composite/parallel/`** - Parallel step + iterator
13. **`pkg/steps/composite/switchstep/`** - Switch step + iterator

### Phase 4: Runtime Core
14. **`pkg/ocw/errors.go`** - Error types
15. **`pkg/ocw/services.go`** - Service tracker
16. **`pkg/ocw/executor.go`** - ExecutionContext (implements Executor)
17. **`pkg/ocw/job.go`** - Job execution and step parsing
18. **`pkg/ocw/runtime.go`** - Main runtime with step execution loop

### Phase 5: Advanced (Future)
19. **`pkg/steps/composite/workflow/`** - External workflow invocation

---

## Testing Strategy

1. **Unit tests for interpolation** - Test all template patterns
2. **Unit tests for iterators** - Test sequence, parallel, switch iteration logic
3. **Unit tests for parsers** - Test schema → step conversion
4. **Integration tests with mock container runtime** - Test step execution
5. **Integration tests with real Docker** - End-to-end workflow execution

Create a mock container runtime in `pkg/container/mock/` for testing:

```go
package mock

type Runtime struct {
    PullFunc        func(ctx context.Context, image string, opts PullOptions) error
    CreateFunc      func(ctx context.Context, opts CreateOptions) (ContainerID, error)
    StartFunc       func(ctx context.Context, id ContainerID) error
    WaitFunc        func(ctx context.Context, id ContainerID) (ExitResult, error)
    RemoveFunc      func(ctx context.Context, id ContainerID, force bool) error
    ImageExistsFunc func(ctx context.Context, image string) (bool, error)
    // ... etc
}

func (m *Runtime) Pull(ctx context.Context, image string, opts PullOptions) error {
    if m.PullFunc != nil {
        return m.PullFunc(ctx, image, opts)
    }
    return nil
}
// ... implement other methods
```

---

## Key Design Decisions

1. **Simple vs Composite separation**: Simple steps do work (Execute), composite steps control flow (Iterator). This gives the runtime full visibility and control.

2. **Iterator pattern for composites**: The runtime drives execution by calling `Next()` repeatedly. This:
   - Keeps call stacks flat
   - Makes loops natural (while, retry)
   - Enables future pause/resume support
   - Gives runtime visibility into execution flow

3. **Multiple steps from Next() = parallel**: When an iterator returns multiple steps, the runtime executes them in parallel. Single step = sequential.

4. **Scope accumulation**: The scope grows as steps complete. Sequence iterators maintain scope internally and update it after each child completes.

5. **Interpolation at parse time**: Templates are resolved when converting schema → executable step. Errors are caught early.

6. **Error in composite = composite failed**: Any error from a child step causes the composite to fail immediately. The runtime stops iteration on error.
