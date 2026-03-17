# Open Container Workflow (ocw) Advanced
This is the second tutorial where we dive into advanced features of ocw workflows. Check-out the [basics tutorial](../basics/README.md) to get an overview of ocw.

Make sure you have the [ocw cli](../basics/README.md#setup) and [Podman](https://podman.io/docs/installation) installed to follow along.

**Table of contents**\
TODO

## Parallel & Sequence
In ocw, steps either run as a `sequence` or in `parallel`. You can also nest the two to express any workflow that comes to mind.

Let's start by seeing a `sequence` in action:


```yaml\
# sequence.yaml
name: Sequential workflow!
sequence:
  - name: Step a)
    image: alpine
    cmd: for i in $(seq 3); do echo "Processing... $i"; sleep 1; done
  - name: Step b)
    image: node:25-alpine
    cmd: npx cowsay Second step!
```

When you run this workflow with `ocw sequence.yaml` the two steps will run one after another.

This is great for sequential workflows - like first building a container and then running it.

Next, let's run something in parallel:
```yaml
# parallel.yaml
name: Parallel workflow
parallel:
  - name: Step a) is happening...
    image: alpine
    cmd: for i in $(seq 10); do echo "Processing step a)... $i"; sleep 1; done
  - name: ...while step b) is happening also
    image: alpine
    cmd: for i in $(seq 7); do echo "Processing step b)... $i"; sleep 1; done
```

As expected both steps run at the same time and the workflow ends when both have ended. This can be great for you have multiple test-suites and you want to run them all in parallel.

## Nesting Parallel and Sequence

You can nest `parallel` inside `sequence` (and vice versa) to create sophisticated workflows:

```yaml
# nested.yaml
name: CI Pipeline
sequence:
  - name: Setup
    image: alpine:latest
    cmd: echo "Setting up..."

  - name: Run Tests in Parallel
    parallel:
      - name: Unit Tests
        image: node:20-alpine
        cmd: echo "Running unit tests..."

      - name: Integration Tests
        image: node:20-alpine
        cmd: echo "Running integration tests..."

      - name: Lint
        image: node:20-alpine
        cmd: echo "Running linter..."

  - name: Build
    image: node:20-alpine
    cmd: echo "Building (only after all tests pass)..."

  - name: Deploy
    image: alpine:latest
    cmd: echo "Deploying..."
```

The tests run in parallel, but the build only starts after all tests pass.

## Templating
Templating make your ocw workflows dynamic. You've already seen `{{ steps.build.image }}` in the basics tutorial - let's explore all the possibilities.

Template expressions use double curly braces: `{{ namespace.key }}`. They can be used in almost any string field in your workflow.

```yaml
# templates.yaml
name: Template Demo
sequence:
  - name: Show Templates
    image: alpine:latest
    cmd: |
      echo "Workflow: {{ workflow.name }}"
      echo "User: {{ env.USER }}"
      echo "Home: {{ env.HOME }}"
```

Run it with `ocw templates.yaml`:

```
▶ Show Templates [run]
  │ Workflow: Template Demo
  │ User: jonaspeeck
  │ Home: /Users/jonaspeeck
✓ Show Templates completed
```

### Available Namespaces

Here's everything you can reference in templates:

| Template              | Description                   |
| --------------------- | ----------------------------- |
| `{{ workflow.name }}` | Name of the workflow          |
| `{{ job.name }}`      | Name of the current job       |
| `{{ env.VARNAME }}`   | Environment variable          |
| `{{ secrets.NAME }}`  | Secret value (from .env file) |
| `{{ steps.ID.KEY }}`  | Output from a previous step   |

Templates work in most string fields including:

- `image`, `cmd`, `entrypoint`, `args[]`
- `env` values, `workdir`
- Build options: `dockerfile`, `context`, `target`, `tags[]`, `buildArgs`
- Switch expressions

## Conditionals (switch / case)

Sometimes you need different behavior based on a value. The `switch/case` construct lets you branch your workflow:

```yaml
# switch.yaml
name: Environment Deploy

# Run with different DEPLOY_ENV values:
#   DEPLOY_ENV=staging ocw switch.yaml     → uses staging case
#   DEPLOY_ENV=production ocw switch.yaml  → uses production case

switch: "{{ env.DEPLOY_ENV }}"
case:
  staging:
    - name: Deploy to Staging
      image: alpine:latest
      cmd: echo "Deploying to staging environment..."

  production:
    - name: Deploy to Production
      image: alpine:latest
      cmd: |
        echo "Deploying to production environment..."
        echo "Running extra safety checks..."

default:
  - name: Deploy to Development
    image: alpine:latest
    cmd: echo "Deploying to development (default)..."
```

Run it with different environments:

```bash
# Uses staging case
DEPLOY_ENV=staging ocw switch.yaml

# Uses production case
DEPLOY_ENV=production ocw switch.yaml
```

The `switch` expression supports any template, so you can base decisions on:

- Environment variables: `{{ env.BRANCH }}`
- Step outputs: `{{ steps.check.result }}`
- Inputs: `{{ inputs.environment }}`

Each case can contain a single step or multiple steps in sequence. If the value doesn't match any case, the `default` case runs.

## Jobs

So far, our workflows have had a single entry point. But real projects need multiple commands: build, test, dev, deploy. Jobs let you define named entry points in one file:

```yaml
# jobs.yaml
name: My Project

jobs:
  build:
    name: Build the App
    sequence:
      - name: Install
        image: node:20-alpine
        cmd: echo "Installing dependencies..."

      - name: Build
        image: node:20-alpine
        cmd: echo "Building..."

  test:
    name: Run Tests
    parallel:
      - name: Unit Tests
        image: node:20-alpine
        cmd: echo "Running unit tests..."

      - name: Lint
        image: node:20-alpine
        cmd: echo "Running linter..."

  dev:
    name: Development Server
    sequence:
      - name: Start Dev
        image: node:20-alpine
        cmd: echo "Starting dev server on http://localhost:3000..."
```

Now you have multiple commands:

```bash
# List available jobs
ocw
# Output:
#   jobs.yaml:
#     - build (Build the App)
#     - test (Run Tests)
#     - dev (Development Server)

# Run specific jobs
ocw build
ocw test
ocw dev
```

This replaces the need for Makefiles, npm scripts, or docker-compose for many use cases.

## Step outputs

## `/workflow`


## Exposing containers
For development environments, you often need to access services from your host machine. The `expose` option makes container ports accessible:

```yaml
# expose.yaml
name: Exposing containers
sequence:
  - name: Start Web Server
    id: webserver
    image: node:25-alpine
    background: true
    cmd: sh -c "echo '<h1>Hello from the dev server 👩🏻‍💻</h1>' > index.html && npx serve -p 8080"
    expose: 8080 # Container port 8080 → localhost:8080
```

You can also map to a different host port:

```yaml
expose:
  - containerPort: 80
    hostPort: 8080 # Container port 80 → localhost:8080
    protocol: http
```

## Container networking

Background containers can be reached by other containers using their `id` as the hostname:

```yaml
# networking.yaml
name: Networking demo
sequence:
  - name: Start Redis
    id: redis # This becomes the hostname
    image: redis:7-alpine
    background: true

  - name: Use Redis
    image: redis:7-alpine
    cmd: redis-cli -h redis SET hello world # Connect via hostname "redis"
```




----
> 🤖 First robot draft - needs human refinement

This tutorial builds on the [basics tutorial](../basics/README.md). We'll cover the powerful features that make ocw great for real-world workflows.

All the example files from this tutorial are available in this directory.

## Table of Contents

- [Open Container Workflow (ocw) Advanced](#open-container-workflow-ocw-advanced)
  - [Table of Contents](#table-of-contents)
  - [Templating](#templating)
    - [Available Namespaces](#available-namespaces)
    - [Step Outputs](#step-outputs)
    - [Using .env Files](#using-env-files)
  - [Parallel Steps](#parallel-steps)
    - [Nesting Parallel and Sequence](#nesting-parallel-and-sequence)
  - [Conditionals (switch / case)](#conditionals-switch--case)
  - [Building Reusable Jobs](#building-reusable-jobs)
    - [Job Outputs](#job-outputs)
  - [Exposing Containers](#exposing-containers)
    - [Background Containers](#background-containers)
    - [Health Checks](#health-checks)
    - [Container Networking](#container-networking)
  - [Putting It All Together](#putting-it-all-together)
  - [What's Next?](#whats-next)
    - [Quick Reference](#quick-reference)

---

## Templating

Templates are how you make workflows dynamic. You've already seen `{{ steps.build.image }}` in the basics tutorial - let's explore all the possibilities.

Template expressions use double curly braces: `{{ namespace.key }}`. They can be used in almost any string field in your workflow.

```yaml
# templates.yaml
name: Template Demo
sequence:
  - name: Show Templates
    image: alpine:latest
    cmd: |
      echo "Workflow: {{ workflow.name }}"
      echo "User: {{ env.USER }}"
      echo "Home: {{ env.HOME }}"
```

Run it with `ocw templates.yaml`:

```
▶ Show Templates [run]
  │ Workflow: Template Demo
  │ User: jonaspeeck
  │ Home: /Users/jonaspeeck
✓ Show Templates completed
```

### Available Namespaces

Here's everything you can reference in templates:

| Template              | Description                   |
| --------------------- | ----------------------------- |
| `{{ workflow.name }}` | Name of the workflow          |
| `{{ job.name }}`      | Name of the current job       |
| `{{ env.VARNAME }}`   | Environment variable          |
| `{{ secrets.NAME }}`  | Secret value (from .env file) |
| `{{ inputs.NAME }}`   | Workflow input                |
| `{{ steps.ID.KEY }}`  | Output from a previous step   |

Templates work in most string fields including:

- `image`, `cmd`, `entrypoint`, `args[]`
- `env` values, `workdir`
- Build options: `dockerfile`, `context`, `target`, `tags[]`, `buildArgs`
- Switch expressions

### Step Outputs

Steps can pass data to later steps. Any step with an `id` gets a special `$OUTPUTS` environment variable. Write `KEY=value` lines to it:

```yaml
# outputs.yaml
name: Step Outputs Demo
sequence:
  - name: Generate Version
    id: versioner
    image: alpine:latest
    cmd: |
      VERSION="1.0.$(date +%s)"
      echo "version=$VERSION" >> $OUTPUTS
      echo "Generated version: $VERSION"

  - name: Use Version
    image: alpine:latest
    cmd: |
      echo "Building version {{ steps.versioner.version }}"
```

Run it with `ocw outputs.yaml`:

```
▶ Generate Version [run]
  │ Generated version: 1.0.1710425678
  Output: version=1.0.1710425678
✓ Generate Version completed

▶ Use Version [run]
  │ Building version 1.0.1710425678
✓ Use Version completed
```

Build steps automatically register their image as an output:

```yaml
- name: Build App
  id: build
  build:
    image: myapp:latest
    dockerfile: Dockerfile

- name: Run App
  image: "{{ steps.build.image }}" # Uses myapp:latest
  cmd: ./start.sh
```

### Using .env Files

Place a `.env` file in your workflow directory and values are automatically loaded:

```bash
# .env
API_KEY=secret123
DATABASE_URL=postgres://localhost/mydb
APP_VERSION=2.0.0
```

Then reference them in your workflow:

```yaml
- name: Show Config
  image: alpine:latest
  cmd: |
    echo "Version: {{ env.APP_VERSION }}"

- name: Connect to Database
  image: postgres:16-alpine
  env:
    DATABASE_URL: "{{ secrets.DATABASE_URL }}"
  cmd: psql "$DATABASE_URL" -c "SELECT 1"
```

Values from `.env` are available as both `{{ env.NAME }}` and `{{ secrets.NAME }}`. Use `secrets` for sensitive values to make your intent clear.

You can also specify a custom env file with the `-e` flag:

```bash
ocw -e production.env deploy
```

---

## Parallel Steps

So far we've run steps in `sequence` - one after another. But what if you have independent tasks that could run at the same time? That's where `parallel` comes in.

```yaml
# parallel.yaml
name: Parallel Checks
parallel:
  - name: Check Node
    image: node:20-alpine
    cmd: node --version

  - name: Check Python
    image: python:3.12-alpine
    cmd: python --version

  - name: Check Go
    image: golang:1.22-alpine
    cmd: go version
```

Run it with `ocw parallel.yaml`:

```
────────────────────────────────────────────────────────────
  Parallel Checks
────────────────────────────────────────────────────────────
>>> Running 3 steps in parallel

▶ Check Node [run]
  │ v20.18.0
✓ Check Node completed

▶ Check Python [run]
  │ Python 3.12.4
✓ Check Python completed

▶ Check Go [run]
  │ go version go1.22.5 linux/arm64
✓ Check Go completed

────────────────────────────────────────────────────────────
  ✓ Parallel Checks completed successfully (1.2s)
────────────────────────────────────────────────────────────
```

All three checks ran simultaneously instead of taking 3x as long. This is great for CI pipelines where you want to run linting, type checking, and tests at the same time.

### Nesting Parallel and Sequence

You can nest `parallel` inside `sequence` (and vice versa) to create sophisticated workflows:

```yaml
# nested.yaml
name: CI Pipeline
sequence:
  - name: Setup
    image: alpine:latest
    cmd: echo "Setting up..."

  - name: Run Tests in Parallel
    parallel:
      - name: Unit Tests
        image: node:20-alpine
        cmd: echo "Running unit tests..."

      - name: Integration Tests
        image: node:20-alpine
        cmd: echo "Running integration tests..."

      - name: Lint
        image: node:20-alpine
        cmd: echo "Running linter..."

  - name: Build
    image: node:20-alpine
    cmd: echo "Building (only after all tests pass)..."

  - name: Deploy
    image: alpine:latest
    cmd: echo "Deploying..."
```

The tests run in parallel, but the build only starts after all tests pass.

---

## Conditionals (switch / case)

Sometimes you need different behavior based on a value. The `switch/case` construct lets you branch your workflow:

```yaml
# switch.yaml
name: Environment Deploy

# Run with different DEPLOY_ENV values:
#   DEPLOY_ENV=staging ocw switch.yaml     → uses staging case
#   DEPLOY_ENV=production ocw switch.yaml  → uses production case

switch: "{{ env.DEPLOY_ENV }}"
case:
  staging:
    - name: Deploy to Staging
      image: alpine:latest
      cmd: echo "Deploying to staging environment..."

  production:
    - name: Deploy to Production
      image: alpine:latest
      cmd: |
        echo "Deploying to production environment..."
        echo "Running extra safety checks..."

default:
  - name: Deploy to Development
    image: alpine:latest
    cmd: echo "Deploying to development (default)..."
```

Run it with different environments:

```bash
# Uses staging case
DEPLOY_ENV=staging ocw switch.yaml

# Uses production case
DEPLOY_ENV=production ocw switch.yaml
```

The `switch` expression supports any template, so you can base decisions on:

- Environment variables: `{{ env.BRANCH }}`
- Step outputs: `{{ steps.check.result }}`
- Inputs: `{{ inputs.environment }}`

Each case can contain a single step or multiple steps in sequence. If the value doesn't match any case, the `default` case runs.

---

## Building Reusable Jobs

So far, our workflows have had a single entry point. But real projects need multiple commands: build, test, dev, deploy. Jobs let you define named entry points in one file:

```yaml
# jobs.yaml
name: My Project

jobs:
  build:
    name: Build the App
    sequence:
      - name: Install
        image: node:20-alpine
        cmd: echo "Installing dependencies..."

      - name: Build
        image: node:20-alpine
        cmd: echo "Building..."

  test:
    name: Run Tests
    parallel:
      - name: Unit Tests
        image: node:20-alpine
        cmd: echo "Running unit tests..."

      - name: Lint
        image: node:20-alpine
        cmd: echo "Running linter..."

  dev:
    name: Development Server
    sequence:
      - name: Start Dev
        image: node:20-alpine
        cmd: echo "Starting dev server on http://localhost:3000..."
```

Now you have multiple commands:

```bash
# List available jobs
ocw
# Output:
#   jobs.yaml:
#     - build (Build the App)
#     - test (Run Tests)
#     - dev (Development Server)

# Run specific jobs
ocw build
ocw test
ocw dev
```

This replaces the need for Makefiles, npm scripts, or docker-compose for many use cases.

### Job Outputs

Jobs can define outputs that are displayed after completion:

```yaml
jobs:
  build:
    name: Build
    outputs:
      version: "{{ steps.version.value }}"
      image: "{{ steps.build.image }}"
    sequence:
      - name: Get Version
        id: version
        image: alpine:latest
        cmd: echo "value=1.2.3" >> $OUTPUTS

      - name: Build Image
        id: build
        build:
          image: myapp:latest
          dockerfile: Dockerfile
```

After the job completes, you'll see:

```
  Outputs
────────────────────────────────────────
  version: 1.2.3
  image: myapp:latest
```

---

## Exposing Containers

For development environments, you often need to access services from your host machine. The `expose` option makes container ports accessible:

```yaml
- name: Start Web Server
  image: nginx:alpine
  background: true
  expose: 8080 # Container port 8080 → localhost:8080
```

You can also map to a different host port:

```yaml
expose:
  - containerPort: 80
    hostPort: 8080 # Container port 80 → localhost:8080
    protocol: http
```

### Background Containers

Background containers start, wait until healthy, and keep running while the workflow continues:

```yaml
# background.yaml
name: Dev Environment

jobs:
  dev:
    name: Start Development
    sequence:
      - name: Start Database
        id: db
        image: postgres:16-alpine
        background: true
        expose: 5432
        healthCheck:
          cmd: pg_isready -U postgres
          interval: 2s
          retries: 15
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: secret
          POSTGRES_DB: myapp

      - name: Show Status
        image: alpine:latest
        cmd: |
          echo "Database is ready!"
          echo "Connect from host: psql -h localhost -U postgres -d myapp"
          echo ""
          echo "Press Ctrl+C to stop all services..."
```

Key things to note:

- `background: true` - Container keeps running
- `expose: 5432` - Access from host at localhost:5432
- `healthCheck` - Waits for service to be ready before continuing
- The `id` becomes the hostname for other containers

When background containers are running, ocw waits for Ctrl+C, then cleans everything up automatically.

### Health Checks

Health checks ensure services are ready before continuing:

```yaml
healthCheck:
  cmd: redis-cli ping # Command to check health
  interval: 1s # Time between checks
  retries: 10 # Number of attempts before failing
  startPeriod: 2s # Grace period before first check
```

Common health check commands:

| Service     | Health Check                                    |
| ----------- | ----------------------------------------------- |
| PostgreSQL  | `pg_isready -U postgres`                        |
| Redis       | `redis-cli ping`                                |
| HTTP Server | `wget -q --spider http://localhost:8080/health` |
| MySQL       | `mysqladmin ping -h localhost`                  |

### Container Networking

Background containers with an `id` can be reached by other containers using that id as the hostname:

```yaml
- name: Start Redis
  id: redis # This becomes the hostname
  image: redis:7-alpine
  background: true

- name: Use Redis
  image: redis:7-alpine
  cmd: redis-cli -h redis SET hello world # Connect via hostname "redis"
```

---

## Putting It All Together

Here's a realistic development workflow combining everything we've learned:

```yaml
# dev-workflow.yaml
name: Full Stack Development

jobs:
  dev:
    name: Start Dev Environment
    outputs:
      db_url: "postgres://postgres:secret@localhost:5432/app"
      redis_url: "redis://localhost:6379"
    sequence:
      # Start services in parallel
      - name: Start Services
        parallel:
          - name: PostgreSQL
            id: postgres
            image: postgres:16-alpine
            background: true
            expose: 5432
            healthCheck:
              cmd: pg_isready -U postgres
            env:
              POSTGRES_PASSWORD: secret
              POSTGRES_DB: app

          - name: Redis
            id: redis
            image: redis:7-alpine
            background: true
            expose: 6379
            healthCheck:
              cmd: redis-cli ping

      - name: Ready
        image: alpine:latest
        cmd: |
          echo "Services are ready!"
          echo ""
          echo "PostgreSQL: localhost:5432 (or postgres:5432 from containers)"
          echo "Redis: localhost:6379 (or redis:6379 from containers)"
          echo ""
          echo "Press Ctrl+C to stop all services..."

  test:
    name: Run Tests
    switch: "{{ env.TEST_TYPE }}"
    case:
      unit:
        - name: Unit Tests
          image: node:20-alpine
          cmd: echo "Running unit tests only..."

      integration:
        - name: Integration Tests
          image: node:20-alpine
          cmd: echo "Running integration tests only..."

    default:
      - name: All Tests
        parallel:
          - name: Unit Tests
            image: node:20-alpine
            cmd: echo "Running unit tests..."

          - name: Integration Tests
            image: node:20-alpine
            cmd: echo "Running integration tests..."

  build:
    name: Build for Production
    outputs:
      image: "{{ steps.build.image }}"
      version: "{{ steps.version.tag }}"
    sequence:
      - name: Get Version
        id: version
        image: alpine:latest
        cmd: |
          TAG="v1.0.$(date +%Y%m%d)"
          echo "tag=$TAG" >> $OUTPUTS
          echo "Building version $TAG"

      - name: Build Image
        id: build
        build:
          image: "myapp:{{ steps.version.tag }}"
          dockerfile: Dockerfile
```

Now you have three commands:

```bash
ocw dev      # Start development environment
ocw test     # Run all tests (or TEST_TYPE=unit ocw test)
ocw build    # Build production image
```

---

## What's Next?

You now know all the major features of ocw! Here are some things to explore:

- Check out the [examples](../../examples/) for more real-world patterns
- Look at [pr.yaml](../../examples/pr.yaml) for a complete CI workflow
- Try building a workflow for your own project

### Quick Reference

| Feature            | Description                                   |
| ------------------ | --------------------------------------------- |
| `{{ template }}`   | Dynamic values from env, secrets, steps, etc. |
| `$OUTPUTS`         | Write `KEY=value` to pass data between steps  |
| `sequence`         | Run steps one after another                   |
| `parallel`         | Run steps simultaneously                      |
| `switch/case`      | Conditional branching                         |
| `jobs`             | Multiple named entry points                   |
| `background: true` | Keep container running                        |
| `expose`           | Make ports accessible from host               |
| `healthCheck`      | Wait for service to be ready                  |
| `-e file.env`      | Load custom environment file                  |

**Got feedback?**
[Join the community on Github](https://github.com/opencontainerworkflow/ocw/discussions) to ask questions, get help and share feedback.
