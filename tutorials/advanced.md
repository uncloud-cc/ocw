# Open Container Workflow (ocw) Advanced

This is the second tutorial where we dive into advanced features of ocw workflows. Check-out the [basics tutorial](./basics.md) to get an overview of ocw.

Make sure you have the [ocw cli](./basics.md#setup) and [Podman](https://podman.io/docs/installation) installed to follow along.

> 📚 **Want to follow along?** All example files from this tutorial are available in the [`examples/`](../examples/) directory with numbered prefixes (4_, 5_, etc.) to help you follow along progressively. Clone the repo to get started:
> ```bash
> git clone https://github.com/uncloud-cc/ocw.git
> cd ocw/examples
> ```

**Table of contents**

- [Parallel & Sequence](#parallel--sequence)
- [Nesting Parallel and Sequence](#nesting-parallel-and-sequence)
- [Templating](#templating)
- [Conditionals (switch / case)](#conditionals-switch--case)
- [Jobs](#jobs)
- [Step outputs](#step-outputs)
- [Understanding the `/workflow` mounted folder](#understanding-the-workflow-mounted-folder)
- [`env` and `secrets`](#env-and-secrets)
- [Exposing containers](#exposing-containers)
- [Container networking](#container-networking)
- [Watch mode](#watch-mode)

## Parallel & Sequence
In ocw, steps either run as a `sequence` or in `parallel`. You can also nest the two to express any workflow that comes to mind.

Let's start by seeing a `sequence` in action:


```yaml
# 4_sequence.yaml
name: Sequential workflow!
sequence:
  - name: Step a)
    image: alpine
    cmd: for i in $(seq 3); do echo "Processing... $i"; sleep 1; done
  - name: Step b)
    image: node:25-alpine
    cmd: npx cowsay Second step!
```

When you run this workflow with `ocw 4_sequence.yaml` the two steps will run one after another.

This is great for sequential workflows - like first building a container and then running it.

Next, let's run something in parallel:
```yaml
# 5_parallel.yaml
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
# 6_nested.yaml
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
# 7_templates.yaml
name: Template Demo
sequence:
  - name: Show Templates
    image: alpine:latest
    cmd: |
      echo "Workflow: {{ workflow.name }}"
      echo "User: {{ env.USER }}"
      echo "Home: {{ env.HOME }}"
```

Run it with `ocw 7_templates.yaml`:

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
| `{{ env.VARNAME }}`   | Environment variable        |
| `{{ steps.ID.KEY }}`  | Output from a previous step   |

Templates work in most string fields including:

- `image`, `cmd`, `entrypoint`, `args[]`
- `env` values, `workdir`
- Build options: `dockerfile`, `context`, `target`, `tags[]`, `buildArgs`
- Switch expressions

## Conditionals (switch / case)

Sometimes you need different behavior based on a value. The `switch/case` construct lets you branch your workflow:

```yaml
# 8_switch.yaml
name: Environment Deploy

# Run with different DEPLOY_ENV values:
#   DEPLOY_ENV=staging ocw 8_switch.yaml     → uses staging case
#   DEPLOY_ENV=production ocw 8_switch.yaml  → uses production case

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
DEPLOY_ENV=staging ocw 8_switch.yaml

# Uses production case
DEPLOY_ENV=production ocw 8_switch.yaml
```

The `switch` expression supports any template, so you can base decisions on:

- Environment variables: `{{ env.BRANCH }}`
- Step outputs: `{{ steps.check.result }}`

Each case can contain a single step or multiple steps in sequence. If the value doesn't match any case, the `default` case runs.

## Jobs

So far, our workflows have had a single entry point. But real projects need multiple commands: build, test, dev, deploy. Jobs let you define named entry points in one file:

```yaml
# 9_jobs.yaml
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
#   9_jobs.yaml:
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
# 10_outputs.yaml
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
# 11_context.yaml
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

Running this with `ocw 11_context.yaml`, this outputs the `/workflow` dir as the (default) working directory and its contents:

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
  │ 14_expose.yaml
  │ index.html
  │ 9_jobs.yaml
  │ 6_nested.yaml
  │ 15_networking.yaml
  │ old stuff
  │ 10_outputs.yaml
  │ 5_parallel.yaml
  │ pwd.yaml
  │ 4_sequence.yaml
  │ 8_switch.yaml
  │ 7_templates.yaml
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
# 12_build_context.yaml
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
# 13_env_secrets.yaml
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

Now run it again with `ocw 13_env_secrets.yaml`. To see the updated secrets in the output, set the `--show-secrets` flag (`ocw 13_env_secrets.yaml --show-secrets`).

> PS: You can set `-e filename.env` to load a different env file

## Exposing containers
For development environments, you often need to access services from your host machine. The `expose` option makes container ports accessible:

```yaml
# 14_expose.yaml
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
# 15_networking.yaml
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

## Watch mode
Another thing that's cruicial for running a local development server is the ability to automatically reload your server on file changes.

With `watch` that becomes straight forward:

```yaml
# 16_watch.yaml
name: Express TypeScript Dev Server
sequence:
  - name: Build express.js image
    id: build
    build:
      image: ocw-tutorials/express
      dockerfile: nodejs/Dockerfile
      context: ./nodejs

  - name: Run Express Server with Watch Mode
    id: run
    image: "{{ steps.build.image }}"
    background: true
    expose: 3000
    workdir: /app
    watch: true
```

Go ahead and change the code in `nodejs/src/index.ts` (in the `examples` folder) and you'll notice that the container is rebuilt & restarted.

If you want to get fancy with it, you can specify which files should trigger the reload:
```yaml
watch:
  files:
    - "./nodejs/src/**/*.ts"
  mode: rebuild-reload
```

See [reference](../README.md#reference) for details.

## Volume mounts (Full example)

Volume mounts let you share directories between your host and containers. Unlike the automatic `/workflow` mount, volumes give you fine-grained control over what gets mounted, where, and with what permissions.

By mounting parts of your system into containers, ocw workflows can easily serve as replacements for Makefile or setup scripts.

Let's look at a real-world example: a Hugo static site workflow with dev server, build, and deploy jobs.

### Defining volumes

Volumes are defined at the workflow level and referenced by name in jobs and steps:

```yaml
# 17_hugo.yaml
name: Hugo static site dev server & deployment

volumes:
  # Source content - read-only by default
  content:
    path: ./website/content
    # mode: readonly (default)
    mountPath: /src/content

  # Hugo config - needs write access for .hugo_build.lock
  site:
    path: ./website/site
    mode: readwrite
    mountPath: /src/site

  # Build output - needs write access
  dist:
    path: ./website/dist
    mode: readwrite
    mountPath: /output

  # AWS credentials for deployment
  secrets:
    path: ~/.aws
    mountPath: /root/.aws
```

Each volume has:
- **`path`**: The host directory (relative to workflow file, or absolute like `~/.aws`)
- **`mountPath`**: Where it appears inside containers
- **`mode`**: Either `readonly` (read-only, default) or `readwrite` (read-write)

### Using volumes in jobs

Reference volumes at the job level to grant all steps access:

```yaml
jobs:
  build:
    # All steps in this job get content and site volumes
    volumes:
      - content
      - site

    sequence:
      - name: Build Hugo site
        image: hugomods/hugo:latest
        cmd: hugo --source /src/site --contentDir /src/content --destination /output --minify
        # This step also needs the dist volume for output
        volumes:
          - dist
```

Job-level volumes are inherited by all steps. Steps can add additional volumes they need.

### Overriding mount options per-step

Sometimes you need different mount settings for specific steps. You can override `mountPath` or make a `readwrite` volume read-only:

```yaml
- name: Verify output
  image: alpine
  volumes:
    # Override mountPath: use /public instead of default /output
    # Also mount as read-only for safety (even though dist is defined as readwrite)
    - name: dist
      mountPath: /public
      readonly: true
  cmd: |
    echo "Generated files:"
    find /public -type f | head -20
```

This is useful when:
- Different tools expect files at different paths
- You want to prevent accidental writes in verification steps
- You're passing volumes to third-party images with specific path requirements

### Shorthand syntax

For simple cases, ocw supports shorthand:

```yaml
# Single volume - string instead of array
clean:
  volumes: dist
  image: alpine
  cmd: rm -rf /output/*

# Equivalent to:
clean:
  volumes:
    - name: dist
```

Run it with:

```bash
ocw dev     # Start dev server at http://localhost:1313
ocw build   # Build static site to ./website/dist
ocw deploy  # Push to S3
```