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
│  - Manages step lifecycle                                   │
│  - Tracks background services                               │
│  - Handles cleanup                                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: Steps (pkg/steps/)                                │
│  - Step interface + implementations                         │
│  - Parsers (schema → executable step with interpolation)    │
│  - run, build, parallel, sequence, switch, workflow         │
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
│   ├── step.go                # Step interface
│   ├── result.go              # Result type with outputs
│   ├── executor.go            # Executor interface (what steps receive)
│   ├── scope.go               # InterpolationScope for template resolution
│   ├── interpolate.go         # Template interpolation logic
│   │
│   ├── run/
│   │   ├── step.go            # RunStep implementation
│   │   └── parser.go          # Parses schema.RunStep → run.Step
│   │
│   ├── build/
│   │   ├── step.go            # BuildStep implementation
│   │   └── parser.go          # Parses schema.BuildStep → build.Step
│   │
│   ├── parallel/
│   │   ├── step.go            # Runs child steps concurrently
│   │   └── parser.go          # Parses schema.ParallelStep → parallel.Step
│   │
│   ├── sequence/
│   │   ├── step.go            # Runs child steps sequentially
│   │   └── parser.go          # Parses schema.SequenceStep → sequence.Step
│   │
│   ├── switchstep/            # "switch" is a reserved word
│   │   ├── step.go            # Conditional branching
│   │   └── parser.go          # Parses schema.SwitchStep → switchstep.Step
│   │
│   └── workflow/
│       ├── step.go            # Sub-workflow invocation
│       └── parser.go          # Parses schema.WorkflowStep → workflow.Step
│
└── ocw/                        # Layer 3: OCW Runtime
    ├── runtime.go             # Main Runtime struct and Run method
    ├── job.go                 # Job execution logic
    ├── context.go             # ExecutionContext (per-run shared state)
    ├── outputs.go             # OutputStore for {{ steps.x.y }} references
    ├── services.go            # Background service tracking & health checks
    ├── cleanup.go             # Resource cleanup logic
    └── errors.go              # Runtime error types
```

---

## Layer 1: Container Runtime Interface (`pkg/container/`)

This layer defines the abstract interface for container operations. The CLI will inject a concrete implementation (Docker, Podman, etc.).

### `pkg/container/types.go`

```go
package container

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
    Driver  string            // "bridge", "overlay", etc.
    Labels  map[string]string
    Internal bool             // No external connectivity
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

import "errors"

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

This layer defines the Step interface and provides implementations for each step type. Each step type has a parser that converts schema types to executable steps, performing template interpolation in the process.

### `pkg/steps/step.go`

```go
package steps

import "context"

// Step is an executable unit of work.
// Each step type (run, build, parallel, etc.) implements this interface.
type Step interface {
    // Execute runs the step and returns its result.
    // The Executor provides access to the container runtime and shared state.
    Execute(ctx context.Context, exec Executor) (*Result, error)
    
    // ID returns the step's identifier (for output references).
    // Returns empty string if the step has no ID.
    ID() string
    
    // Name returns the step's display name.
    Name() string
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
```

### `pkg/steps/executor.go`

```go
package steps

import (
    "context"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
)

// Executor provides the execution environment for steps.
// It is passed to Step.Execute() and provides access to:
// - The container runtime
// - Output storage for step communication  
// - The ability to run child steps (for composite steps)
// - Logging
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
    
    // RunStep executes a child step (used by parallel, sequence, switch steps).
    // The scope provides the interpolation context for template resolution.
    RunStep(ctx context.Context, step schema.Step, scope *Scope) (*Result, error)
    
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

// OutputStore provides thread-safe storage for step outputs.
// Outputs are stored as map[stepID]map[key]value.
type OutputStore struct {
    // Implementation uses sync.RWMutex for thread safety
}

// Get retrieves an output value: store.Get("stepID", "key")
func (s *OutputStore) Get(stepID, key string) (string, bool)

// GetAll retrieves all outputs for a step
func (s *OutputStore) GetAll(stepID string) (map[string]string, bool)

// Set stores output values for a step
func (s *OutputStore) Set(stepID string, outputs map[string]string)

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
func (s *Scope) Clone() *Scope

// WithStepOutputs returns a new scope with the given step's outputs added
func (s *Scope) WithStepOutputs(stepID string, outputs map[string]string) *Scope

// Merge combines another scope into this one (other takes precedence)
func (s *Scope) Merge(other *Scope)
```

