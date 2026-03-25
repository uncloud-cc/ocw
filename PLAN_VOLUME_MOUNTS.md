# Volume Mounts Feature - Implementation Plan

## Overview

This document describes the implementation plan for adding named volume mounts to OCW workflows. This feature allows workflows to declare explicit access to host filesystem directories, enabling use cases like Makefile replacements where containers need to read source files and write build outputs to the real filesystem.

**Note**: This feature is independent of the immutable containers feature (FUSE-based copy-on-write for `/workflow`). Volume mounts provide explicit, controlled access to host directories, while immutable containers (separate plan) will make the default `/workflow` mount read-only with copy-on-write semantics.

## Core Concepts

### 1. Named Volumes

A new top-level `volumes` field allows declaring named volumes that map to host directories:

```yaml
volumes:
  src:
    path: ./src
    # mode defaults to "ro" (read-only)
  dist:
    path: ./dist
    mode: rw  # explicit read-write access
    mountPath: /output  # custom default mount path (instead of /volumes/dist)
```

### 2. Volume References in Steps/Jobs

Volumes are explicitly granted to steps or jobs via a `volumes` field:

```yaml
jobs:
  build:
    volumes:
      - src  # All steps in this job get access
    sequence:
      - name: Build
        image: node:20
        volumes:
          - dist  # This step also gets dist access
        cmd: cp -r /volumes/src/build/* /volumes/dist/
```

### 3. Mode Restriction Rules

**Volumes can only be made MORE restrictive when mounted, never less restrictive:**

- A volume defined as `rw` (read-write) can be mounted as `readonly: true` in a step
- A volume defined as `ro` (read-only) **cannot** be mounted as read-write in a step
- This ensures workflow authors maintain control over filesystem access

```yaml
volumes:
  config:
    path: ./config
    # mode: ro (default)
  output:
    path: ./output
    mode: rw

jobs:
  build:
    sequence:
      # VALID: rw volume mounted as readonly for this step
      - name: Read output
        image: alpine
        volumes:
          - name: output
            readonly: true
        cmd: cat /volumes/output/log.txt
      
      # INVALID: would error - cannot upgrade ro volume to rw
      # - name: Write config
      #   image: alpine
      #   volumes:
      #     - name: config
      #       readonly: false  # ERROR: cannot make ro volume writable
```

---

## Schema Changes

### File: `pkg/schema/schema.go`

Add new `Volumes` field to the `OCW` struct:

```go
type OCW struct {
    SchemaVersion string            `yaml:"schemaVersion" json:"schemaVersion" jsonschema:"required"`
    Name          Name              `yaml:"name" json:"name" jsonschema:"required"`
    ID            ID                `yaml:"id,omitempty" json:"id,omitempty"`
    Description   Description       `yaml:"description,omitempty" json:"description,omitempty"`
    Config        Config            `yaml:"config,omitempty" json:"config,omitempty"`
    Env           Env               `yaml:"env,omitempty" json:"env,omitempty"`
    Secrets       Secrets           `yaml:"secrets,omitempty" json:"secrets,omitempty"`
    Outputs       Outputs           `yaml:"outputs,omitempty" json:"outputs,omitempty"`
    
    // NEW: Named volumes for host filesystem access
    Volumes       Volumes           `yaml:"volumes,omitempty" json:"volumes,omitempty"`
    
    Jobs          Jobs              `yaml:"jobs,omitempty" json:"jobs,omitempty"`
    // ... rest of fields
}
```

### File: `pkg/schema/volumes.go` (NEW FILE)

Create a new file to define volume-related types:

