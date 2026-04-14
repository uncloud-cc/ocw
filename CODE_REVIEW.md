# Code Review: OCW (Open Container Workflow)

This document contains actionable recommendations for improving the OCW codebase. Each task is
self-contained and can be implemented independently.

## Overview

OCW is a container-native workflow orchestration CLI tool written in Go 1.24.1. The codebase is
well-structured but has gaps in test coverage, some code duplication, and areas needing refactoring.

---

## Architecture Discussion: Package Structure

### Current Structure

```
ocw/
├── cmd/
│   ├── ocw/main.go           # CLI entry point (361 lines - too much logic)
│   └── schema-gen/main.go    # Schema generator (89 lines - appropriate)
└── pkg/
    ├── runner/               # Workflow execution engine
    └── schema/               # YAML schema definitions
```

### The Question

Which packages should be public (`pkg/`) vs internal (`internal/`)?

### Recommendation: Keep Current Structure, Extract CLI Logic

**Keep `pkg/schema` public** - This is clearly a public API. External tools and CI/CD integrations
will want to:
- Parse OCW workflow files
- Validate configurations
- Generate schemas
- Build tooling around the OCW format

**Keep `pkg/runner` public** - This enables the primary use case you mentioned: others building their
own OCW implementations or embedding OCW into CI/CD systems. The runner is the core value
proposition.

**Extract CLI helpers to `internal/cli`** - The `cmd/ocw/main.go` file contains ~200 lines of CLI
helper logic that is not useful to external consumers:
- `reorderArgs()` - flag reordering
- `discoverWorkflowFiles()` - file discovery
- `findJobInFiles()` - job lookup
- `listAvailableJobs()` - user-facing output
- `printWorkflowSummary()` - user-facing output

This logic is specific to the CLI UX and should be internal.

### Proposed Structure

```
ocw/
├── cmd/
│   ├── ocw/main.go           # Thin entry point (~50 lines)
│   └── schema-gen/main.go    # Keep as-is (already thin)
├── internal/
│   └── cli/
│       ├── discovery.go      # Workflow file discovery
│       ├── args.go           # Argument parsing helpers
│       └── output.go         # User-facing output formatting
└── pkg/
    ├── runner/               # Public: workflow execution
    └── schema/               # Public: schema parsing & validation
```

### What the Thin cmd/ocw/main.go Should Look Like

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/uncloud-cc/ocw/internal/cli"
)

var version = "dev"