### `pkg/steps/interpolate.go`

```go
package steps

import (
    "regexp"
    "strings"
)

// Interpolate resolves template expressions in a string.
// Supported syntax:
//   - {{ env.VAR_NAME }}        - Environment variable
//   - {{ secrets.SECRET_NAME }} - Secret value
//   - {{ inputs.INPUT_NAME }}   - Workflow input
//   - {{ steps.STEP_ID.KEY }}   - Output from a previous step
//   - {{ config.ns.key }}       - Configuration value
//
// Returns the interpolated string and any error encountered.
func Interpolate(template string, scope *Scope) (string, error)

// InterpolateMap interpolates all values in a map
func InterpolateMap(m map[string]string, scope *Scope) (map[string]string, error)

// InterpolateSlice interpolates all values in a slice
func InterpolateSlice(s []string, scope *Scope) ([]string, error)

// MustInterpolate is like Interpolate but panics on error (for testing)
func MustInterpolate(template string, scope *Scope) string
```

The interpolation implementation should:
1. Use regex to find `{{ ... }}` patterns
2. Parse the expression (e.g., `steps.api.host`)
3. Look up the value in the appropriate scope field
4. Replace the template with the resolved value
5. Return an error if a referenced value is not found

### Step Type: Run (`pkg/steps/run/`)

#### `pkg/steps/run/step.go`

```go
package run

import (
    "context"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes a container
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
    // 1. Wait for dependencies (needs)
    if len(s.needs) > 0 {
        if err := exec.WaitForServices(ctx, s.needs); err != nil {
            return nil, fmt.Errorf("waiting for services: %w", err)
        }
    }
    
    // 2. Pull image if needed
    if err := s.ensureImage(ctx, exec); err != nil {
        return nil, err
    }
    
    // 3. Create container
    containerID, err := exec.Container().Create(ctx, s.createOptions())
    if err != nil {
        return nil, fmt.Errorf("creating container: %w", err)
    }
    
    // 4. Start container
    if err := exec.Container().Start(ctx, containerID); err != nil {
        return nil, fmt.Errorf("starting container: %w", err)
    }
    
    // 5. Handle background vs foreground
    if s.background {
        // Register as service for health tracking
        exec.RegisterService(s.id, containerID, s.healthCheck)
        
        return &steps.Result{
            ContainerID:  string(containerID),
            IsBackground: true,
            Outputs:      s.buildServiceOutputs(containerID, exec),
        }, nil
    }
    
    // 6. Wait for completion
    result, err := exec.Container().Wait(ctx, containerID)
    if err != nil {
        return nil, fmt.Errorf("waiting for container: %w", err)
    }
    
    // 7. Cleanup
    if err := exec.Container().Remove(ctx, containerID, false); err != nil {
        exec.Logger().Warn("failed to remove container", "id", containerID, "error", err)
    }
    
    // 8. Return result
    if result.StatusCode != 0 {
        return steps.Failed(result.StatusCode), nil
    }
    
    return steps.Success(), nil
}

func (s *Step) ensureImage(ctx context.Context, exec steps.Executor) error {
    // Implement pull policy logic
}

func (s *Step) createOptions() container.CreateOptions {
    // Build CreateOptions from step fields
}

func (s *Step) buildServiceOutputs(id container.ContainerID, exec steps.Executor) map[string]string {
    // For background services, output host/port info
    // e.g., {"host": "localhost", "port": "5432"}
}
```