```go
package schema

// VolumeMode defines the access mode for a volume
type VolumeMode string

const (
    VolumeModeReadOnly  VolumeMode = "ro"
    VolumeModeReadWrite VolumeMode = "rw"
)

// Volume defines a named volume that provides access to host filesystem
type Volume struct {
    // Path to the host directory (relative to workflow file or absolute)
    Path string `yaml:"path" json:"path" jsonschema:"required"`
    
    // Access mode: "ro" (read-only, default) or "rw" (read-write)
    // Default: "ro" - volumes are read-only unless explicitly set to "rw"
    Mode VolumeMode `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"enum=ro,enum=rw,default=ro"`
    
    // Default mount path inside containers (default: /volumes/<name>)
    // Can be overridden per-step/job in volume references
    MountPath string `yaml:"mountPath,omitempty" json:"mountPath,omitempty"`
}

// Volumes is a map of volume names to their definitions
type Volumes map[string]Volume

// VolumeRef references a named volume when mounting in steps/jobs
type VolumeRef struct {
    // Name of the volume (must match a key in top-level volumes)
    Name string `yaml:"name" json:"name" jsonschema:"required"`
    
    // Override mount path inside the container for this specific mount
    // If not specified, uses the volume's mountPath or defaults to /volumes/<name>
    MountPath string `yaml:"mountPath,omitempty" json:"mountPath,omitempty"`
    
    // Force read-only access for this mount, even if volume is defined as rw
    // Default: false (uses volume's mode)
    // NOTE: Can only make volumes MORE restrictive (rw -> ro), never less restrictive
    // Setting readonly: false on a volume defined as "ro" will cause a validation error
    ReadOnly *bool `yaml:"readonly,omitempty" json:"readonly,omitempty"`
}

// VolumeRefs is a list of volume references for a step/job
// Can be specified as:
// - A single string (volume name)
// - An array of strings (volume names)
// - An array of VolumeRef objects (with mount path/readonly overrides)
type VolumeRefs []VolumeRef

// UnmarshalYAML implements custom unmarshaling for VolumeRefs
// to support both string shorthand and full object notation
func (v *VolumeRefs) UnmarshalYAML(unmarshal func(interface{}) error) error {
    // Try single string first
    var single string
    if err := unmarshal(&single); err == nil {
        *v = []VolumeRef{{Name: single}}
        return nil
    }
    
    // Try array of strings
    var strings []string
    if err := unmarshal(&strings); err == nil {
        refs := make([]VolumeRef, len(strings))
        for i, s := range strings {
            refs[i] = VolumeRef{Name: s}
        }
        *v = refs
        return nil
    }
    
    // Try array of VolumeRef objects
    var refs []VolumeRef
    if err := unmarshal(&refs); err != nil {
        return err
    }
    *v = refs
    return nil
}
```

### File: `pkg/schema/steps.go`

Add `volumes` field to `RunStep` and `BuildStep`:

```go
type RunStep struct {
    StepBase     `yaml:",inline"`
    Image        string      `yaml:"image" json:"image" jsonschema:"required"`
    Cmd          string      `yaml:"cmd,omitempty" json:"cmd,omitempty"`
    Args         []string    `yaml:"args,omitempty" json:"args,omitempty"`
    Entrypoint   string      `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
    Shell        string      `yaml:"shell,omitempty" json:"shell,omitempty"`
    Workdir      string      `yaml:"workdir,omitempty" json:"workdir,omitempty"`
    Env          Env         `yaml:"env,omitempty" json:"env,omitempty"`
    HealthCheck  *HealthCheck `yaml:"healthcheck,omitempty" json:"healthcheck,omitempty"`
    Expose       *Expose     `yaml:"expose,omitempty" json:"expose,omitempty"`
    Watch        *Watch      `yaml:"watch,omitempty" json:"watch,omitempty"`
    Background   bool        `yaml:"background,omitempty" json:"background,omitempty"`
    
    // NEW: Volume access for this step
    Volumes      VolumeRefs  `yaml:"volumes,omitempty" json:"volumes,omitempty"`
}

type BuildStep struct {
    StepBase     `yaml:",inline"`
    Build        BuildConfig `yaml:"build" json:"build" jsonschema:"required"`
    Push         bool        `yaml:"push,omitempty" json:"push,omitempty"`
    
    // NEW: Volume access during build
    Volumes      VolumeRefs  `yaml:"volumes,omitempty" json:"volumes,omitempty"`
}
```

### File: `pkg/schema/jobs.go`

Add `volumes` field to `Job`:

```go
type Job struct {
    Name        Name        `yaml:"name,omitempty" json:"name,omitempty"`
    Description Description `yaml:"description,omitempty" json:"description,omitempty"`
    Env         Env         `yaml:"env,omitempty" json:"env,omitempty"`
    Needs       []string    `yaml:"needs,omitempty" json:"needs,omitempty"`
    Outputs     Outputs     `yaml:"outputs,omitempty" json:"outputs,omitempty"`
    
    // NEW: Volume access for all steps in this job
    Volumes     VolumeRefs  `yaml:"volumes,omitempty" json:"volumes,omitempty"`
    
    // Job content (one of)
    Parallel    []Step      `yaml:"parallel,omitempty" json:"parallel,omitempty"`
    Sequence    []Step      `yaml:"sequence,omitempty" json:"sequence,omitempty"`
    // ... switch/case fields
}
```

---

## Runner Changes

### File: `pkg/runner/runner.go`

#### Add resolvedVolumes to Runner

```go
type Runner struct {
    WorkflowDir           string
    WorkflowFile          string
    EnvFile               string
    Output                func(...)
    podman                *Podman
    builtImages           map[string]string
    backgroundContainers  []string
    networkName           string
    exposedServices       []ExposedService
    templateCtx           *TemplateContext
    styles                *Styles
    secretEnvKeys         map[string]bool
    secretValues          []string
    showSecrets           bool
    force                 bool
    runID                 string
    reloader              *Reloader
    builtImageConfigs     map[string]*BuildConfig
    
    // NEW: Resolved volumes from workflow schema
    resolvedVolumes       map[string]*ResolvedVolume
}