func main() {
    if err := cli.Run(context.Background(), os.Args[1:], version); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

All the logic moves to `internal/cli`, which:
1. Parses flags and arguments
2. Discovers workflow files
3. Initializes the runner
4. Executes the workflow
5. Handles output formatting

This keeps `cmd/` as pure initialization while making the CLI logic testable (you can test
`cli.Run()` directly).

---

## Task 1: Add Linter Configuration

**Priority:** High  
**Effort:** 10 minutes  
**Files to create:** `.golangci.yml`

Create a linter configuration file at the project root:

```yaml
run:
  timeout: 5m
  go: "1.24"

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - gosec
    - ineffassign
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam

linters-settings:
  errcheck:
    check-type-assertions: true
  govet:
    enable-all: true
  goimports:
    local-prefixes: github.com/uncloud-cc/ocw

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

Then add a lint target to the Makefile:

```makefile
.PHONY: lint
lint:
	golangci-lint run ./...
```

---

## Task 2: Remove Unused Dependency

**Priority:** High  
**Effort:** 5 minutes  
**Files to modify:** `go.mod`

The `github.com/charmbracelet/lipgloss` dependency is declared but not used anywhere in the codebase.
The `pkg/runner/colors.go` file uses raw ANSI escape codes instead.

**Action:** Run the following command to clean up unused dependencies:

```bash
go mod tidy
```

If lipgloss remains after tidy, explicitly remove it and run tidy again.

---

## Task 3: Extract Duplicate sanitizeName Function

**Priority:** High  
**Effort:** 15 minutes  
**Files to modify:**
- `pkg/runner/runner.go` (lines 1352-1371)
- `pkg/runner/reloader.go` (lines 193-212)

**Problem:** Two nearly identical functions exist:
- `sanitizeName()` in `runner.go`
- `sanitizeNameForHostname()` in `reloader.go`

**Action:** Create a new file `pkg/runner/names.go` with a single exported function:

```go
package runner

import (
	"regexp"
	"strings"
)

var (
	// sanitizeRegex matches characters that are not allowed in container names
	sanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	// leadingInvalidRegex matches leading characters that are not alphanumeric
	leadingInvalidRegex = regexp.MustCompile(`^[^a-zA-Z0-9]+`)
)

// SanitizeName converts a string to a valid container/hostname name.
// It replaces invalid characters with hyphens and removes leading non-alphanumeric characters.
func SanitizeName(name string) string {
	// Replace invalid characters with hyphens
	sanitized := sanitizeRegex.ReplaceAllString(name, "-")
	// Remove leading non-alphanumeric characters
	sanitized = leadingInvalidRegex.ReplaceAllString(sanitized, "")
	// Collapse multiple hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	// Trim trailing hyphens
	sanitized = strings.TrimSuffix(sanitized, "-")
	// Ensure non-empty
	if sanitized == "" {
		sanitized = "container"
	}
	return sanitized
}
```

Then update both `runner.go` and `reloader.go` to use `SanitizeName()` and delete the duplicate
functions.

---

## Task 4: Add Constants for Magic Numbers

**Priority:** Medium  
**Effort:** 20 minutes  
**Files to modify:**
- `pkg/runner/watcher.go`
- `pkg/runner/docker.go`
- `pkg/runner/reloader.go`

**Problem:** Several magic numbers are scattered throughout the code.

**Action:** Add constants at the top of each file or create a `pkg/runner/constants.go`:

```go
package runner

import "time"

const (
	// DefaultDebounceInterval is the default debounce duration for file watcher events
	DefaultDebounceInterval = 100 * time.Millisecond

	// ContainerStopGracePeriod is the time to wait for a container to stop gracefully
	ContainerStopGracePeriod = 500 * time.Millisecond

	// HealthCheckInterval is the interval between container health checks
	HealthCheckInterval = 100 * time.Millisecond

	// MaxHealthCheckAttempts is the maximum number of health check attempts
	MaxHealthCheckAttempts = 300
)
```

Replace the following occurrences:
- `pkg/runner/watcher.go:48` - `100 * time.Millisecond` → `DefaultDebounceInterval`
- `pkg/runner/docker.go:457` - `500 * time.Millisecond` → `ContainerStopGracePeriod`
- `pkg/runner/reloader.go:136` - `100 * time.Millisecond` → `HealthCheckInterval`
- `pkg/runner/reloader.go:137` - `300` → `MaxHealthCheckAttempts`

---

## Task 5: Add Verbose Logging for Silent Errors

**Priority:** Medium  
**Effort:** 15 minutes  
**Files to modify:** `cmd/ocw/main.go`

**Problem:** At line 254-256, parsing errors are silently skipped:

```go
ocw, err := schema.ParseFile(file)
if err != nil {
    continue // Skip files that fail to parse
}
```

**Action:** Add verbose logging:

```go
ocw, err := schema.ParseFile(file)
if err != nil {
    if verbose {
        fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", file, err)
    }
    continue
}
```

This requires passing the `verbose` flag to the function or checking it from the command context.

---

## Task 6: Fix Silent Step Type Mismatch

**Priority:** High  
**Effort:** 10 minutes  
**Files to modify:** `pkg/schema/steps.go`

**Problem:** At line 874, when no step type is matched, the function returns `nil` instead of an
error:

```go
return nil // Returns nil when no step type matched
```

**Action:** Return an error when no step type is recognized:

```go
return fmt.Errorf("unrecognized step type: must have one of 'run', 'build', 'push', 'parallel', or 'workflow' field")
```

---

## Task 7: Fix Input Mutation in runRunStep

**Priority:** Medium  
**Effort:** 20 minutes  
**Files to modify:** `pkg/runner/runner.go`

**Problem:** At lines 853-861, the function mutates its input:

```go
if step.Watch != nil && step.Watch.IsEnabled() && !step.Background {
    step.Background = true  // MUTATES INPUT!
}
```

**Action:** Create a local copy of the relevant field instead of mutating the input:

```go
background := step.Background
if step.Watch != nil && step.Watch.IsEnabled() && !background {
    background = true
}
// Use 'background' variable instead of 'step.Background' throughout the function
```

---

## Task 8: Add Tests for Template Interpolation

**Priority:** High  
**Effort:** 1-2 hours  
**Files to create:** `pkg/runner/template_test.go`

**Problem:** The template interpolation logic (`{{ steps.x.y }}` syntax) has no tests.

**Action:** Create comprehensive tests:

```go
package runner

import (
	"testing"
)

func TestInterpolateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "Hello {{ name }}",
			data:     map[string]interface{}{"name": "World"},
			want:     "Hello World",
		},
		{
			name:     "nested variable",
			template: "{{ steps.build.outputs.image }}",
			data: map[string]interface{}{
				"steps": map[string]interface{}{
					"build": map[string]interface{}{
						"outputs": map[string]interface{}{
							"image": "myapp:latest",
						},
					},
				},
			},
			want: "myapp:latest",
		},
		{
			name:     "missing variable",
			template: "{{ missing }}",
			data:     map[string]interface{}{},
			want:     "",
		},
		{
			name:     "no interpolation needed",
			template: "plain string",
			data:     map[string]interface{}{},
			want:     "plain string",
		},
		// Add more test cases for edge cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the interpolation function and verify results
			// Adjust based on actual function signature
		})
	}
}
```

---

## Task 9: Add Tests for Validation Logic

**Priority:** High  
**Effort:** 2-3 hours  
**Files to create:** `pkg/schema/validate_test.go`

**Problem:** The validation logic in `validate.go` lacks comprehensive tests.

**Action:** Create tests covering:

1. Valid workflow configurations
2. Missing required fields
3. Invalid field combinations
4. Circular dependencies
5. Invalid step references
6. Volume validation
7. Environment variable validation

Example structure:

```go
package schema

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid minimal workflow",
			yaml: `
jobs:
  build:
    steps:
      - run: echo hello
`,
			wantErr: false,
		},
		{
			name: "missing jobs",
			yaml: `
name: test
`,
			wantErr:   true,
			errSubstr: "jobs",
		},
		{
			name: "invalid step reference",
			yaml: `
jobs:
  build:
    steps:
      - run: echo {{ steps.nonexistent.output }}
`,
			wantErr:   true,
			errSubstr: "nonexistent",
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Parse() error = %v", err)
				}
				return
			}

			err = ocw.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errSubstr)
			}
		})
	}
}
```

---

## Task 10: Split runner.go Into Smaller Files

**Priority:** Medium  
**Effort:** 2-3 hours  
**Files to modify:** `pkg/runner/runner.go`  
**Files to create:**
- `pkg/runner/execution.go`
- `pkg/runner/volumes.go`
- `pkg/runner/cleanup.go`

**Problem:** `runner.go` is 1541 lines and contains multiple concerns.

**Action:** Split into focused files:

### `pkg/runner/runner.go` (keep ~400 lines)
- `Runner` struct definition
- `NewRunner()` constructor
- `With*()` builder methods
- `Run()` entry point
- High-level orchestration

### `pkg/runner/execution.go` (new file, ~600 lines)
- `runStep()`
- `runRunStep()`
- `runBuildStep()`
- `runPushStep()`
- `runWorkflowStep()`
- `runParallel()`

### `pkg/runner/volumes.go` (new file, ~200 lines)
- Volume preparation logic
- `prepareVolumes()`
- Volume path resolution

### `pkg/runner/cleanup.go` (new file, ~200 lines)
- `Cleanup()`
- `cleanupContainers()`
- `cleanupNetworks()`
- `cleanupVolumes()`
- Signal handling

### `pkg/runner/services.go` (new file, ~150 lines)
- `ExposedService` struct
- `registerExposedService()`
- `printExposedServices()`
- Port tracking

---

## Task 11: Split steps.go Into Smaller Files

**Priority:** Medium  
**Effort:** 1-2 hours  
**Files to modify:** `pkg/schema/steps.go`  
**Files to create:**
- `pkg/schema/run_step.go`
- `pkg/schema/build_step.go`
- `pkg/schema/watch.go`

**Problem:** `steps.go` is 898 lines with multiple step type definitions.

**Action:** Split by step type:

### `pkg/schema/steps.go` (keep ~200 lines)
- `Step` interface/struct
- `UnmarshalYAML` for Step
- Common step utilities

### `pkg/schema/run_step.go` (new file, ~300 lines)
- `RunStep` struct
- `RunStepOptions` struct
- Related types and methods

### `pkg/schema/build_step.go` (new file, ~200 lines)
- `BuildStep` struct
- `PushStep` struct
- Build-related types

### `pkg/schema/watch.go` (new file, ~150 lines)
- `Watch` struct
- `WatchConfig` struct
- Watch-related methods

---

## Task 12: Implement Workflow Step

**Priority:** Medium  
**Effort:** 4-6 hours  
**Files to modify:** `pkg/runner/runner.go` (lines 1308-1320)

**Problem:** The workflow step is advertised but not implemented:

```go
func (r *Runner) runWorkflowStep(ctx context.Context, step *schema.WorkflowStep) error {
    // ...
    r.Output("  %s\n", r.styles.Warning("Warning: workflow invocation not yet implemented"))
    return nil
}
```

**Action:** Implement workflow step execution:

1. Parse the workflow reference (local file path or remote URL)
2. Load and parse the referenced workflow file
3. Create a sub-runner with appropriate context
4. Execute the referenced workflow
5. Propagate outputs back to the parent workflow

```go
func (r *Runner) runWorkflowStep(ctx context.Context, step *schema.WorkflowStep) error {
    r.Output("%s Running workflow: %s\n", r.styles.StepIcon(), step.ID)

    // Resolve workflow path
    workflowPath := step.Workflow
    if !filepath.IsAbs(workflowPath) {
        workflowPath = filepath.Join(r.WorkflowDir, workflowPath)
    }

    // Parse the referenced workflow
    refWorkflow, err := schema.ParseFile(workflowPath)
    if err != nil {
        return fmt.Errorf("failed to parse workflow %s: %w", step.Workflow, err)
    }

    // Create sub-runner
    subRunner := NewRunner(filepath.Dir(workflowPath)).
        WithVerbose(r.verbose).
        WithDryRun(r.dryRun)

    // Pass inputs if specified
    if step.With != nil {
        // Set inputs as environment variables or context
    }

    // Execute
    if err := subRunner.Run(ctx, refWorkflow); err != nil {
        return fmt.Errorf("workflow %s failed: %w", step.Workflow, err)
    }

    return nil
}
```

---

## Task 13: Add Makefile Targets

**Priority:** Medium  
**Effort:** 15 minutes  
**Files to modify:** `Makefile`

**Action:** Add the following targets:

```makefile
.PHONY: fmt
fmt:
	goimports -w .
	gofmt -w .

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: coverage
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check: fmt vet lint test
	@echo "All checks passed"