#### `pkg/steps/run/parser.go`

```go
package run

import (
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Parse converts a schema.RunStep into an executable run.Step.
// This is where template interpolation happens.
func Parse(rs *schema.RunStep, scope *steps.Scope, volumes map[string]steps.ResolvedVolume) (*Step, error) {
    // Interpolate all string fields
    image, err := steps.Interpolate(rs.Image, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating image: %w", err)
    }
    
    cmd, err := steps.Interpolate(rs.Cmd, scope)
    if err != nil {
        return nil, fmt.Errorf("interpolating cmd: %w", err)
    }
    
    // Interpolate environment variables
    env, err := s.parseEnv(rs, scope)
    if err != nil {
        return nil, err
    }
    
    // Resolve volume mounts
    mounts, err := s.resolveMounts(rs.Volumes, volumes)
    if err != nil {
        return nil, err
    }
    
    // Parse port exposures
    ports := s.parsePorts(rs.Expose)
    
    // Parse health check
    var healthCheck *container.HealthCheckConfig
    if rs.HealthCheck != nil {
        healthCheck = s.parseHealthCheck(rs.HealthCheck)
    }
    
    return &Step{
        id:          string(rs.ID),
        name:        string(rs.Name),
        image:       image,
        cmd:         parseCommand(cmd),
        entrypoint:  parseCommand(rs.Entrypoint),
        env:         env,
        workdir:     rs.Workdir,
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

func parseCommand(cmd string) []string {
    // Parse shell-style command into []string
    // Handle quoting, escaping, etc.
}

func parseCPUs(n *schema.NumberOrString) float64 {
    // Convert NumberOrString to float64
}

func parseMemory(mem string) int64 {
    // Parse "512m", "2g", etc. to bytes
}
```

### Step Type: Build (`pkg/steps/build/`)

Similar structure to run, but uses `container.Build()` instead. The parser converts `schema.BuildStep` to an executable step.

### Step Type: Parallel (`pkg/steps/parallel/`)

#### `pkg/steps/parallel/step.go`

```go
package parallel

import (
    "context"
    "sync"
    
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// Step executes child steps concurrently
type Step struct {
    id       string
    name     string
    children []schema.Step
    scope    *steps.Scope
}

func (s *Step) Execute(ctx context.Context, exec steps.Executor) (*steps.Result, error) {
    var wg sync.WaitGroup
    results := make([]*steps.Result, len(s.children))
    errors := make([]error, len(s.children))
    
    for i, child := range s.children {
        wg.Add(1)
        go func(idx int, step schema.Step) {
            defer wg.Done()
            // Each parallel branch gets its own scope clone
            results[idx], errors[idx] = exec.RunStep(ctx, step, s.scope.Clone())
        }(i, child)
    }
    
    wg.Wait()
    
    // Collect errors
    for i, err := range errors {
        if err != nil {
            return nil, fmt.Errorf("parallel step %d failed: %w", i, err)
        }
    }
    
    // Merge outputs from all children
    outputs := make(map[string]string)
    for _, result := range results {
        for k, v := range result.Outputs {
            outputs[k] = v
        }
    }
    
    return steps.SuccessWithOutputs(outputs), nil
}
```

### Step Type: Sequence (`pkg/steps/sequence/`)

Similar to parallel, but executes children sequentially. After each step completes, its outputs are added to the scope for subsequent steps.

### Step Type: Switch (`pkg/steps/switchstep/`)

Evaluates the switch expression, matches against cases, and executes the matching branch (or default).

### Step Type: Workflow (`pkg/steps/workflow/`)

Loads and executes another OCW workflow file. Handles inheritance settings for env/secrets.

---

## Layer 3: OCW Runtime (`pkg/ocw/`)

This layer orchestrates the execution of workflows and jobs.

### `pkg/ocw/runtime.go`

