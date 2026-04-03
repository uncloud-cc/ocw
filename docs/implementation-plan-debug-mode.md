# Implementation Plan: Debug Mode for OCW

This document provides a detailed implementation plan for adding a debug mode feature to OCW (Open Container Workflow). The debug mode will spin up a "netshoot" sidecar container that shares namespaces with the target container, allowing inspection of the running container or its filesystem if it has crashed.

## Overview

### Feature Requirements

1. **Schema-based debug mode**: Add `debug: true` to any step in the workflow YAML
2. **CLI debug mode**: Invoke via `ocw dev --debug` to enable debug mode for the entire job
3. **Sidecar container**: Spin up a netshoot container sharing namespaces with the target container (similar to `docker debug`)
4. **Custom debug image**: Allow users to specify a different debug image via `debug.image` in the schema

### User Experience Examples

```yaml
# Simple debug mode
- name: My Service
  image: myapp:latest
  debug: true

# Custom debug image
- name: My Service
  image: myapp:latest
  debug:
    image: nicolaka/netshoot:latest

# Full debug config (future extensibility)
- name: My Service
  image: myapp:latest
  debug:
    image: nicolaka/netshoot:latest
    # Future options could include:
    # cmd: /bin/bash
    # env: { ... }
```

CLI usage:
```bash
# Enable debug mode for entire job
ocw dev --debug

# Combine with other flags
ocw --debug --verbose dev
```

---

## Implementation Tasks

### Task 1: Add Debug Configuration to Schema

**File: `pkg/schema/steps.go`**

#### 1.1 Create DebugConfig struct

Add a new struct to represent the debug configuration:

```go
// DebugConfig represents debug sidecar configuration for a step
type DebugConfig struct {
    // Image is the debug sidecar image (default: nicolaka/netshoot)
    Image string `yaml:"image,omitempty" json:"image,omitempty"`
}
```

#### 1.2 Create Debug union type

Similar to `Watch`, create a union type that can be a bool or a full config object:

```go
// Debug can be a bool or a DebugConfig object
// - debug: true (uses default netshoot image)
// - debug: { image: "custom/debug:latest" }
type Debug struct {
    Bool   *bool
    Config *DebugConfig
}

// UnmarshalYAML implements custom unmarshaling for Debug
func (d *Debug) UnmarshalYAML(unmarshal func(interface{}) error) error {
    // Try bool first
    var b bool
    if err := unmarshal(&b); err == nil {
        d.Bool = &b
        return nil
    }

    // Try full config object
    var c DebugConfig
    if err := unmarshal(&c); err == nil {
        d.Config = &c
        return nil
    }

    return nil
}

// MarshalYAML implements custom marshaling for Debug
func (d Debug) MarshalYAML() (interface{}, error) {
    if d.Bool != nil {
        return *d.Bool, nil
    }
    if d.Config != nil {
        return d.Config, nil
    }
    return nil, nil
}

// IsEnabled returns true if debug mode is enabled
func (d *Debug) IsEnabled() bool {
    if d == nil {
        return false
    }
    if d.Bool != nil {
        return *d.Bool
    }
    return d.Config != nil
}

// GetImage returns the debug image to use (default: nicolaka/netshoot)
func (d *Debug) GetImage() string {
    const defaultDebugImage = "nicolaka/netshoot"
    
    if d == nil {
        return defaultDebugImage
    }
    if d.Config != nil && d.Config.Image != "" {
        return d.Config.Image
    }
    return defaultDebugImage
}
```

#### 1.3 Add Debug field to RunStep

In the `RunStep` struct, add the debug field:

```go
type RunStep struct {
    StepBase `yaml:",inline" json:",inline"`

    // ... existing fields ...

    // === Debug Mode ===
    // Debug enables a debug sidecar container that shares namespaces with this container.
    // The sidecar allows inspecting the running container or its filesystem.
    // Can be a bool (uses default netshoot image) or a config object with custom image.
    Debug *Debug `yaml:"debug,omitempty" json:"debug,omitempty"`

    // ... rest of existing fields ...
}
```

