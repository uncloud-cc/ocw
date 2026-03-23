# Open Container Workflow (ocw)

Container-native workflows for local development & CI/CD.

## Example
```javascript
// server.js
require('http').createServer((_, res) => {
  res.end(`Hello world!`);
}).listen(8888);
```

```yaml
# hello.yaml
name: Hello World
sequence:
  - id: server
    image: node:25-alpine
    cmd: node server.js

    # Make it a hot-reloading dev server
    background: true
    watch: true
    expose: 8888
```

Run the workflow with `ocw hello.yaml` and play around with `server.js` to see hot-reloading in action ✨

## Install

Make sure you have these two installed:

- [Go](https://go.dev/dl/) `v1.24`
- [Podman](https://podman.io/docs/installation) `v5.7`

Then install the `ocw` CLI: `go install github.com/uncloud-cc/ocw/cmd/ocw@<commit-hash>`

Check if it worked by running `ocw --help`

## Getting started

- Go through the [basics tutorial](./tutorials/basics/README.md)
- Check out advanced features in the [advanced tutorial](./tutorials/advanced/README.md)
- Or check out the [examples](./examples/)

## Feedback

[Join the community](https://github.com/uncloud-cc/ocw/discussions) to ask questions, get help and share feedback.

## License

MIT License - see [LICENSE](LICENSE) file.

## Reference

- [Open Container Workflow (ocw)](#open-container-workflow-ocw)
  - [Install](#install)
  - [Getting started](#getting-started)
  - [Feedback](#feedback)
  - [License](#license)
  - [Reference](#reference)
    - [Workflow Structure](#workflow-structure)
    - [Jobs](#jobs)
    - [Flow Control](#flow-control)
      - [sequence](#sequence)
      - [parallel](#parallel)
      - [switch](#switch)
      - [Nesting](#nesting)
    - [Steps](#steps)
      - [Run Step](#run-step)
      - [Build Step](#build-step)
      - [Workflow Step](#workflow-step)
    - [Background Containers](#background-containers)
    - [Environment & Secrets](#environment--secrets)
    - [Template Syntax](#template-syntax)

---

### Workflow Structure

Every workflow needs a `name`. Use either direct flow control or `jobs` for multiple entry points.

```yaml
name: My Workflow
id: my-workflow          # optional, must start with letter/underscore
schemaVersion: "0.1.0"   # optional, defaults to 0.1.0
description: My workflow description

env:
  NODE_ENV: production
  API_KEY:
    secret: true         # masked in output
    default: changeme    # optional default

# Choose ONE: direct flow OR jobs
sequence: [...]         # or parallel: [...] or switch: "..."
# OR
jobs:
  build: ...
  test: ...
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workflow name |
| `id` | string | No | Workflow identifier (must start with letter/underscore) |
| `schemaVersion` | string | No | Schema version, default: "0.1.0" |
| `description` | string | No | Human readable description |
| `env` | map | No | Environment variables |
| `jobs` | map | No | Named entry points |
| `sequence` | array | No | Sequential steps (direct flow) |
| `parallel` | array | No | Parallel steps (direct flow) |
| `switch` | string | No | Switch expression (direct flow) |
| `case` | map | No | Switch case branches |
| `default` | array/object | No | Switch default branch |

---

### Jobs

Jobs are named entry points. Run with `ocw <job-name>`.

```yaml
jobs:
  build:
    name: Build App           # optional display name
    description: Build image  # optional
    id: build-job             # optional step ID
    env:
      NODE_ENV: production
    sequence:
      - name: Install
        image: node:20
        cmd: npm ci

  # Shorthand for single-step jobs
  lint:
    image: node:20
    cmd: npm run lint
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Display name for the job |
| `description` | string | No | Job description |
| `id` | string | No | Job identifier |
| `env` | map | No | Environment variables for all steps |
| `sequence` | array | No | Run steps in sequence |
| `parallel` | array | No | Run steps in parallel |
| `switch` | string | No | Conditional branching |
| `case` | map | No | Switch cases |
| `default` | array/object | No | Switch default |

---

### Flow Control

#### sequence

Runs steps one after another. Fails fast on first error.

```yaml
sequence:
  - name: Install
    image: node:20
    cmd: npm ci
  - name: Test
    image: node:20
    cmd: npm test
```

#### parallel

Runs steps concurrently. Waits for all to complete.

```yaml
parallel:
  - name: Unit Tests
    image: node:20
    cmd: npm run test:unit
  - name: Integration Tests
    image: node:20
    cmd: npm run test:integration
```

#### switch

Conditional branching based on an expression.

```yaml
env:
  DEPLOY_ENV: staging

switch: "{{env.DEPLOY_ENV}}"
case:
  staging:
    - name: Deploy Staging
      image: kubectl
      cmd: kubectl apply -f staging.yaml
  production:
    - name: Deploy Production
      image: kubectl
      cmd: kubectl apply -f production.yaml
default:
  - name: Deploy Dev
    image: kubectl
    cmd: kubectl apply -f dev.yaml
```

#### Nesting

Flow controls can be nested for complex workflows.

---

### Steps

All steps share these common fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Step name |
| `id` | string | No | Step identifier (for referencing outputs) |
| `description` | string | No | Step description |
| `env` | map | No | Environment variables |
| `secrets` | map | No | Secrets (use env with `secret: true` instead) |
| `needs` | array | No | Service IDs to wait for before running |

#### Run Step

Runs a container. The workflow directory is mounted at `/workflow`.

```yaml
- name: Run Tests
  id: tests
  image: node:20-alpine     # required
  cmd: npm test              # command to run
  args: ["--coverage"]       # command arguments
  entrypoint: /bin/sh       # override entrypoint
  workdir: /workspace/app   # working directory (default: /workflow)

  # Background execution
  background: true
  healthCheck:
    cmd: pg_isready -U postgres
    interval: 2s
    timeout: 5s
    retries: 15
    startPeriod: 10s
  expose: 8080

  # Environment
  env:
    NODE_ENV: test
  envFile: .env.test        # or [.env, .env.local]

  # Resources
  cpus: 2
  memory: 2g
  gpus: 1                   # or "all"

  # Image handling
  pull: always              # always | missing | never
  platform: linux/amd64

  # Output control
  quiet: true
  tty: true
```

**Run Step Options:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | Yes | Container image |
| `cmd` | string | No | Command to run |
| `args` | array | No | Command arguments |
| `entrypoint` | string | No | Override entrypoint |
| `workdir` | string | No | Working directory |
| `background` | bool | No | Run in background |
| `healthCheck` | object | No | Health check config |
| `healthCheck.cmd` | string | Yes | Health check command |
| `healthCheck.interval` | string | No | Interval (e.g., "2s") |
| `healthCheck.timeout` | string | No | Timeout (e.g., "5s") |
| `healthCheck.retries` | int | No | Number of retries |
| `healthCheck.startPeriod` | string | No | Grace period (e.g., "10s") |
| `expose` | int/array/object | No | Port exposure |
| `env` | map/array | No | Environment variables |
| `envFile` | string/array | No | Env file(s) to load |
| `cpus` | int/string | No | CPU limit |
| `memory` | string | No | Memory limit (e.g., "512m", "2g") |
| `gpus` | int/string | No | GPU devices |
| `pull` | string | No | Pull policy: always, missing, never |
| `platform` | string | No | Target platform |
| `quiet` | bool | No | Suppress output |
| `tty` | bool | No | Allocate TTY |

#### Build Step

Builds a container image using Podman/Buildah.

```yaml
- name: Build Image
  build:
    # Core options
    image: myapp:latest       # required, primary tag
    context: /workspace         # build context (default: /workspace)

    # Dockerfile
    dockerfile: Dockerfile    # path to Dockerfile
    target: production        # multi-stage target

    # Build arguments
    buildArgs:
      NODE_ENV: production

    # Platform
    platform: linux/amd64     # or [linux/amd64, linux/arm64]

    # Caching
    cacheFrom: [myapp:cache]
    cacheTo: [type=registry, ref=myapp:cache]
    noCache: false
    noCacheFilter: [stage1]

    # Tags and output
    tags:
      - myapp:v1.0
      - myapp:latest
    output:
      type: docker
      dest: /tmp/image.tar

    # Push/Load
    push: true
    load: true

    # Base image
    pull: true

    # Secrets
    secrets:
      npm_token: "{{env.NPM_TOKEN}}"

    # Labels
    labels:
      version: "1.0"
    annotation:
      org.opencontainers.image.source: https://github.com/...

    # Resources
    shmSize: 128m
    ulimit:
      nofile: 1024:2048

    # Progress
    progress: plain           # auto | quiet | plain | tty | rawjson
    quiet: false

    # Attestations
    provenance: true          # or "min"
    sbom: true                # or "generator=syft"
    attest:
      - type=provenance,disabled=true

    # Metadata
    metadataFile: /tmp/meta.json
    iidfile: /tmp/image-id.txt

    # Additional contexts
    buildContext:
      alpine: docker-image://alpine:latest
```

**Build Step Options:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | Yes | Primary image tag |
| `context` | string | No | Build context path |
| `dockerfile` | string | No | Dockerfile path |
| `target` | string | No | Multi-stage target |
| `buildArgs` | map | No | Build arguments |
| `platform` | string/array | No | Target platform(s) |
| `cacheFrom` | array | No | Cache sources |
| `cacheTo` | array | No | Cache export destinations |
| `noCache` | bool | No | Disable cache |
| `noCacheFilter` | array | No | Disable cache for stages |
| `tags` | array | No | Additional tags |
| `output` | string/object | No | Output destination |
| `push` | bool | No | Push to registry |
| `load` | bool | No | Load into docker |
| `pull` | bool | No | Always pull base images |
| `secrets` | map/array | No | Build secrets |
| `labels` | map | No | Image labels |
| `annotation` | map | No | OCI annotations |
| `shmSize` | string | No | Shared memory size |
| `ulimit` | map | No | Resource limits |
| `progress` | string | No | Progress mode |
| `quiet` | bool | No | Suppress build output |
| `provenance` | bool/string | No | Provenance attestation |
| `sbom` | bool/string | No | SBOM attestation |
| `attest` | array | No | Custom attestations |
| `metadataFile` | string | No | Metadata output file |
| `iidfile` | string | No | Image ID file |
| `buildContext` | map | No | Additional build contexts |

#### Workflow Step

Invokes another workflow (local or remote).

```yaml
- name: Run Security Scan
  workflow:
    from: github.com/org/workflows/security@v1.0.0
    # or from: ./local/path.yaml
    env:
      SEVERITY: high
      SCAN_TARGET: /workspace
      TOKEN: "{{env.SCAN_TOKEN}}"
    inherit:
      env: all              # none | all
      secrets: none         # none | all
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | string | Yes | Workflow reference (path or URL) |
| `env` | map | No | Environment variables to pass |
| `inherit` | object | No | Inheritance settings |
| `inherit.env` | string | No | Inherit env: none, all |
| `inherit.secrets` | string | No | Inherit secrets: none, all |

---

### Background Containers

Run services (databases, caches) that persist across steps.

```yaml
sequence:
  - name: Start PostgreSQL
    id: postgres
    image: postgres:16
    background: true
    expose: 5432
    healthCheck:
      cmd: pg_isready -U postgres
      interval: 2s
      timeout: 5s
      retries: 15
      startPeriod: 10s
    env:
      POSTGRES_PASSWORD: secret

  - name: Run Migrations
    image: node:20
    cmd: npm run migrate
    needs: [postgres]       # wait for postgres to be healthy
    env:
      DATABASE_URL: postgres://postgres:secret@postgres:5432/db
```

**Port Exposure Formats:**

```yaml
expose: 8080                            # single port
expose: [8080, 9229]                    # multiple ports
expose:
  - containerPort: 3000
    hostPort: 80
    protocol: http                      # http | https | tcp | udp
```

Background services are accessible via their `id` as hostname (e.g., `postgres:5432`).

---

### Environment & Secrets

Environment variables cascade: workflow → job → step (later overrides earlier).

```yaml
env:
  # Plain value
  NODE_ENV: production

  # Secret with default
  DB_PASSWORD:
    secret: true
    default: changeme

  # Secret without default (must be in .env file)
  API_KEY:
    secret: true
```

**Secret Masking:**
- Secret values are masked as `[secret]` in output
- Use `--show-secrets` to reveal actual values
- Load env files with `-e filename.env`

---

### Template Syntax

Use `{{...}}` to reference dynamic values in almost any string field.

| Template | Description |
|----------|-------------|
| `{{env.VAR}}` | Environment variable |
| `{{steps.ID.output}}` | Output from previous step |
| `{{workflow.name}}` | Workflow name |
| `{{job.name}}` | Current job name |

**Example:**

```yaml
- name: Deploy
  image: myapp:{{env.TAG}}
  cmd: |
    echo "Deploying to {{env.ENVIRONMENT}}"
    curl -H "Authorization: Bearer {{env.API_TOKEN}}" \
      https://api.example.com/deploy
  env:
    VERSION: "{{env.TAG}}"
```