```

---

## Task 14: Extract CLI Logic to internal/cli

**Priority:** Medium  
**Effort:** 1-2 hours  
**Files to modify:** `cmd/ocw/main.go`  
**Files to create:**
- `internal/cli/cli.go`
- `internal/cli/discovery.go`
- `internal/cli/output.go`

**Problem:** `cmd/ocw/main.go` contains 361 lines of mixed concerns - flag parsing, file discovery,
job lookup, output formatting, and runner initialization. This makes it hard to test and violates
the principle of keeping cmd/ as thin entry points.

**Action:** Extract CLI logic into `internal/cli`:

### `internal/cli/cli.go` (~150 lines)

```go
package cli

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "strings"
    "syscall"

    "github.com/uncloud-cc/ocw/pkg/runner"
    "github.com/uncloud-cc/ocw/pkg/schema"
)

// Config holds CLI configuration parsed from flags
type Config struct {
    ValidateOnly bool
    WorkflowFile string
    EnvFile      string
    ShowSecrets  bool
    Force        bool
    Verbose      bool
    JobName      string
}

// Run executes the CLI with the given arguments
func Run(ctx context.Context, args []string, version string) error {
    cfg, err := ParseArgs(args, version)
    if err != nil {
        return err
    }
    if cfg == nil {
        return nil // --help or --version was shown
    }

    return Execute(ctx, cfg)
}

