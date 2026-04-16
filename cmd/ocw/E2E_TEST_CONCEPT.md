# OCW CLI E2E Test Concept

## Goal

Create straightforward end-to-end tests for the `ocw` CLI that verify the entirety of supported features by running the CLI against fixture workflow files and asserting on console output.

## Location

- Test file: `cmd/ocw/main_test.go`
- Fixtures: `cmd/ocw/testdata/` (workflow YAML files organized by feature)

## Basic Approach

### 1. Test Structure

Use Go's standard `testing` package with table-driven tests. Each test case:

1. Specifies a fixture workflow file
2. Provides CLI arguments (flags, job name)
3. Optionally sets environment variables
4. Runs the `ocw` CLI as a subprocess
5. Asserts on:
   - Exit code (0 for success, 1 for error)
   - Stdout contains expected patterns
   - Stderr contains expected patterns (for errors)

### 2. Test Execution Helper

```go
type TestCase struct {
    Name           string            // Test name
    Fixture        string            // Path to fixture file relative to testdata/
    Args           []string          // CLI args (e.g., []string{"build"} for job name)
    Env            map[string]string // Environment variables to set
    WantExitCode   int               // Expected exit code
    WantStdout     []string          // Patterns that must appear in stdout
    WantStdoutNot  []string          // Patterns that must NOT appear in stdout
    WantStderr     []string          // Patterns that must appear in stderr
}
```

The helper function:
1. Builds the CLI binary once per test run using `go build`
2. Creates a temp directory, copies the fixture file
3. Runs the CLI with `exec.Command`
4. Captures stdout and stderr
5. Verifies exit code and output patterns

### 3. Pattern Matching

Use simple substring matching for assertions (not regex). This keeps tests readable and maintainable:

```go
// Good: Simple substring checks
WantStdout: []string{"Hello OCW World", "Step completed"}

// Avoid: Complex regex unless truly necessary
```

For cases where order matters, tests can use indexed patterns:
```go
WantStdoutOrdered: []string{"Step 1", "Step 2", "Step 3"}
```

---

## Test Categories and Cases

### Category 1: Basic Workflow Execution

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestHelloWorld` | `hello_world.yaml` | Simple sequence with echo |
| `TestSequence` | `sequence.yaml` | Multiple sequential steps |
| `TestParallel` | `parallel.yaml` | Concurrent step execution |

**Assertions:**
- Exit code 0
- Output from each step appears
- For parallel: all outputs present (order may vary)
- For sequence: outputs in order

### Category 2: Build Steps

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestBuildSimple` | `build_simple.yaml` | Basic Dockerfile build |
| `TestBuildAndRun` | `build_and_run.yaml` | Build then run with `{{ steps.X.image }}` |
| `TestBuildContext` | `build_context.yaml` | Custom build context directory |
| `TestBuildArgs` | `build_args.yaml` | Build with build arguments |

**Assertions:**
- Build output appears ("Building image...")
- Image name appears in output
- Run step uses built image

### Category 3: Jobs

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestJobExecution` | `jobs.yaml` | Run specific job by name |
| `TestJobNotFound` | `jobs.yaml` | Error when job doesn't exist |
| `TestListJobs` | `jobs.yaml` | No job arg lists available jobs |

**Assertions:**
- Correct job runs
- Other jobs don't run
- Job listing shows all job names

### Category 4: Flow Control - Switch

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestSwitchCase` | `switch.yaml` | Match specific case |
| `TestSwitchDefault` | `switch.yaml` | Default case when no match |
| `TestSwitchNoMatch` | `switch_no_default.yaml` | No match, no default |

**Assertions:**
- Correct branch executes based on env var
- Non-matching branches don't execute

### Category 5: Nested Workflows

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestNestedSequenceParallel` | `nested.yaml` | Sequence containing parallel |
| `TestNestedParallelSequence` | `nested_parallel_seq.yaml` | Parallel containing sequences |
| `TestDeeplyNested` | `deeply_nested.yaml` | 3+ levels of nesting |

**Assertions:**
- All nested steps execute
- Correct execution order for sequences
- Parallel steps all complete

### Category 6: Environment & Secrets

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestEnvVariables` | `env.yaml` | Workflow-level env vars |
| `TestEnvInheritance` | `env_inherit.yaml` | Env inheritance to steps |
| `TestSecretsMasked` | `secrets.yaml` | Secrets appear as masked |
| `TestSecretsShown` | `secrets.yaml` | `--show-secrets` unmasks values |
| `TestEnvFile` | `env_file.yaml` | Load from env file with `-e` |

