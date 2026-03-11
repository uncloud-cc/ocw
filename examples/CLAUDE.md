# OCW Examples

This folder contains example OCW workflow files demonstrating various features.

## Schema Version

All examples use `schemaVersion: "0.1.0"`.

## Key Concepts

### Workflow Structure

A workflow can have:
- **Direct flow control** (`parallel`, `sequence`, or `switch`) - runs when you execute the file directly
- **Jobs** - Named entry points that can be run via `ocw <job-name>`

### Jobs

Jobs are named entry points in a workflow file. They allow you to define multiple related workflows in a single file:

```yaml
jobs:
  build:
    name: Build Application
    sequence:
      - name: Build
        build:
          image: myapp:latest
          context: /workflow

  test:
    name: Run Tests
    parallel:
      - name: Unit Tests
        image: node:20-alpine
        cmd: npm test

  dev:
    name: Development
    sequence:
      - name: Start Dev Server
        image: node:20-alpine
        cmd: npm run dev
```

Run with: `ocw build`, `ocw test`, or `ocw dev`

### Path Convention

Use `/workflow` as the base path in containers. This is automatically mounted to the directory containing the workflow file.

```yaml
- name: Build
  build:
    context: /workflow           # The workflow directory
    dockerfile: Dockerfile

- name: Run Tests
  image: node:20-alpine
  workdir: /workflow/tests       # Subdirectory
  cmd: npm test
```

## Examples Overview

| File | Description |
|------|-------------|
| `01-basic-ci.yaml` | Simple sequential CI pipeline |
| `02-docker-build.yaml` | Docker image build with multi-platform support |
| `03-parallel-testing.yaml` | Parallel test execution |
| `04-complex-pipeline.yaml` | Mixed sequential/parallel pipeline |
| `05-switch-case-deployment.yaml` | Environment-based conditional deployment |
| `06-with-extensions-and-config.yaml` | Extensions, config, and monitoring |
| `07-nested-switch-parallel.yaml` | Multi-region deployment with nested constructs |
| `08-secrets-example.yaml` | Secrets management |
| `09-workflow-invocation.yaml` | Calling reusable workflows |
| `10-advanced-features.yaml` | Advanced build/run options |
| `11-jobs-local-dev.yaml` | Local dev environment with jobs |
| `12-jobs-ci-pipeline.yaml` | CI pipeline with multiple jobs |
| `pr.yaml` | Real-world PR CI workflow |
| `nodejs/` | Full-stack Node.js example |

## Running Examples

```bash
# List available jobs in current directory
ocw

# Run a specific job
ocw dev
ocw build
ocw test

# Run from a specific file
ocw -f pr.yaml lint

# Validate without running
ocw -validate -f 01-basic-ci.yaml
```
