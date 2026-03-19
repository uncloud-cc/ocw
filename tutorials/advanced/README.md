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
Steps can set key-value pairs which other steps can consume. Similar to Github Actions you simply append `key=value` statements to the `$OUTPUTS` file to set key-value pairs.

To consume key-value pairs, use [templating](#templating) and reference the outputs using the step-id and the key `{{ steps.<step-id>.<key>}}` 👇🏻

```yaml
# outputs.yaml
name: Step Outputs
outputs:
  version: "{{ steps.version.version }}"
  image: "{{ steps.build.image }}"

sequence:
  - name: Generate Version
    id: version
    image: alpine:latest
    cmd: |
      echo "version=1.0.0" >> $OUTPUTS
      echo "Generated version"

  - name: Generate Build Time
    id: buildtime
    image: alpine:latest
    cmd: |
      echo "timestamp=$(date -u +%Y-%m-%d)" >> $OUTPUTS
      echo "Generated timestamp"

  - name: Display Results
    image: alpine:latest
    cmd: |
      echo "Version: {{ steps.version.version }}"
      echo "Build Time: {{ steps.buildtime.timestamp }}"
```

Step outputs can also be added to the outputs of the ocw workflow itself:
```yaml
name: Step Outputs
outputs:
  version: "{{ steps.version.version }}"
  image: "{{ steps.build.image }}"
```

Workflow outputs are displayed the bottom of the workflow run:

```yaml
  Outputs
────────────────────────────────────────
  version: 1.0.0
  image: 2026-03-17

```

## Understanding the `/workflow` mounted folder
The parent folder of workflow files is automatically mounted as `/workflow` inside the containers.

For `build` steps, it's the default `context`.\
For `run` steps, it's the default workdir.

Let's first see what that looks like for a `run` step:
```yaml
# context.yaml
name: /workdir example
sequence:
  - name: Print current directory & contents
    image: alpine:latest
    cmd: |
      echo "Current folder:"
      pwd
      echo "---"
      echo "Contents:"
      ls
```

Running this with `ocw pwd.yaml`, this outputs the `/workflow` dir as the (default) working directory and its contents:

```bash
▶ Pwd & /workdir contents [run]
  Image: alpine:latest
  Image exists: alpine:latest
  │ Current folder:
  │ /workflow
  │ ---
  │ Contents:
  │ Dockerfiles
  │ README.md
  │ demo-build-patterns.yaml
  │ expose.yaml
  │ index.html
  │ jobs.yaml
  │ nested.yaml
  │ networking.yaml
  │ old stuff
  │ outputs.yaml
  │ parallel.yaml
  │ pwd.yaml
  │ sequence.yaml
  │ switch.yaml
  │ templates.yaml
```

For every `run` step, the `/workflow` dir contains the contents of the workflow file's parent folder and is the default dir.

You can also set a different working directory by specify it as `workdir`:

```yaml
- name: Install Dependencies
  image: node:25-alpine
  workdir: /workflow/backend    # Changes working directory to backend
  cmd: npm install
```

Now let's see how the `/worflow` folder is made available in `build` steps:

```yaml
# build-context.yaml
name: Build context
sequence:
  - name: Build the container
    id: build
    build:
      image: ocw-tutorials/context
      dockerfile: context/Dockerfile
      context: ./context

  - name: Run the container
    id: run
    image: "{{ steps.build.image }}"
    background: true
    expose: 80
```

The Dockerfile in question, merely copies a HTML file into the right place:
```Dockerfile
FROM nginx
COPY hello.html /usr/share/nginx/html/index.html
EXPOSE 80
```

Notice how we're setting the context to the subfolder `./context`:
```yaml
- name: Build the container
    id: build
    build:
      image: ocw-tutorials/context
      dockerfile: context/Dockerfile
      context: ./context
```

By default, the `build` steps also have the `/workflow` folder mounted and have this as their default working directory.

## `env` and `secrets`

Use `env` to define workflow-level environment variables with optional defaults. Sensitive env vars can be **marked as secrets** and will be masked in output.

```yaml
# env-secrets.yaml
name: Environment and Secrets Demo

env:
  # Regular env vars (not masked in output)
  DB_PORT: 8080
  DB_USER: admin

  # Secret env vars (marked with 'secret: true', masked in output)
  DB_PASSWORD:
    secret: true
    default: givemeaccess  # Optional default value

  API_KEY:
    secret: true  # No default - must be set in .env

outputs:
  dsn: "psql://{{ env.DB_USER }}@{{ env.DB_PASSWORD }}:{{ env.DB_PORT }}/mytable"

sequence:
  - name: Show Values
    image: alpine:latest
    cmd: |
      echo "DB port: {{ env.DB_PORT }}"
      echo "DB user: {{ env.DB_USER }}"
      echo "DB password: {{ env.DB_PASSWORD }}"
      echo "API key: {{ secrets.API_KEY }}"
```

Create a `.env` file to override defaults:

```bash
# .env
DB_PASSWORD=supersecret123
API_KEY=sk-test-abc123
```

Now run it again with `ocw env-secrets.yaml`. To see the updated secrets in the output, set the `--show-secrets` flag (`ocw env-secrets.yaml --show-secrets`).

> PS: You can set `-e filename.env` to load a different env file

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