```go
package ocw

import (
    "context"
    
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
    return r
}

// Run executes a job from the workflow
func (r *Runtime) Run(ctx context.Context, wf *schema.OCW, jobName string) error {
    // 1. Find the job
    job := wf.GetJob(jobName)
    if job == nil {
        // Check if workflow has direct flow control
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

// RunDefault runs the default job or direct flow
func (r *Runtime) RunDefault(ctx context.Context, wf *schema.OCW) error {
    if wf.HasDirectFlow() {
        return r.runDirectFlow(ctx, wf)
    }
    
    jobs := wf.GetJobNames()
    if len(jobs) == 0 {
        return fmt.Errorf("workflow has no jobs or steps")
    }
    if len(jobs) > 1 {
        return fmt.Errorf("workflow has multiple jobs, specify which one to run")
    }
    
    return r.Run(ctx, wf, jobs[0])
}
```

### `pkg/ocw/context.go`

```go
package ocw

import (
    "context"
    "sync"
    
    "github.com/uncloud-cc/ocw/pkg/container"
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// ExecutionContext holds state for a single workflow execution
type ExecutionContext struct {
    runtime  *Runtime
    workflow *schema.OCW
    
    // Shared state
    outputs  *steps.OutputStore
    services *ServiceTracker
    
    // Resolved resources
    volumes  map[string]steps.ResolvedVolume
    network  container.NetworkID
    
    // Cleanup tracking
    cleanupMu sync.Mutex
    cleanup   []func() error
}

// newExecutionContext creates a new execution context
func (r *Runtime) newExecutionContext(ctx context.Context, wf *schema.OCW) *ExecutionContext {
    ec := &ExecutionContext{
        runtime:  r,
        workflow: wf,
        outputs:  steps.NewOutputStore(),
        services: NewServiceTracker(),
        volumes:  make(map[string]steps.ResolvedVolume),
    }
    
    // Resolve volumes
    ec.resolveVolumes(wf.Volumes)
    
    // Create workflow network (for service discovery)
    // All containers in the workflow will be connected to this network
    networkID, err := r.container.CreateNetwork(ctx, generateNetworkName(), container.NetworkOptions{})
    if err == nil {
        ec.network = networkID
        ec.addCleanup(func() error {
            return r.container.RemoveNetwork(context.Background(), networkID)
        })
    }
    
    return ec
}

// Implements steps.Executor interface
func (ec *ExecutionContext) Container() container.Runtime { return ec.runtime.container }
func (ec *ExecutionContext) Outputs() *steps.OutputStore  { return ec.outputs }
func (ec *ExecutionContext) Logger() steps.Logger          { return ec.runtime.logger }
func (ec *ExecutionContext) WorkDir() string               { /* ... */ }
func (ec *ExecutionContext) ResolvedVolumes() map[string]steps.ResolvedVolume { return ec.volumes }

func (ec *ExecutionContext) RunStep(ctx context.Context, step schema.Step, scope *steps.Scope) (*steps.Result, error) {
    // Dispatch to appropriate step parser/executor based on step type
    return ec.runtime.runSchemaStep(ctx, step, scope, ec)
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
    ec.cleanupMu.Lock()
    defer ec.cleanupMu.Unlock()
    
    // Run cleanup functions in reverse order
    for i := len(ec.cleanup) - 1; i >= 0; i-- {
        if err := ec.cleanup[i](); err != nil {
            ec.runtime.logger.Warn("cleanup failed", "error", err)
        }
    }
}

func (ec *ExecutionContext) addCleanup(fn func() error) {
    ec.cleanupMu.Lock()
    defer ec.cleanupMu.Unlock()
    ec.cleanup = append(ec.cleanup, fn)
}
```

### `pkg/ocw/job.go`