**Location for insertion**: After the `Watch` field block (around line 283), add the Debug field to maintain logical grouping.

---

### Task 2: Add CLI Flag for Debug Mode

**File: `cmd/ocw/main.go`**

#### 2.1 Add debug flag

In the `run()` function, add a new flag:

```go
debug := flag.Bool("debug", false, "Enable debug mode (spawn netshoot sidecar for all containers)")
```

**Location**: Add after the `verbose` flag definition (around line 67).

#### 2.2 Update flag.Usage

Add documentation for the new flag in the usage text:

```go
fmt.Fprintf(os.Stderr, "  ocw -debug dev              Run 'dev' job with debug sidecars\n")
```

#### 2.3 Pass debug flag to Runner

After creating the runner, pass the debug flag:

```go
if *debug {
    r.WithDebug(true)
}
```

---

### Task 3: Update Runner to Support Debug Mode

**File: `pkg/runner/runner.go`**

#### 3.1 Add debug field to Runner struct

```go
type Runner struct {
    // ... existing fields ...
    
    // debug enables debug sidecar containers for all steps
    debug bool
    
    // debugContainers tracks running debug sidecar containers for cleanup
    debugContainers []string
    // debugMu protects debugContainers slice
    debugMu sync.Mutex
}
```

#### 3.2 Add WithDebug method

```go
// WithDebug enables or disables debug mode for all containers
func (r *Runner) WithDebug(debug bool) *Runner {
    r.debug = debug
    return r
}
```

#### 3.3 Add debug container registration

```go
// registerDebugContainer adds a debug container to the cleanup list
func (r *Runner) registerDebugContainer(name string) {
    r.debugMu.Lock()
    defer r.debugMu.Unlock()
    r.debugContainers = append(r.debugContainers, name)
}
```

#### 3.4 Update cleanupBackgroundContainers

Modify to also clean up debug containers:

```go
func (r *Runner) cleanupBackgroundContainers() {
    // Stop reloader first
    if r.reloader != nil {
        r.reloader.Stop()
    }

    // Clean up debug containers first (they depend on main containers)
    r.debugMu.Lock()
    debugContainers := make([]string, len(r.debugContainers))
    copy(debugContainers, r.debugContainers)
    r.debugContainers = r.debugContainers[:0]
    r.debugMu.Unlock()

    if len(debugContainers) > 0 {
        r.Output("\n%s\n", r.styles.Dim(fmt.Sprintf("Cleaning up %d debug container(s)...", len(debugContainers))))
        ctx := context.Background()
        for _, name := range debugContainers {
            if err := r.docker.StopContainer(ctx, name); err != nil {
                r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to stop debug container %s: %v", name, err)))
            }
            if err := r.docker.RemoveContainer(ctx, name); err != nil {
                r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to remove debug container %s: %v", name, err)))
            }
        }
    }

    // Then clean up main containers (existing code)
    // ... rest of existing implementation ...
}
```

#### 3.5 Update runRunStep to spawn debug sidecar

In the `runRunStep` function, after successfully starting the main container, add logic to spawn the debug sidecar:

```go
// After: if err := r.docker.RunContainer(ctx, opts); err != nil { ... }

// Spawn debug sidecar if debug mode is enabled
debugEnabled := (step.Debug != nil && step.Debug.IsEnabled()) || r.debug
if debugEnabled && step.Background && containerName != "" {
    debugImage := "nicolaka/netshoot" // default
    if step.Debug != nil {
        debugImage = step.Debug.GetImage()
    }
    
    debugContainerName := containerName + "-debug"
    
    r.Output("  %s %s\n", r.styles.Label("Debug:"), r.styles.Value(fmt.Sprintf("spawning sidecar (%s)", debugImage)))
    
    if err := r.spawnDebugSidecar(ctx, containerName, debugContainerName, debugImage); err != nil {
        r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to spawn debug sidecar: %v", err)))
    } else {
        r.registerDebugContainer(debugContainerName)
        r.Output("  %s %s\n", r.styles.Label("Debug container:"), r.styles.Value(debugContainerName))
        r.Output("  %s %s\n", r.styles.Dim("Attach with:"), r.styles.Value(fmt.Sprintf("docker exec -it %s /bin/bash", debugContainerName)))
    }
}
```