// ResolvedVolume contains the resolved paths for a workflow volume
type ResolvedVolume struct {
    Name       string
    HostPath   string             // Absolute path on host
    Mode       schema.VolumeMode  // "ro" or "rw"
    MountPath  string             // Default mount path (empty = /volumes/<name>)
}
```

#### Initialize Volumes in Run()

In the `Run()` method, resolve workflow volumes:

```go
func (r *Runner) Run(ctx context.Context, cfg RunConfig) error {
    // ... existing setup code ...
    
    // Resolve workflow volumes
    if err := r.resolveVolumes(ocwFile.Volumes); err != nil {
        return fmt.Errorf("failed to resolve volumes: %w", err)
    }
    
    // ... rest of Run() ...
}

// resolveVolumes resolves volume paths from the workflow schema
func (r *Runner) resolveVolumes(volumes schema.Volumes) error {
    r.resolvedVolumes = make(map[string]*ResolvedVolume)
    
    for name, vol := range volumes {
        hostPath := vol.Path
        if !filepath.IsAbs(hostPath) {
            hostPath = filepath.Join(r.WorkflowDir, vol.Path)
        }
        
        absPath, err := filepath.Abs(hostPath)
        if err != nil {
            return fmt.Errorf("volume %q: %w", name, err)
        }
        
        // Verify path exists
        if _, err := os.Stat(absPath); err != nil {
            return fmt.Errorf("volume %q path does not exist: %s", name, absPath)
        }
        
        mode := vol.Mode
        if mode == "" {
            mode = schema.VolumeModeReadOnly
        }
        
        r.resolvedVolumes[name] = &ResolvedVolume{
            Name:      name,
            HostPath:  absPath,
            Mode:      mode,
            MountPath: vol.MountPath,  // May be empty; resolved later
        }
    }
    
    return nil
}
```

#### Volume Mount Resolution

Add methods to resolve volume mounts for a step:

```go
// resolveStepVolumes resolves volume references for a step
// Returns a list of mount specifications for podman
func (r *Runner) resolveStepVolumes(stepVolumes, jobVolumes schema.VolumeRefs) ([]VolumeMount, error) {
    var mounts []VolumeMount
    seen := make(map[string]bool)
    
    // Process step-level volumes (highest priority)
    for _, ref := range stepVolumes {
        if seen[ref.Name] {
            continue
        }
        seen[ref.Name] = true
        
        mount, err := r.resolveVolumeRef(ref)
        if err != nil {
            return nil, err
        }
        mounts = append(mounts, mount)
    }
    
    // Process job-level volumes
    for _, ref := range jobVolumes {
        if seen[ref.Name] {
            continue
        }
        seen[ref.Name] = true
        
        mount, err := r.resolveVolumeRef(ref)
        if err != nil {
            return nil, err
        }
        mounts = append(mounts, mount)
    }
    
    return mounts, nil
}

// VolumeMount represents a resolved volume mount
type VolumeMount struct {
    HostPath      string
    ContainerPath string
    ReadOnly      bool
}