```go
package ocw

import (
    "context"
    
    "github.com/uncloud-cc/ocw/pkg/schema"
    "github.com/uncloud-cc/ocw/pkg/steps"
)

// runJob executes a job within an execution context
func (r *Runtime) runJob(ctx context.Context, job *schema.Job, ec *ExecutionContext) error {
    // Build initial scope from workflow and job env/secrets
    scope := r.buildInitialScope(ec.workflow, job)
    
    // Execute based on flow type
    switch job.GetFlowType() {
    case "parallel":
        return r.runParallel(ctx, job.Parallel, scope, ec)
    case "sequence":
        return r.runSequence(ctx, job.Sequence, scope, ec)
    case "switch":
        return r.runSwitch(ctx, *job.Switch, job.Case, job.Default, scope, ec)
    case "step":
        _, err := r.runSchemaStep(ctx, *job.Step, scope, ec)
        return err
    default:
        return fmt.Errorf("job has no steps")
    }
}

// runSequence executes steps sequentially, passing outputs forward
func (r *Runtime) runSequence(ctx context.Context, stps []schema.Step, scope *steps.Scope, ec *ExecutionContext) error {
    currentScope := scope.Clone()
    
    for _, step := range stps {
        result, err := r.runSchemaStep(ctx, step, currentScope, ec)
        if err != nil {
            return err
        }
        
        // Failed step aborts sequence
        if result.ExitCode != 0 {
            return &StepError{
                StepID:   getStepID(step),
                StepName: getStepName(step),
                ExitCode: result.ExitCode,
            }
        }
        
        // Add outputs to scope for next step
        if id := getStepID(step); id != "" {
            currentScope = currentScope.WithStepOutputs(id, result.Outputs)
            ec.outputs.Set(id, result.Outputs)
        }
    }
    
    return nil
}

// runParallel executes steps concurrently
func (r *Runtime) runParallel(ctx context.Context, stps []schema.Step, scope *steps.Scope, ec *ExecutionContext) error {
    // Implementation similar to parallel step
}

// runSwitch evaluates the switch expression and runs matching case
func (r *Runtime) runSwitch(ctx context.Context, expr string, cases map[string]schema.StepOrSteps, defaultCase *schema.StepOrSteps, scope *steps.Scope, ec *ExecutionContext) error {
    // Interpolate switch expression
    value, err := steps.Interpolate(expr, scope)
    if err != nil {
        return fmt.Errorf("evaluating switch expression: %w", err)
    }
    
    // Find matching case
    if stepsOrSteps, ok := cases[value]; ok {
        return r.runStepOrSteps(ctx, stepsOrSteps, scope, ec)
    }
    
    // Fall back to default
    if defaultCase != nil {
        return r.runStepOrSteps(ctx, *defaultCase, scope, ec)
    }
    
    // No match, no default - not an error
    return nil
}

// runSchemaStep dispatches to the appropriate step type
func (r *Runtime) runSchemaStep(ctx context.Context, step schema.Step, scope *steps.Scope, ec *ExecutionContext) (*steps.Result, error) {
    switch {
    case step.RunStep != nil:
        s, err := run.Parse(step.RunStep, scope, ec.volumes)
        if err != nil {
            return nil, fmt.Errorf("parsing run step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    case step.BuildStep != nil:
        s, err := build.Parse(step.BuildStep, scope, ec.volumes)
        if err != nil {
            return nil, fmt.Errorf("parsing build step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    case step.ParallelStep != nil:
        s, err := parallel.Parse(step.ParallelStep, scope)
        if err != nil {
            return nil, fmt.Errorf("parsing parallel step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    case step.SequenceStep != nil:
        s, err := sequence.Parse(step.SequenceStep, scope)
        if err != nil {
            return nil, fmt.Errorf("parsing sequence step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    case step.SwitchStep != nil:
        s, err := switchstep.Parse(step.SwitchStep, scope)
        if err != nil {
            return nil, fmt.Errorf("parsing switch step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    case step.WorkflowStep != nil:
        s, err := workflow.Parse(step.WorkflowStep, scope)
        if err != nil {
            return nil, fmt.Errorf("parsing workflow step: %w", err)
        }
        return s.Execute(ctx, ec)
        
    default:
        return nil, fmt.Errorf("unknown step type")
    }
}
```