#### 3.6 Add spawnDebugSidecar method

```go
// spawnDebugSidecar creates a debug sidecar container that shares namespaces with the target container
func (r *Runner) spawnDebugSidecar(ctx context.Context, targetContainer, debugContainer, debugImage string) error {
    // The sidecar shares:
    // - PID namespace (can see target's processes)
    // - Network namespace (can inspect network)
    // - IPC namespace (can inspect shared memory)
    // - Also mounts target's filesystem via /proc/<pid>/root
    
    return r.docker.RunDebugSidecar(ctx, DebugSidecarOptions{
        TargetContainer: targetContainer,
        DebugContainer:  debugContainer,
        DebugImage:      debugImage,
        Network:         r.networkName,
    })
}
```

---

### Task 4: Add Docker Debug Sidecar Support

**File: `pkg/runner/docker.go`**

#### 4.1 Add DebugSidecarOptions struct

```go
// DebugSidecarOptions holds options for creating a debug sidecar container
type DebugSidecarOptions struct {
    TargetContainer string // Name of the container to debug
    DebugContainer  string // Name for the debug sidecar container
    DebugImage      string // Image to use for the sidecar (e.g., nicolaka/netshoot)
    Network         string // Network to connect to
}
```

#### 4.2 Add RunDebugSidecar method

This method creates a container that shares namespaces with the target, similar to `docker debug`:

```go
// RunDebugSidecar creates a debug sidecar container sharing namespaces with the target container.
// This is similar to `docker debug` - it creates a container that can:
// - See the target container's processes (--pid=container:...)
// - Access the target's network namespace (--network=container:...)
// - Inspect the target's filesystem via /proc/1/root
func (d *Docker) RunDebugSidecar(ctx context.Context, opts DebugSidecarOptions) error {
    if d.verbose {
        d.Output("  %s Creating debug sidecar for container: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.TargetContainer))
    }

    // Build docker run command with namespace sharing
    args := []string{
        "run",
        "-d",                                              // Detached
        "--name", opts.DebugContainer,                     // Container name
        "--pid=container:" + opts.TargetContainer,         // Share PID namespace
        "--network=container:" + opts.TargetContainer,     // Share network namespace
        "--ipc=container:" + opts.TargetContainer,         // Share IPC namespace
        // Note: We don't share the mount namespace because that would hide the debug tools.
        // Instead, users can access the target's filesystem via /proc/1/root
        "--init",                                          // Use tini for proper signal handling
        "--cap-add=SYS_PTRACE",                            // Allow debugging processes
        "--cap-add=SYS_ADMIN",                             // Allow various admin operations
        opts.DebugImage,                                   // The debug image
        "sleep", "infinity",                               // Keep the container running
    }

    if d.verbose {
        d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
    }

    cmd := exec.CommandContext(ctx, "docker", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to create debug sidecar: %w\nOutput: %s", err, string(output))
    }

    if d.verbose {
        d.Output("  %s Debug sidecar created successfully\n", d.styles.Dim("[verbose]"))
    }

    return nil
}
```

---

### Task 5: Update JSON Schema Generation

**File: `pkg/schema/jsonschema.go`** (if applicable) or regenerate schema

The JSON schema needs to include the new `debug` field. If the schema is auto-generated from Go structs via reflection tags, ensure the tags are correct:

```go
Debug *Debug `yaml:"debug,omitempty" json:"debug,omitempty" jsonschema:"oneOf"`
```

After implementing the schema changes, regenerate the schema:

