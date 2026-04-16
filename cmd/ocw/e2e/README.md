# OCW CLI End-to-End Tests

This directory contains end-to-end (E2E) tests for the OCW CLI that verify complete functionality by running the CLI as a subprocess against test fixtures.

## Structure

```
e2e/
├── README.md              # This file
├── e2e_test.go           # TestMain, helpers, and TestCase struct
├── basic_test.go         # Basic workflow execution tests
├── jobs_test.go          # Job execution tests
├── switch_test.go        # Switch/conditional tests
├── build_test.go         # Docker build tests
├── nested_test.go        # Nested workflow tests
├── env_test.go           # Environment variable and secrets tests
├── templates_test.go     # Template expression tests
├── outputs_test.go       # Step outputs tests
├── background_test.go    # Background service tests
├── errors_test.go        # Validation and error handling tests
├── cli_test.go           # CLI flag tests
└── testdata/             # Test fixtures organized by category
    ├── basic/
    ├── jobs/
    ├── switch/
    ├── build/
    ├── nested/
    ├── env/
    ├── templates/
    ├── outputs/
    ├── background/
    └── errors/
```

## Running Tests

### Run all E2E tests
```bash
go test ./cmd/ocw/e2e/...
```

### Run from the e2e directory
```bash
cd cmd/ocw/e2e
go test
```

### Run specific test categories
```bash
go test ./cmd/ocw/e2e/... -run TestE2E_Basic
go test ./cmd/ocw/e2e/... -run TestE2E_Jobs
go test ./cmd/ocw/e2e/... -run TestE2E_Background
```

### Run with verbose output
```bash
go test ./cmd/ocw/e2e/... -v
```

### Run multiple times to check stability
```bash
go test ./cmd/ocw/e2e/... -count=3
```

## Test Coverage

The E2E test suite covers:

- **Basic Execution**: Hello world, sequences, parallel execution
- **Jobs**: Job execution, job discovery, job listing
- **Switch Statements**: Case matching, default cases, no-match scenarios
- **Build Steps**: Simple builds, build and run, image references
- **Nested Workflows**: Sequences containing parallel, and vice versa
- **Environment Variables**: Regular env vars, secrets (masked/unmasked)
- **Templates**: Workflow metadata, environment variable interpolation, step outputs
- **Step Outputs**: Output generation, output passing between steps, workflow outputs
- **Background Services**: Background containers, service dependencies
- **Validation & Errors**: Schema validation, YAML parsing, file not found
- **CLI Flags**: Version, help, verbose mode

## Requirements

- **Docker**: Most tests require Docker to be running
- **Timeout**: Default timeout is 60s per test, builds get 120s, background services get 10s

## How Tests Work

1. **TestMain** builds the OCW binary once before running tests
2. Each test case:
   - Creates a temporary directory
   - Copies test fixtures to the temp directory
   - Runs the OCW binary as a subprocess
   - Captures stdout and stderr
   - Validates exit code and output patterns
   - Cleans up the temp directory

## Adding New Tests

1. Create or update fixture files in `testdata/<category>/`
2. Add a test function in the appropriate `*_test.go` file:
   ```go
   func TestE2E_Category_TestName(t *testing.T) {
       t.Parallel()
       runTest(t, TestCase{
           Name:         "Descriptive Name",
           Fixture:      "category",
           Args:         []string{"-f", "workflow.yaml"},
           WantExitCode: 0,
           WantStdout:   []string{"expected output"},
           SkipNoDocker: true,
       })
   }
   ```

## TestCase Fields

- `Name`: Human-readable test name
- `Fixture`: Directory name in testdata/
- `Args`: CLI arguments
- `Env`: Environment variables to set
- `WantExitCode`: Expected exit code
- `WantStdout`: Patterns that must appear in stdout
- `WantStdoutNot`: Patterns that must NOT appear in stdout
- `WantStderr`: Patterns that must appear in stderr
- `WantStdoutOrdered`: Patterns that must appear in order
- `Timeout`: Test timeout (default: 60s)
- `SkipNoDocker`: Skip if Docker unavailable
- `InterruptAfter`: Send SIGINT after duration (for background services)