func (r *Runner) resolveVolumeRef(ref schema.VolumeRef) (VolumeMount, error) {
    vol, ok := r.resolvedVolumes[ref.Name]
    if !ok {
        return VolumeMount{}, fmt.Errorf("volume %q not defined", ref.Name)
    }
    
    // Determine mount path (ref override > volume default > /volumes/<name>)
    mountPath := ref.MountPath
    if mountPath == "" {
        mountPath = vol.MountPath
    }
    if mountPath == "" {
        mountPath = "/volumes/" + ref.Name
    }
    
    // Determine if mount should be read-only
    // Start with volume's mode
    readOnly := vol.Mode == schema.VolumeModeReadOnly || vol.Mode == ""
    
    // ref.ReadOnly can only make it MORE restrictive (rw -> ro)
    // It cannot make a ro volume writable
    if ref.ReadOnly != nil {
        if *ref.ReadOnly {
            // Always allowed: making mount read-only
            readOnly = true
        } else if readOnly {
            // ERROR: Cannot make a read-only volume writable
            return VolumeMount{}, fmt.Errorf(
                "volume %q is read-only and cannot be mounted as read-write; "+
                "steps can only make volumes more restrictive, not less",
                ref.Name,
            )
        }
        // If ref.ReadOnly is false and volume is rw, readOnly stays false
    }
    
    return VolumeMount{
        HostPath:      vol.HostPath,
        ContainerPath: mountPath,
        ReadOnly:      readOnly,
    }, nil
}
```

### File: `pkg/runner/podman.go`

#### Update RunContainerOptions

```go
type RunContainerOptions struct {
    Name         string
    Hostname     string
    Network      string
    Image        string
    Cmd          string
    Args         []string
    Entrypoint   string
    Env          map[string]string
    WorkDir      string
    WorkflowDir  string
    
    // NEW: Additional volume mounts (for explicit host access)
    VolumeMounts []VolumeMount
    
    TTY          bool
    Remove       bool
    Background   bool
    HealthCheck  *HealthCheckConfig
    PortMappings []PortMapping
    Force        bool
}
```

#### Update RunContainer()

Modify the volume mounting logic to include explicit volumes:

```go
func (p *Podman) RunContainer(ctx context.Context, opts RunContainerOptions) error {
    args := []string{"run"}
    
    // ... existing flag setup ...
    
    // Mount workflow directory as /workflow (existing behavior)
    if opts.WorkflowDir != "" {
        absPath, err := filepath.Abs(opts.WorkflowDir)
        if err != nil {
            return fmt.Errorf("failed to get absolute path for workflow dir: %w", err)
        }
        args = append(args, "-v", fmt.Sprintf("%s:/workflow:rw", absPath))
    }
    
    // NEW: Mount explicit volumes
    for _, mount := range opts.VolumeMounts {
        mode := "rw"
        if mount.ReadOnly {
            mode = "ro"
        }
        args = append(args, "-v", fmt.Sprintf("%s:%s:%s", 
            mount.HostPath, 
            mount.ContainerPath, 
            mode))
    }
    
    // ... rest of method ...
}
```

---

## Usage Examples

### Example 1: Simple Build Output

```yaml
schemaVersion: "0.1.0"
name: Build and Output

volumes:
  dist:
    path: ./dist
    mode: rw  # Explicit read-write access
    mountPath: /output  # Custom default mount path

