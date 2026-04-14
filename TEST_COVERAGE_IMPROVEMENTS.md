# Test Coverage Improvements for OCW

## Summary

This document summarizes the integration tests added to improve test coverage for the `pkg/runner` package, with a focus on Docker CLI interactions that were previously untested.

## Coverage Improvements

### Before
- **pkg/runner**: 20.4% coverage
- **pkg/schema**: 68.1% coverage
- **128 completely untested functions** in pkg/runner
- **58 completely untested functions** in pkg/schema

### After
- **pkg/runner**: 29.1% coverage (+8.7 percentage points)
- Core Docker operations now have test coverage:
  - `CreateNetwork`: 75.0% (was 0%)
  - `RemoveNetwork`: 100% (was 0%)
  - `NetworkExists`: 100% (was 0%)
  - `PullImage`: 16.0% (was 0%)
  - `RunContainer`: 46.4% (was 0%)
  - `IsContainerRunning`: 80.0% (was 0%)
  - `GetContainerLogs`: 80.0% (was 0%)
  - `StopContainer`: 100% (was 0%)
  - `BuildImage`: 61.8% (was 0%)

## Approach

Rather than using testcontainers-go to mock Docker, the integration tests use the actual Docker CLI installed on the system. This approach:

1. **Tests real behavior**: Validates actual Docker CLI interactions rather than mocked behavior
2. **Catches CLI changes**: Will detect if Docker CLI behavior changes between versions
3. **Simpler setup**: No need to manage testcontainer lifecycle
4. **More confidence**: Tests run against the same Docker environment the application will use

## Integration Tests Added

### File: `pkg/runner/docker_simple_integration_test.go`

#### TestDockerBasicOperations
Comprehensive test covering basic Docker operations:

- **PullImage**: Verifies image pulling functionality
- **NetworkOperations**: Tests network creation, existence checking, and removal
- **RunSimpleContainer**: Tests running a simple foreground container
- **RunBackgroundContainer**: Tests background container lifecycle (start, verify running, stop)
- **BuildSimpleImage**: Tests building a Docker image from a Dockerfile

#### TestDockerContainerLogs
Tests container log retrieval functionality:
- Runs a container that outputs a test message
- Retrieves logs and verifies the message is present

#### TestDockerNetworkCommunication
Tests container-to-container networking:
- Creates a custom network
- Starts a server container on the network
- Starts a client container that pings the server
- Verifies containers can communicate by hostname

## Test Execution

### Run all tests (including integration tests):
```bash
go test ./pkg/runner -v
```

### Run only unit tests (skip integration tests):
```bash
go test ./pkg/runner -v -short
```

### Run specific integration test:
```bash
go test ./pkg/runner -v -run TestDockerBasicOperations/PullImage
```

### Check coverage:
```bash
go test ./pkg/runner -cover
```

## Dependencies Added

- `github.com/testcontainers/testcontainers-go@v0.42.0` - Added to go.mod for potential future use
  - Note: Current implementation uses actual Docker CLI instead of testcontainers
  - testcontainers-go is available if needed for future container-based testing

## Key Functions Now Tested

### Docker Operations (docker.go)
Previously 0% coverage, now significantly improved:

| Function | Coverage | Tests |
|----------|----------|-------|
| CreateNetwork | 75.0% | Network creation and idempotency |
| RemoveNetwork | 100% | Network removal |
| NetworkExists | 100% | Network existence checking |
| PullImage | 16.0% | Image pulling (basic path) |
| RunContainer | 46.4% | Container execution (foreground & background) |
| IsContainerRunning | 80.0% | Container status checking |
| GetContainerLogs | 80.0% | Log retrieval |
| StopContainer | 100% | Container stopping |
| BuildImage | 61.8% | Image building from Dockerfile |

## Remaining Coverage Gaps

While significant progress was made, the following areas still need test coverage:

### High Priority (Critical Untested Areas)
1. **Execution Engine** (execution.go): Step execution logic
   - `runStep`, `runRunStep`, `runBuildStep`
   - `runParallel`, `runSequence`, `runSwitch`
   - These require more complex test scaffolding with workflow files

2. **Runner Core** (runner.go): Main orchestration
   - `Run`, `RunJob` - Entry point functions
   - Environment loading and output handling
   - These would benefit from end-to-end workflow tests

3. **Reloader** (reloader.go): File watching and hot reload
   - All reloader functionality (0% coverage)
   - Requires file system watching test infrastructure

### Medium Priority
4. **Advanced Docker Operations**:
   - Health check waiting logic
   - Volume mount handling
   - Advanced build options (multi-stage, build contexts)

5. **Schema Validation** (pkg/schema):
   - Complex step validation (parallel, sequence, workflow, switch)
   - `ValidateAndParse`, `ValidateAndParseFile`
   - JSON Schema generation

## Next Steps

To further improve coverage, consider:

1. **End-to-End Workflow Tests**: Create test workflow files and run them through the full execution pipeline
2. **Reloader Tests**: Add file watching tests with temporary file modifications
3. **Error Path Testing**: Add tests for failure scenarios and error handling
4. **Advanced Docker Features**: Test health checks, volume mounts, and port mappings more thoroughly
5. **Schema Validation Tests**: Add tests for all validation scenarios

## Notes

- Integration tests are skipped when running with `-short` flag
- Tests use unique names with timestamps to avoid conflicts
- All tests include proper cleanup (network/container removal)
- Tests have reasonable timeouts (60-120 seconds) to handle Docker operations
- The actual Docker daemon must be running for integration tests to pass
