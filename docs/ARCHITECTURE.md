# OCW Architecture

OCW separates concerns into three distinct layers: a container runtime abstraction, step implementations, and a workflow orchestrator. This separation allows each layer to evolve independently and makes the system easier to test and extend.

## The Three Layers

```
                    ┌──────────────────┐
                    │       CLI        │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │   OCW Runtime    │  ← Orchestrates workflows and jobs
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │      Steps       │  ← Executable units of work
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │ Container Runtime│  ← Abstract container operations
                    └──────────────────┘
```

### Container Runtime

The container runtime is an interface that abstracts away the specifics of Docker, Podman, or any other container engine. It handles:

- Pulling and building images
- Creating, starting, and stopping containers
- Managing volumes and networks
- Streaming logs and attaching to containers

The CLI is responsible for providing a concrete implementation. This means OCW's core logic never directly calls Docker - it always goes through the interface. Swapping container engines requires no changes to the runtime or steps.

### Steps

Steps are the building blocks of workflows. Each step type knows how to execute one kind of operation:

| Step | What it does |
|------|--------------|
| Run | Executes a container |
| Build | Builds an image from a Dockerfile |
| Parallel | Runs child steps concurrently |
| Sequence | Runs child steps one after another |
| Switch | Chooses a branch based on a value |
| Workflow | Invokes another OCW workflow |

Every step follows the same pattern: it receives an execution context, does its work using the container runtime, and returns a result. Steps don't know about jobs or workflows - they just execute and report back.

Each step type has a parser that converts the YAML schema into an executable step. This is where template interpolation happens. By the time a step executes, all `{{ }}` expressions have been resolved to concrete values.

### OCW Runtime

The runtime is the orchestrator. It takes a parsed workflow, finds the requested job, and executes its steps according to the defined flow control. The runtime handles:

- Building the interpolation scope (environment, secrets, previous step outputs)
- Dispatching steps to the appropriate executor
- Tracking background services and their health
- Cleaning up resources when the workflow completes

## How Execution Works

When you run `ocw build`, here's what happens:

1. The CLI loads and parses the workflow file
2. The CLI creates a container runtime (e.g., Docker) and an OCW runtime
3. The OCW runtime finds the `build` job
4. For each step in the job:
   - The runtime builds an interpolation scope with current env/secrets/outputs
   - The step's parser resolves all template expressions
   - The step executes using the container runtime
   - Outputs are collected and added to the scope for the next step
5. Background services are monitored until the workflow completes
6. Resources are cleaned up

## Step Communication

Steps communicate through outputs. A step can declare an ID and produce key-value outputs that subsequent steps reference:

```yaml
- name: Build image
  id: build
  build:
    image: myapp:latest

- name: Run tests
  image: "{{ steps.build.image }}"
  cmd: npm test
```

The runtime maintains an output store that accumulates results as steps complete. When parsing a step, the interpolation scope includes all outputs from previously completed steps.

For parallel execution, each branch receives a copy of the current scope. This prevents race conditions - parallel steps can't see each other's outputs, only outputs from steps that completed before the parallel block started.

## Background Services

Steps can run in the background with `background: true`. These become services that other steps can depend on:

```yaml
- name: Database
  id: db
  image: postgres:15
  background: true
  healthCheck:
    cmd: pg_isready

- name: Run migrations
  image: myapp
  needs: [db]
  cmd: ./migrate
```

The runtime tracks background containers, monitors their health checks, and handles the `needs` dependencies. When a step declares `needs: [db]`, the runtime waits for that service to pass its health check before proceeding.

Background containers are cleaned up when the workflow completes, unless configured otherwise.

## Resource Management

The runtime creates a network for each workflow execution. All containers in the workflow join this network, allowing them to communicate by container name.

Volumes defined in the workflow are resolved to host paths and mounted into containers as needed. The runtime tracks all created resources and cleans them up in reverse order when execution completes - containers first, then networks and volumes.

Cleanup behavior is configurable. Background containers can be kept running after the workflow completes for development workflows, or always cleaned up for CI environments.

## Template Interpolation

OCW uses `{{ }}` syntax for dynamic values. Templates are resolved when parsing steps, not during execution. Supported references:

- `{{ env.VAR }}` - Environment variable
- `{{ secrets.NAME }}` - Secret value
- `{{ steps.ID.key }}` - Output from a previous step
- `{{ inputs.NAME }}` - Workflow input

Resolving templates at parse time catches errors early. If a step references `{{ steps.foo.bar }}` but step `foo` doesn't exist or hasn't run yet, the error is raised before any containers start.

## Extending OCW

**Adding a step type**: Create a new step implementation with an `Execute` method and a parser. Register it in the runtime's step dispatcher.

**Supporting a new container engine**: Implement the container runtime interface. The rest of OCW works unchanged.

**Custom cleanup policies**: The runtime accepts configuration for cleanup behavior, allowing different policies for development vs. CI environments.
