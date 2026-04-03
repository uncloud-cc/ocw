# OCW GitHub Action

Run [Open Container Workflow (ocw)](https://github.com/uncloud-cc/ocw) - a container-native workflow engine for CI/CD - directly in your GitHub Actions workflows.

## Features

- **Zero configuration** - Docker is pre-installed on GitHub runners
- **Automatic platform detection** - Downloads the correct binary for your runner OS/architecture
- **Individual CLI arguments** - Pass flags as separate inputs, just like running locally
- **Version pinning** - Use specific versions or always get latest
- **Fast setup** - Composite action installs in seconds

## Usage

### Basic Example

```yaml
name: CI

on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run OCW workflow
        uses: uncloud-cc/ocw@main
```

### Run a Specific Workflow File

```yaml
- name: Run workflow file
  uses: uncloud-cc/ocw@main
  with:
    file: 'workflow.yaml'
```

### Run a Specific Job

```yaml
- name: Build
  uses: uncloud-cc/ocw@main
  with:
    job: 'build'

- name: Test
  uses: uncloud-cc/ocw@main
  with:
    job: 'test'
```

### Combine File and Job

```yaml
- name: Build from specific file
  uses: uncloud-cc/ocw@main
  with:
    file: 'ci.yaml'
    job: 'build'
```

### Enable Verbose Output

```yaml
- name: Run with verbose output
  uses: uncloud-cc/ocw@main
  with:
    file: 'workflow.yaml'
    verbose: 'true'
```

### Use Environment File

```yaml
- name: Run with env file
  uses: uncloud-cc/ocw@main
  with:
    file: 'workflow.yaml'
    env: '.env.production'
```

### Force Remove Existing Containers

```yaml
- name: Force run
  uses: uncloud-cc/ocw@main
  with:
    job: 'dev'
    force: 'true'
```

### Validate Without Running

```yaml
- name: Validate workflow
  uses: uncloud-cc/ocw@main
  with:
    file: 'workflow.yaml'
    validate: 'true'
```

### Run in a Subdirectory

```yaml
- name: Run in project directory
  uses: uncloud-cc/ocw@main
  with:
    working-directory: './my-project'
    job: 'build'
```

### Use a Specific Version

```yaml
- name: Run with specific ocw version
  uses: uncloud-cc/ocw@main
  with:
    version: 'v0.1.0'  # Pin to specific version
    file: 'workflow.yaml'
```

### Show Secrets (Not Recommended for CI)

```yaml
- name: Debug with secrets
  uses: uncloud-cc/ocw@main
  with:
    file: 'workflow.yaml'
    show-secrets: 'true'
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `version` | Version of ocw to install (e.g., `v0.1.0`, `latest`) | No | `latest` |
| `file` | Workflow file(s) to run (-f flag) | No | - |
| `env` | Environment file to load (-e flag) | No | - |
| `job` | Job name to run (e.g., `build`, `test`, `dev`) | No | - |
| `show-secrets` | Show secret values in output | No | `false` |
| `force` | Force remove existing containers | No | `false` |
| `verbose` | Enable verbose logging | No | `false` |
| `validate` | Validate workflow without running | No | `false` |
| `working-directory` | Working directory to run ocw in | No | `.` |
| `args` | Additional raw arguments (for advanced use cases) | No | - |

## Supported Platforms

The action automatically detects and downloads the correct binary for:

- **Linux**: x86_64, arm64, i386
- **macOS**: x86_64, arm64 (for self-hosted runners)

GitHub-hosted runners include:
- `ubuntu-latest` (Linux x86_64) ✓
- `ubuntu-22.04` (Linux x86_64) ✓
- `ubuntu-20.04` (Linux x86_64) ✓

Windows runners are not supported (ocw requires Docker, which works differently on Windows runners).

## How It Works

1. Downloads the ocw binary from [GitHub Releases](https://github.com/uncloud-cc/ocw/releases)
2. Installs it to `~/.local/bin/`
3. Builds the command from individual inputs
4. Runs ocw with your specified arguments
5. Propagates exit codes - workflow failures will fail the GitHub Action

## Requirements

- GitHub-hosted runner with Docker (Linux runners only)
- Workflow files in your repository

## Example Workflows

See [`.github/workflows/example-ocw-action.yml`](.github/workflows/example-ocw-action.yml) for complete examples including:
- Running workflow files
- Running specific jobs
- Multi-step CI/CD pipelines
- Running in subdirectories
- Validation

## License

MIT - see [LICENSE](../LICENSE) in the main repository.
