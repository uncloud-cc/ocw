# OCW Architecture

OCW separates concerns into three distinct layers: a container runtime abstraction, step implementations, and a workflow orchestrator. This separation allows each layer to evolve independently and makes the system easier to test and extend.

## The Three Layers

```
                    ┌──────────────────┐
                    │       CLI        │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │   OCW Runtime    │  ← Orchestrates workflows, drives iteration
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │      Steps       │  ← Simple (execute) + Composite (iterate)
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

### Steps: Simple and Composite

Steps are divided into two categories based on what they do:

**Simple steps** are leaf nodes that do actual work. They interact with the container runtime and execute to completion:

| Step | What it does |
|------|--------------|
| Run | Executes a container |
| Build | Builds an image from a Dockerfile |

**Composite steps** are control flow nodes that contain other steps. They don't execute directly - instead, they expose an iterator that yields child steps:

| Step | What it does |
|------|--------------|
| Sequence | Runs children one after another |
| Parallel | Runs children concurrently |
| Switch | Chooses a branch based on a value |
| Workflow | Invokes another OCW workflow |

This separation is important: simple steps have an `Execute()` method, while composite steps have an `Iterator()` method. The runtime drives execution by repeatedly asking iterators for the next step(s).

### OCW Runtime

The runtime is the orchestrator. It takes a parsed workflow, finds the requested job, and drives step execution. The runtime handles:

- Building the interpolation scope (environment, secrets, previous step outputs)
- Driving iteration for composite steps
- Executing simple steps
- Running parallel steps concurrently when an iterator yields multiple steps
- Tracking background services and their health
- Cleaning up resources when the workflow completes

## How Execution Works

When you run `ocw build`, here's what happens:

1. The CLI loads and parses the workflow file
2. The CLI creates a container runtime (e.g., Docker) and an OCW runtime
3. The OCW runtime finds the `build` job
4. The job's steps are parsed into executable form
5. The runtime executes each step:
   - For simple steps: call `Execute()` directly
   - For composite steps: get an iterator and call `Next()` repeatedly
6. As steps complete, their outputs are added to the scope
7. Background services are tracked until the workflow completes
8. Resources are cleaned up

### The Iterator Pattern

Composite steps use an iterator pattern that gives the runtime full control over execution:

```
Runtime                          Composite Step
   │                                   │
   │─── Iterator(scope) ──────────────>│
   │<── returns iterator ──────────────│
   │                                   │
   │─── Next(nil) ────────────────────>│
   │<── [step1] ───────────────────────│  (first step)
   │                                   │
   │    (runtime executes step1)       │
   │                                   │
   │─── Next([result1]) ──────────────>│
   │<── [step2] ───────────────────────│  (next step with previous result)
   │                                   │
   │    (runtime executes step2)       │
   │                                   │
   │─── Next([result2]) ──────────────>│
   │<── [], done=true ─────────────────│  (no more steps)
```

When `Next()` returns multiple steps, the runtime executes them in parallel. This is how the parallel step works - it returns all its children at once on the first `Next()` call.

This pattern keeps the runtime in control. It can see what's happening, handle errors uniformly, and in the future could support pause/resume or step-by-step debugging.

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

**Adding a simple step type**: Create a step that implements `Execute()` and a parser that converts the schema to your step type.

**Adding a composite step type**: Create a step that implements `Iterator()` returning a `StepIterator`. The iterator's `Next()` method yields child steps and tracks state between calls.

**Supporting a new container engine**: Implement the container runtime interface. The rest of OCW works unchanged.

**Custom cleanup policies**: The runtime accepts configuration for cleanup behavior, allowing different policies for development vs. CI environments.