// Execute runs the workflow based on the parsed configuration
func Execute(ctx context.Context, cfg *Config) error {
    // Resolve workflow file
    workflowPath, err := ResolveWorkflowFile(cfg)
    if err != nil {
        return err
    }

    // Parse and validate
    ocw, err := schema.ParseFile(workflowPath)
    if err != nil {
        return fmt.Errorf("failed to parse workflow: %w", err)
    }
    if err := ocw.Validate(); err != nil {
        return fmt.Errorf("workflow validation failed:\n%w", err)
    }

    if cfg.ValidateOnly {
        PrintWorkflowSummary(ocw)
        return nil
    }

    // Set up cancellation
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        fmt.Println("\nReceived interrupt signal, cancelling...")
        cancel()
    }()

    // Initialize and run
    workflowDir := filepath.Dir(workflowPath)
    r := runner.NewRunner(workflowDir).
        WithVerbose(cfg.Verbose).
        WithShowSecrets(cfg.ShowSecrets).
        WithForce(cfg.Force)

    if cfg.EnvFile != "" {
        r.WithEnvFile(cfg.EnvFile)
    }

    if cfg.JobName != "" {
        return r.RunJob(ctx, ocw, cfg.JobName)
    }

    if !ocw.HasDirectFlow() {
        PrintAvailableJobs(ocw)
        return fmt.Errorf("specify a job name to run")
    }

    return r.Run(ctx, ocw)
}
```

### `internal/cli/discovery.go` (~100 lines)

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/uncloud-cc/ocw/pkg/schema"
)

// DiscoverWorkflowFiles finds all YAML files in a directory
func DiscoverWorkflowFiles(dir string) ([]string, error) {
    // ... move discoverWorkflowFiles logic here
}

// FindJobInFiles searches for a job name across multiple workflow files
func FindJobInFiles(files []string, jobName string, verbose bool) (string, error) {
    // ... move findJobInFiles logic here
    // Add verbose parameter for logging parse errors
}

// ResolveWorkflowFile determines the workflow file to use based on config
func ResolveWorkflowFile(cfg *Config) (string, error) {
    // ... consolidate workflow resolution logic here
}

// IsYAMLFile checks if a path looks like a YAML file
func IsYAMLFile(path string) bool {
    lower := strings.ToLower(path)
    return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}
```