```bash
make schema  # or: go run cmd/schema-gen/main.go > schema.json
```

---

### Task 6: Add Validation for Debug Field

**File: `pkg/schema/validate.go`**

Add validation to ensure debug configuration is valid:

```go
// In the validateRunStep function (or equivalent):
func (v *validator) validateDebug(debug *Debug, path string) {
    if debug == nil {
        return
    }
    
    if debug.Config != nil && debug.Config.Image != "" {
        // Validate image name format if needed
        // For now, we just accept any non-empty string
        // Docker will validate the actual image reference
    }
}
```

---

### Task 7: Initialize debugContainers Slice

**File: `pkg/runner/runner.go`**

Update `NewRunner` to initialize the debug containers slice:

```go
func NewRunner(workflowDir string) *Runner {
    styles := NewStyles()
    output := func(format string, args ...any) {
        fmt.Printf(format, args...)
    }
    return &Runner{
        WorkflowDir:          workflowDir,
        Output:               output,
        docker:               NewDocker(output, styles, nil),
        builtImages:          make(map[string]string),
        builtImageConfigs:    make(map[string]*schema.BuildConfig),
        backgroundContainers: make([]string, 0),
        debugContainers:      make([]string, 0),  // ADD THIS LINE
        exposedServices:      make([]ExposedService, 0),
        templateCtx:          NewTemplateContext(),
        styles:               styles,
        secretEnvKeys:        make(map[string]bool),
    }
}
```

---

### Task 8: Handle Debug Mode for Non-Background Containers

For foreground (non-background) containers, debug mode needs special handling since the container exits after execution. Options:

#### Option A: Convert to background mode when debug is enabled (Recommended)

```go
// In runRunStep, before running the container:
if step.Debug != nil && step.Debug.IsEnabled() || r.debug {
    if !step.Background {
        r.Output("  %s\n", r.styles.Info("Debug mode: running container in background for inspection"))
        step.Background = true
        // Also ensure it doesn't auto-remove
    }
}
```

#### Option B: Only support debug for background containers

Add validation that warns/errors if debug is used on non-background containers:

```go
if step.Debug != nil && step.Debug.IsEnabled() && !step.Background {
    r.Output("  %s\n", r.styles.Warning("Warning: debug mode only works with background containers"))
}
```

**Recommendation**: Implement Option A for better UX, but print a notice so users understand the behavior change.

---

### Task 9: Handle Container Crash Scenarios

When a container crashes, the debug sidecar should remain accessible for post-mortem inspection. The current implementation using `--pid=container:...` will keep the sidecar running even if the target crashes, because:

1. The sidecar has its own PID 1 (`sleep infinity`)
2. Docker maintains the namespace references even if the target exits

However, add handling for graceful messaging:

```go
// In the debug sidecar output section:
r.Output("  %s\n", r.styles.Dim("Note: Debug sidecar remains accessible even if main container crashes"))
r.Output("  %s\n", r.styles.Dim("Access target filesystem via: /proc/1/root (while running) or container's filesystem"))
```

---

### Task 10: Add Example and Documentation

**File: `examples/debug_mode.yaml`** (new file)

```yaml
schemaVersion: "0.1.0"
name: Debug Mode Example
description: Demonstrates the debug mode feature for container inspection

jobs:
  debug-basic:
    name: Basic Debug Mode
    description: Simple debug mode with default netshoot image
    sequence:
      - name: Web Server with Debug
        id: webserver
        image: nginx:alpine
        background: true
        expose: 8080
        healthCheck:
          cmd: curl -f http://localhost:80/ || exit 1
        debug: true

  debug-custom-image:
    name: Custom Debug Image
    description: Debug mode with a custom debug image
    sequence:
      - name: App with Custom Debug
        id: app
        image: python:3.11-slim
        cmd: python -m http.server 8000
        background: true
        expose: 8000
        debug:
          image: busybox:latest

  debug-cli:
    name: CLI Debug Mode
    description: Run with --debug flag to enable debug sidecars for all containers
    parallel:
      - name: Service A
        id: svc-a
        image: alpine
        cmd: sleep infinity
        background: true
      
      - name: Service B
        id: svc-b
        image: alpine
        cmd: sleep infinity
        background: true
```