jobs:
  build:
    sequence:
      - name: Install dependencies
        image: node:20
        cmd: npm install
        
      - name: Build
        image: node:20
        cmd: npm run build
        
      - name: Copy artifacts
        image: node:20
        volumes:
          - dist  # Mounts at /output (volume's default mountPath)
        cmd: cp -r /workflow/build/* /output/
```

### Example 2: Read-Only Source Access

```yaml
schemaVersion: "0.1.0"
name: Build with Source

volumes:
  src:
    path: ./src
    # mode: ro (default - read-only)
  dist:
    path: ./dist
    mode: rw

jobs:
  build:
    volumes:
      - src
      - dist
    sequence:
      - name: Build
        image: gcc:latest
        cmd: gcc /volumes/src/*.c -o /volumes/dist/app
```

### Example 3: Custom Mount Paths

```yaml
schemaVersion: "0.1.0"
name: Custom Paths

volumes:
  credentials:
    path: ~/.aws
    mountPath: /root/.aws  # Default mount path
  src:
    path: ./src
    mountPath: /app/src

jobs:
  deploy:
    volumes:
      - credentials  # Mounts at /root/.aws (volume default)
      - name: src
        mountPath: /code  # Override: mount at /code instead of /app/src
    sequence:
      - name: Deploy
        image: aws-cli
        cmd: aws s3 sync /code s3://my-bucket
```

### Example 4: Step-Level ReadOnly Override

```yaml
schemaVersion: "0.1.0"
name: Selective Access

volumes:
  output:
    path: ./output
    mode: rw

jobs:
  process:
    sequence:
      - name: Write output
        image: alpine
        volumes:
          - output  # rw access
        cmd: echo "result" > /volumes/output/result.txt
        
      - name: Verify output (read-only)
        image: alpine
        volumes:
          - name: output
            readonly: true  # Mount the rw volume as read-only for this step
        cmd: cat /volumes/output/result.txt
```

### Example 5: Job-Level Volumes

```yaml
schemaVersion: "0.1.0"
name: Job Volumes

volumes:
  cache:
    path: ~/.cache/build
    mode: rw
    mountPath: /cache

jobs:
  build:
    volumes:
      - cache  # All steps in this job get /cache
    sequence:
      - name: Step 1
        image: build-tool
        cmd: build --cache /cache
        
      - name: Step 2
        image: build-tool
        cmd: test --cache /cache
```

---

## Volume Schema Summary

### Volume Definition (top-level `volumes`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | (required) | Host directory path (relative to workflow file or absolute) |
| `mode` | `"ro"` \| `"rw"` | `"ro"` | Access mode. Read-only by default; use `"rw"` for write access |
| `mountPath` | string | `/volumes/<name>` | Default mount path inside containers |

### Volume Reference (in steps/jobs `volumes` field)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | (required) | Name of volume (must exist in top-level `volumes`) |
| `mountPath` | string | volume's `mountPath` | Override mount path for this specific mount |
| `readonly` | boolean | `false` | Force read-only mount. Can only be `true` to restrict an `rw` volume |

### Shorthand Syntax

```yaml
# These are equivalent:
volumes:
  - dist                    # String shorthand
  - name: dist              # Object form

# These are equivalent:
volumes: dist               # Single string
volumes: [dist]             # Array with one element
volumes:
  - name: dist              # Full object form
```

### Mount Path Resolution Order

1. VolumeRef `mountPath` (if specified)
2. Volume `mountPath` (if specified)  
3. `/volumes/<volume-name>` (default)

### Mode Resolution Rules

| Volume Mode | VolumeRef `readonly` | Effective Mode | Valid? |
|-------------|---------------------|----------------|--------|
| `ro` | not set | read-only | Yes |
| `ro` | `true` | read-only | Yes |
| `ro` | `false` | - | **Error** |
| `rw` | not set | read-write | Yes |
| `rw` | `true` | read-only | Yes |
| `rw` | `false` | read-write | Yes |

**Key principle**: Steps can only make volumes MORE restrictive, never less.

---

## Implementation Order

### Phase 1: Schema Changes
1. Create `pkg/schema/volumes.go` with `Volume`, `Volumes`, `VolumeRef`, and `VolumeRefs` types
2. Add `Volumes` field to `OCW` struct in `schema.go`
3. Add `Volumes` field to `RunStep` and `BuildStep` in `steps.go`
4. Add `Volumes` field to `Job` in `jobs.go`
5. Update JSON Schema generation in `jsonschema.go`
6. Add validation for volume references in `validate.go`
7. Write tests for schema parsing with volumes

### Phase 2: Runner Integration
1. Add `resolvedVolumes` to `Runner` struct
2. Implement `resolveVolumes()` in `Run()`
3. Implement `resolveStepVolumes()` and `resolveVolumeRef()`
4. Update `RunContainerOptions` with `VolumeMounts`
5. Update `RunContainer()` to mount explicit volumes
6. Update `BuildImage()` similarly for build steps
7. Write integration tests

### Phase 3: Polish
1. Error messages for missing volumes
2. Warning for volumes that shadow `/workflow` paths
3. Documentation for volume feature
4. Update `schema.json`
5. Example workflows

---

## Testing Strategy

### Unit Tests
- Volume schema parsing (all shorthand variations)
- Volume reference resolution
- Mount path generation (all priority levels)
- Mode restriction validation (ro cannot become rw)

### Integration Tests
- Workflow with read-only volume
- Workflow with read-write volume
- Job-level volume inheritance
- Step-level volume override
- Custom mount paths
- Volume path validation (non-existent paths)

---

## File Summary

| File | Action | Description |
|------|--------|-------------|
| `pkg/schema/volumes.go` | Create | Volume types and parsing |
| `pkg/schema/schema.go` | Modify | Add Volumes field to OCW |
| `pkg/schema/steps.go` | Modify | Add Volumes field to RunStep, BuildStep |
| `pkg/schema/jobs.go` | Modify | Add Volumes field to Job |
| `pkg/schema/validate.go` | Modify | Add volume validation |
| `pkg/schema/jsonschema.go` | Modify | Update JSON Schema generation |
| `pkg/runner/runner.go` | Modify | Add volume resolution |
| `pkg/runner/podman.go` | Modify | Volume mount handling |

---

## Open Questions

1. **Workflow-level volume grants**: Should there be a way to grant all steps in a workflow access to a volume without repeating it in each job?

2. **Nested workflows**: How should volumes propagate to nested workflows executed via `workflow` step type?

3. **Volume path overlap**: What happens if a volume path overlaps with the workflow directory? Should we warn or error?