### `pkg/ocw/services.go`

```go
package ocw

import (
    "context"
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
    Ready       chan struct{} // Closed when healthy
    Error       error         // Set if health check fails permanently
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
    
    // Start health check goroutine
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
    
    // Wait for all services
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
    // Implementation:
    // 1. Wait for startPeriod
    // 2. Run health check command at interval
    // 3. Track retries
    // 4. Close Ready channel on success, or set Error on permanent failure
}
```

### `pkg/ocw/outputs.go`

```go
package ocw

import "sync"

// OutputStore provides thread-safe storage for step outputs
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

// Get retrieves an output value
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
    return fmt.Sprintf("step %s failed with exit code %d", e.StepID, e.ExitCode)
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

## Implementation Order

Implement in this order to allow incremental testing:

### Phase 1: Foundation
1. **`pkg/container/`** - All files (types, options, runtime interface, errors)
2. **`pkg/steps/step.go`** - Step interface
3. **`pkg/steps/result.go`** - Result type
4. **`pkg/steps/scope.go`** - Scope type
5. **`pkg/steps/interpolate.go`** - Template interpolation
6. **`pkg/steps/executor.go`** - Executor interface

### Phase 2: Simple Steps
7. **`pkg/steps/run/`** - Run step (most important, test first)
8. **`pkg/steps/build/`** - Build step

### Phase 3: Composite Steps
9. **`pkg/steps/sequence/`** - Sequence step
10. **`pkg/steps/parallel/`** - Parallel step
11. **`pkg/steps/switchstep/`** - Switch step

### Phase 4: Runtime Core
12. **`pkg/ocw/outputs.go`** - Output store
13. **`pkg/ocw/errors.go`** - Error types
14. **`pkg/ocw/services.go`** - Service tracker
15. **`pkg/ocw/context.go`** - Execution context
16. **`pkg/ocw/job.go`** - Job execution
17. **`pkg/ocw/runtime.go`** - Main runtime

### Phase 5: Advanced
18. **`pkg/steps/workflow/`** - Workflow step (sub-workflow invocation)

---

## Testing Strategy

1. **Unit tests for interpolation** - Test all template patterns
2. **Unit tests for parsers** - Test schema → step conversion
3. **Integration tests with mock container runtime** - Test step execution
4. **Integration tests with real Docker** - End-to-end workflow execution

Create a mock container runtime in `pkg/container/mock/` for testing:

```go
package mock

type Runtime struct {
    PullFunc   func(ctx context.Context, image string, opts PullOptions) error
    CreateFunc func(ctx context.Context, opts CreateOptions) (ContainerID, error)
    // ... etc
}
```

---

## Key Design Decisions

1. **Interpolation at parse time**: Template strings are resolved when parsing schema → executable step, not during execution. This means:
   - Parsers receive a `Scope` with all available values
   - Errors are caught early (before container operations)
   - Steps receive fully-resolved values

2. **Executor interface for dependency injection**: Steps receive an `Executor` interface, not concrete types. This allows:
   - Easy testing with mocks
   - Decoupling steps from runtime implementation
   - Composite steps can delegate to runtime for child execution

3. **Service tracking in runtime**: Background containers are tracked by the runtime, not individual steps. This allows:
   - Centralized health monitoring
   - Coordinated cleanup
   - Cross-step service dependencies (`needs`)

4. **Scope cloning for parallel branches**: Each parallel branch gets its own scope clone. This prevents race conditions when multiple branches write outputs.

5. **Cleanup via deferred functions**: Resources are tracked with cleanup functions added to a slice. Cleanup runs in reverse order (LIFO), ensuring containers are removed before networks.