---

## Testing Plan

### Manual Testing

1. **Basic debug mode**:
   ```bash
   # Create a simple workflow with debug: true
   ocw debug-basic
   # Verify: debug sidecar container is created
   docker ps | grep debug
   # Verify: can exec into debug container
   docker exec -it <container>-debug /bin/bash
   ```

2. **CLI debug flag**:
   ```bash
   ocw --debug debug-cli
   # Verify: all background containers have debug sidecars
   ```

3. **Custom debug image**:
   ```bash
   ocw debug-custom-image
   # Verify: sidecar uses the custom image (busybox)
   ```

4. **Crash inspection**:
   ```bash
   # Create a workflow where container crashes
   # Verify: debug sidecar remains running
   # Verify: can inspect via /proc or filesystem
   ```

5. **Cleanup**:
   ```bash
   # Start workflow with debug
   # Press Ctrl+C
   # Verify: both main and debug containers are cleaned up
   ```

### Unit Tests

Add tests in `pkg/schema/steps_test.go`:

```go
func TestDebugUnmarshal(t *testing.T) {
    tests := []struct {
        name     string
        yaml     string
        enabled  bool
        image    string
    }{
        {
            name:    "bool true",
            yaml:    "debug: true",
            enabled: true,
            image:   "nicolaka/netshoot",
        },
        {
            name:    "bool false",
            yaml:    "debug: false",
            enabled: false,
            image:   "nicolaka/netshoot",
        },
        {
            name:    "custom image",
            yaml:    "debug:\n  image: busybox:latest",
            enabled: true,
            image:   "busybox:latest",
        },
    }
    // ... test implementation
}
```

---

## File Summary

| File | Changes |
|------|---------|
| `pkg/schema/steps.go` | Add `Debug`, `DebugConfig` types and `Debug` field to `RunStep` |
| `cmd/ocw/main.go` | Add `--debug` CLI flag, pass to runner |
| `pkg/runner/runner.go` | Add debug support: field, methods, sidecar spawning, cleanup |
| `pkg/runner/docker.go` | Add `RunDebugSidecar` method with namespace sharing |
| `pkg/schema/validate.go` | Add debug validation (optional) |
| `schema.json` | Regenerate to include debug field |
| `examples/debug_mode.yaml` | Add example workflow (new file) |

---

## Implementation Order

1. **Schema changes** (`pkg/schema/steps.go`) - Define the data structures
2. **CLI flag** (`cmd/ocw/main.go`) - Add the `--debug` flag  
3. **Docker support** (`pkg/runner/docker.go`) - Add `RunDebugSidecar` method
4. **Runner integration** (`pkg/runner/runner.go`) - Wire everything together
5. **Validation** (`pkg/schema/validate.go`) - Add any necessary validation
6. **Schema regeneration** - Run `make schema` to update JSON schema
7. **Examples and testing** - Add example workflow and test manually

---

## Edge Cases to Handle

1. **Container already exists**: Handle case where debug container name conflicts
2. **Target container not found**: Handle race condition where target exits before sidecar starts
3. **Image pull failure**: Handle case where debug image can't be pulled
4. **Network isolation**: Ensure debug sidecar is in same network as target
5. **Multiple debug requests**: Prevent multiple debug sidecars for same container
6. **Non-background containers**: Decide behavior (convert to background vs warn)

---

## Future Enhancements

1. **Interactive debug session**: `ocw debug <container-id>` to attach directly
2. **Debug shell selection**: Allow specifying shell (bash, sh, zsh)
3. **Debug environment**: Pass custom env vars to debug container
4. **Debug volumes**: Mount additional volumes in debug container
5. **Debug on failure**: Automatically spawn debug sidecar only when container fails