**Assertions:**
- Env values appear in container output
- Secrets show as `***` by default
- `--show-secrets` shows actual values
- Env file values used

### Category 7: Template Expressions

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestTemplateWorkflowName` | `template_workflow.yaml` | `{{ workflow.name }}` |
| `TestTemplateEnv` | `template_env.yaml` | `{{ env.VAR }}` |
| `TestTemplateStepOutput` | `template_output.yaml` | `{{ steps.X.output }}` |

**Assertions:**
- Template expressions resolved in output
- Correct values substituted

### Category 8: Outputs

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestStepOutputs` | `outputs.yaml` | Step writes to $OUTPUTS |
| `TestOutputPassing` | `output_passing.yaml` | Output from one step used in another |
| `TestWorkflowOutputs` | `workflow_outputs.yaml` | Workflow-level outputs |

**Assertions:**
- Output values captured correctly
- Template references resolve to output values

### Category 9: Background Services & Networking

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestBackgroundService` | `background.yaml` | Service runs in background |
| `TestHealthCheck` | `healthcheck.yaml` | Health check before continuing |
| `TestNeeds` | `needs.yaml` | Step waits for service |
| `TestExpose` | `expose.yaml` | Port exposed on host |
| `TestContainerNetworking` | `networking.yaml` | Container-to-container by hostname |

**Assertions:**
- Service starts and stays running
- Health check passes before dependent step
- Network connectivity between containers

### Category 10: Volumes

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestVolumeReadonly` | `volume_readonly.yaml` | Read-only volume access |
| `TestVolumeReadwrite` | `volume_readwrite.yaml` | Read-write volume access |
| `TestVolumeCustomMount` | `volume_mount.yaml` | Custom mount path |

**Assertions:**
- Files accessible in container
- Write operations work for readwrite
- Write operations fail for readonly

### Category 11: Watch Mode

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestWatchTriggersReload` | `watch.yaml` | File change triggers reload |

**Note:** Watch tests are more complex - they require:
1. Start workflow
2. Modify a watched file
3. Observe reload in output
4. Kill workflow

Consider keeping watch tests minimal or separate.

### Category 12: Validation & Errors

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestValidateOnly` | `valid.yaml` | `--validate` exits 0, doesn't run |
| `TestValidateInvalid` | `invalid_schema.yaml` | Invalid schema fails validation |
| `TestMissingImage` | `missing_image.yaml` | Missing required field error |
| `TestInvalidYAML` | `invalid_yaml.yaml` | YAML parse error |
| `TestFileNotFound` | (none) | Non-existent file error |
| `TestMultipleFlowControls` | `multiple_flows.yaml` | Can't have parallel + sequence |

**Assertions:**
- Exit code 1 for errors
- Error message is descriptive
- Validation shows specific field errors

### Category 13: CLI Flags & Behavior

| Test Case | Fixture | Description |
|-----------|---------|-------------|
| `TestVersionFlag` | (none) | `--version` shows version |
| `TestHelpFlag` | (none) | `--help` shows usage |
| `TestVerboseOutput` | `hello.yaml` | `--verbose` shows internal steps |
| `TestForceFlag` | `background.yaml` | `--force` removes existing containers |
| `TestWorkflowFileFlag` | `hello.yaml` | `-f` specifies workflow file |

---

## Fixture File Organization

```
cmd/ocw/testdata/
  basic/
    hello_world.yaml
    sequence.yaml
    parallel.yaml
  build/
    build_simple.yaml
    build_simple/Dockerfile
    build_and_run.yaml
    build_context.yaml
    build_context/app/Dockerfile
  jobs/
    jobs.yaml
    jobs_single.yaml
  switch/
    switch.yaml
    switch_no_default.yaml
  nested/
    nested.yaml
    deeply_nested.yaml
  env/
    env.yaml
    env_inherit.yaml
    secrets.yaml
    env_file.yaml
    test.env
  templates/
    template_workflow.yaml
    template_env.yaml
    template_output.yaml
  outputs/
    outputs.yaml
    output_passing.yaml
  background/
    background.yaml
    healthcheck.yaml
    needs.yaml
    expose.yaml
    networking.yaml
  volumes/
    volume_readonly.yaml
    volume_readwrite.yaml
    testfile.txt
  watch/
    watch.yaml
    src/index.js
  errors/
    invalid_schema.yaml
    invalid_yaml.yaml
    missing_image.yaml
    multiple_flows.yaml
```

