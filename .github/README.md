# OCW GitHub Action

Run [Open Container Workflow (ocw)](https://github.com/uncloud-cc/ocw) - a container-native workflow engine for CI/CD - directly in your GitHub Actions workflows.

## Features

- **Zero configuration** - Docker is pre-installed on GitHub runners
- **Automatic platform detection** - Downloads the correct binary for your runner OS/architecture
- **Simple interface** - Just pass ocw CLI arguments directly
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
        with:
          args: '.'  # Run all workflows in current directory
```

### Run a Specific Job

```yaml
- name: Build
  uses: uncloud-cc/ocw@main
  with:
    args: 'build'  # Run the 'build' job from discovered workflows

- name: Test
  uses: uncloud-cc/ocw@main
  with:
    args: 'test'   # Run the 'test' job
```

### Specify a Workflow File

```yaml
- name: Run specific workflow
  uses: uncloud-cc/ocw@main
  with:
    args: '-f my-workflow.yaml build'
```

### Use a Specific Version

```yaml
- name: Run with specific ocw version
  uses: uncloud-cc/ocw@main
  with:
    version: 'v0.1.0'  # Pin to specific version
    args: 'build'
```

### Run in a Subdirectory

```yaml
- name: Run in project directory
  uses: uncloud-cc/ocw@main
  with:
    working-directory: './my-project'
    args: '.'
```

### Enable Verbose Output

```yaml
- name: Run with verbose output
  uses: uncloud-cc/ocw@main
  with:
    args: '-verbose build'
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `version` | Version of ocw to install (e.g., `v0.1.0`, `latest`) | No | `latest` |
| `args` | Arguments to pass to ocw CLI | Yes | - |
| `working-directory` | Working directory to run ocw in | No | `.` |

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
3. Runs ocw with your specified arguments
4. Propagates exit codes - workflow failures will fail the GitHub Action

## Requirements

- GitHub-hosted runner with Docker (Linux runners only)
- Workflow files in your repository

## Example Workflows

See [`.github/workflows/example-ocw-action.yml`](.github/workflows/example-ocw-action.yml) for complete examples including:
- Running simple jobs
- Running specific workflow files
- Multi-step CI/CD pipelines
- Running in subdirectories

## License

MIT - see [LICENSE](../LICENSE) in the main repository.