### `internal/cli/output.go` (~80 lines)

```go
package cli

import (
    "fmt"

    "github.com/uncloud-cc/ocw/pkg/schema"
)

// PrintWorkflowSummary displays workflow information after validation
func PrintWorkflowSummary(ocw *schema.OCW) {
    // ... move printWorkflowSummary logic here
}

// PrintAvailableJobs lists jobs from the workflow
func PrintAvailableJobs(ocw *schema.OCW) {
    // ... move job listing logic here
}

// ListAvailableJobsFromFiles lists jobs from multiple workflow files
func ListAvailableJobsFromFiles(files []string) error {
    // ... move listAvailableJobs logic here
}
```

### Updated `cmd/ocw/main.go` (~20 lines)

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/uncloud-cc/ocw/internal/cli"
)

var version = "dev"

func main() {
    if err := cli.Run(context.Background(), os.Args[1:], version); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Benefits:**
1. `cmd/ocw/main.go` becomes a trivial entry point
2. CLI logic is testable via `cli.Run()` and `cli.Execute()`
3. Clear separation of concerns
4. Internal implementation details are hidden from external consumers

---

## Task 15: Add Package Documentation

**Priority:** Low  
**Effort:** 1 hour  
**Files to create:**
- `pkg/runner/doc.go`
- `pkg/schema/doc.go`

**Action:** Create doc.go files:

### `pkg/runner/doc.go`

```go
// Package runner provides the execution engine for OCW workflows.
//
// The runner package handles:
//   - Workflow execution orchestration
//   - Docker container management
//   - Volume and network setup
//   - File watching and hot reload
//   - Parallel step execution
//
// Basic usage:
//
//	r := runner.NewRunner("/path/to/workflow").
//	    WithVerbose(true)
//	err := r.Run(ctx, workflow)
package runner
```

### `pkg/schema/doc.go`

```go
// Package schema defines the OCW workflow configuration types.
//
// The schema package provides:
//   - YAML parsing for OCW workflow files
//   - Validation of workflow configurations
//   - JSON Schema generation for IDE support
//
// Basic usage:
//
//	ocw, err := schema.ParseFile("workflow.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := ocw.Validate(); err != nil {
//	    log.Fatal(err)
//	}
package schema
```

---

## Task 16: Add Integration Tests

**Priority:** Medium  
**Effort:** 4-6 hours  
**Files to create:** `pkg/runner/runner_integration_test.go`

**Problem:** No integration tests exist for the Docker interactions.

**Action:** Create integration tests that run actual containers:

```go
//go:build integration

package runner

import (
	"context"
	"testing"
	"time"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestRunner_SimpleWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	yaml := `
jobs:
  test:
    steps:
      - id: hello
        run: echo "Hello, World!"
`
	ocw, err := schema.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	r := NewRunner(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.Run(ctx, ocw); err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestRunner_BuildStep(t *testing.T) {
	// Test building a Docker image
}

func TestRunner_ParallelSteps(t *testing.T) {
	// Test parallel execution
}

func TestRunner_VolumeMount(t *testing.T) {
	// Test volume mounting
}
```

Add to Makefile:

```makefile
.PHONY: integration-test
integration-test:
	go test -race -tags=integration ./...
```

---

## Task 17: Document Context Usage in Cleanup

**Priority:** Low  
**Effort:** 10 minutes  
**Files to modify:** `pkg/runner/docker.go`

**Problem:** At lines 450-451, cleanup uses `context.Background()` instead of the parent context,
which is intentional but undocumented:

```go
d.StopContainer(context.Background(), containerName)
d.RemoveContainer(context.Background(), containerName)
```

**Action:** Add a comment explaining the design decision:

```go
// Use context.Background() for cleanup operations to ensure they complete
// even if the parent context is cancelled. This prevents orphaned containers
// when the workflow is interrupted.
d.StopContainer(context.Background(), containerName)
d.RemoveContainer(context.Background(), containerName)
```

---

## Task 18: Optimize YAML Unmarshaling

**Priority:** Low  
**Effort:** 2-3 hours  
**Files to modify:** `pkg/schema/steps.go`

**Problem:** The `UnmarshalYAML` method parses YAML twice (once to probe, once to unmarshal):

```go
var probe map[string]interface{}
if err := unmarshal(&probe); err != nil {
    return err
}
// Then unmarshal again...
```

**Action:** Use a single unmarshal with a union type:

```go
func (s *Step) UnmarshalYAML(unmarshal func(interface{}) error) error {
    // Define a union type that can hold any step type
    type stepUnion struct {
        // Common fields
        ID   string `yaml:"id"`
        Name string `yaml:"name"`
        If   string `yaml:"if"`

        // Discriminating fields
        Run      interface{} `yaml:"run"`
        Build    interface{} `yaml:"build"`
        Push     interface{} `yaml:"push"`
        Parallel interface{} `yaml:"parallel"`
        Workflow interface{} `yaml:"workflow"`
    }

    var union stepUnion
    if err := unmarshal(&union); err != nil {
        return err
    }

    // Determine step type based on which field is set
    switch {
    case union.Run != nil:
        s.RunStep = &RunStep{}
        return unmarshal(s.RunStep)
    case union.Build != nil:
        s.BuildStep = &BuildStep{}
        return unmarshal(s.BuildStep)
    // ... etc
    }
}
```

This is more complex but avoids double parsing.

---

## Summary

| Task | Priority | Effort | Impact |
|------|----------|--------|--------|
| 1. Add linter config | High | 10 min | Catches bugs early |
| 2. Remove unused dep | High | 5 min | Clean dependencies |
| 3. Extract duplicate function | High | 15 min | DRY principle |
| 4. Add constants | Medium | 20 min | Maintainability |
| 5. Add verbose logging | Medium | 15 min | Debuggability |
| 6. Fix silent step mismatch | High | 10 min | Error handling |
| 7. Fix input mutation | Medium | 20 min | Bug prevention |
| 8. Add template tests | High | 1-2 hr | Test coverage |
| 9. Add validation tests | High | 2-3 hr | Test coverage |
| 10. Split runner.go | Medium | 2-3 hr | Maintainability |
| 11. Split steps.go | Medium | 1-2 hr | Maintainability |
| 12. Implement workflow step | Medium | 4-6 hr | Feature completion |
| 13. Add Makefile targets | Medium | 15 min | Developer experience |
| 14. Extract CLI to internal | Medium | 1-2 hr | Code organization |
| 15. Add package docs | Low | 1 hr | Documentation |
| 16. Add integration tests | Medium | 4-6 hr | Test coverage |
| 17. Document context usage | Low | 10 min | Code clarity |
| 18. Optimize YAML parsing | Low | 2-3 hr | Performance |

**Recommended order:** 1, 2, 6, 3, 4, 5, 7, 13, 14, 8, 9, 10, 11, 12, 15, 16, 17, 18