---

## Implementation Notes

### Binary Building

Build the binary once at the start of test execution:

```go
var binaryPath string

func TestMain(m *testing.M) {
    // Build binary to temp location
    tmpDir, _ := os.MkdirTemp("", "ocw-test")
    binaryPath = filepath.Join(tmpDir, "ocw")
    
    cmd := exec.Command("go", "build", "-o", binaryPath, ".")
    if err := cmd.Run(); err != nil {
        log.Fatal(err)
    }
    
    code := m.Run()
    os.RemoveAll(tmpDir)
    os.Exit(code)
}
```

### Test Isolation

Each test runs in its own temp directory:

```go
func runTest(t *testing.T, tc TestCase) {
    t.Helper()
    
    // Create temp dir
    tmpDir, err := os.MkdirTemp("", "ocw-e2e-*")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)
    
    // Copy fixture files
    copyFixture(t, tc.Fixture, tmpDir)
    
    // Build args
    args := []string{"-f", "workflow.yaml"}
    args = append(args, tc.Args...)
    
    // Run CLI
    cmd := exec.Command(binaryPath, args...)
    cmd.Dir = tmpDir
    cmd.Env = buildEnv(tc.Env)
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    err = cmd.Run()
    exitCode := 0
    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    }
    
    // Assert
    assert.Equal(t, tc.WantExitCode, exitCode)
    for _, pattern := range tc.WantStdout {
        assert.Contains(t, stdout.String(), pattern)
    }
    for _, pattern := range tc.WantStderr {
        assert.Contains(t, stderr.String(), pattern)
    }
}
```

### Docker Requirements

Tests require Docker to be available. Skip tests gracefully if Docker is not running:

```go
func init() {
    cmd := exec.Command("docker", "info")
    if err := cmd.Run(); err != nil {
        // Set flag to skip docker-dependent tests
        dockerAvailable = false
    }
}

func skipIfNoDocker(t *testing.T) {
    if !dockerAvailable {
        t.Skip("Docker not available")
    }
}
```

### Timeout Handling

Set reasonable timeouts for tests (builds and network operations can be slow):

```go
func runWithTimeout(t *testing.T, tc TestCase, timeout time.Duration) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, binaryPath, tc.Args...)
    // ...
}
```

Default timeout: 60 seconds per test
Build tests: 120 seconds

### Parallel Tests

Use `t.Parallel()` for independent tests, but be mindful of:
- Docker resource limits
- Port conflicts for expose tests (use dynamic ports or unique ports per test)

---

## Minimum Viable Test Suite

For initial implementation, focus on these core tests (roughly 20 tests):

1. **Basic execution:** hello_world, sequence, parallel
2. **Jobs:** run job, list jobs, job not found
3. **Switch:** case match, default
4. **Build:** simple build, build and run
5. **Nested:** sequence in parallel
6. **Env:** env vars, secrets masked, `--show-secrets`
7. **Templates:** workflow.name, step outputs
8. **Outputs:** step outputs, passing between steps
9. **Background:** simple background service, needs
10. **Validation:** `--validate`, invalid schema

This covers the core functionality. Expand to full coverage iteratively.

---

## Test Naming Convention

```go
func TestE2E_Basic_HelloWorld(t *testing.T) { ... }
func TestE2E_Jobs_RunSpecificJob(t *testing.T) { ... }
func TestE2E_Switch_DefaultCase(t *testing.T) { ... }
func TestE2E_Validation_InvalidSchema(t *testing.T) { ... }
```

Pattern: `TestE2E_<Category>_<Scenario>`

---

## Running Tests

```bash
# Run all e2e tests
go test ./cmd/ocw/... -v

# Run specific category
go test ./cmd/ocw/... -v -run "TestE2E_Jobs"

# Run with race detector
go test ./cmd/ocw/... -v -race

# Skip docker tests (if needed)
SKIP_DOCKER_TESTS=1 go test ./cmd/ocw/... -v
```
