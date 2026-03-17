# Open Container Workflow (ocw)

Container-native workflows for local development & CI/CD.

## `<RemoveBeforeRelease`>
**Out of scope for now:**
- `inputs` >> `env` and `secrets` are enough - we can add validation options later to automatically validate `env / secrets`
- `secrets` being set inline inside the files (secrets are just configured so we know which values to mask in logs but are passed in via `.env` or the implementing platform)
- Invoking other workflows / importing them from Github >> Waaaaayyyy advanced right now!

## Install

Make sure you have these two installed:

- [Go](https://go.dev/dl/) `v1.24`
- [Podman](https://podman.io/docs/installation) `v5.7`

Then install the `ocw` CLI: `go install github.com/opencontainerworkflow/ocw/cmd/ocw@<commit-hash>`

Check if it worked by running `ocw --help`

## Getting started

- Go through the [basics tutorial](./tutorials/basics/README.md)
- Check out advanced features in the [advanced tutorial](./tutorials/advanced/README.md)
- Or check out the [examples](./examples/)

## Feedback

[Join the community](https://github.com/opencontainerworkflow/ocw/discussions) to ask questions, get help and share feedback.

## Reference

> 🤖 First robot draft - needs human refinement

- [Open Container Workflow (ocw)](#open-container-workflow-ocw)
  - [Install](#install)
  - [Getting started](#getting-started)
  - [Feedback](#feedback)
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
    - [Inputs](#inputs)
    - [Environment \& Secrets](#environment--secrets)
    - [Template Syntax](#template-syntax)

---

### Workflow Structure

Every workflow requires `schemaVersion` and `name`. Use either direct flow control (`sequence`/`parallel`/`switch`) or `jobs` for multiple entry points.

```yaml
schemaVersion: "0.1.0"
name: My Workflow
description: Optional description # optional

env: # workflow-level environment variables
  NODE_ENV: production

secrets: # encrypted secrets
  API_KEY:
    secure: v1:encrypted...

inputs: # parameterize your workflow
  tag:
    type: string
    default: latest

# Choose ONE: direct flow OR jobs
sequence: [...] # or parallel: [...] or switch: "..."
# OR
jobs:
  build: ...
  test: ...
```

---

### Jobs

Jobs are named entry points. Run with `ocw <job-name>`.

```yaml
jobs:
  build:
    name: Build App # optional display name
    description: Build image # optional
    sequence:
      - name: Build
        build:
          image: myapp:latest

  test:
    parallel:
      - name: Unit Tests
        image: node:20
        cmd: npm test

  # Single-step job (shorthand)
  lint:
    image: node:20
    cmd: npm run lint
```

```bash
ocw build    # runs build job
ocw test     # runs test job
ocw          # lists available jobs
```

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
inputs:
  environment:
    type: choice
    options: [staging, production]

switch: "{{inputs.environment}}"
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

Flow controls can be nested:

```yaml
sequence:
  - name: Build
    image: node:20
    cmd: npm run build

  - name: Test in Parallel
    parallel:
      - name: Unit
        image: node:20
        cmd: npm run test:unit

      - name: E2E
        image: playwright
        cmd: npm run test:e2e
```

---

### Steps

#### Run Step

Runs a container. The workflow directory is mounted at `/workflow`.

```yaml
- name: Run Tests # required
  id: tests # optional, for referencing
  description: Run unit tests
  image: node:20-alpine # required

  # Command
  cmd: npm test # shell command
  args: ["--coverage"] # command arguments
  entrypoint: /bin/sh # override entrypoint
  workdir: /workflow/app # working directory

  # Environment
  env:
    NODE_ENV: test
  envFile: .env.test # or array: [.env, .env.local]

  # Resources
  memory: "2g"
  cpus: 2
  gpus: all # or number

  # Image handling
  pull: always # always | missing | never
  platform: linux/amd64

  # Output
  quiet: true # suppress pull output
  tty: true # allocate TTY (colored output)
```

#### Build Step

Builds a container image using Buildah/Podman.

```yaml
- name: Build Image
  build:
    image: myapp:latest # required
    context: /workflow # build context (default: /workflow)
    dockerfile: Dockerfile # path to Dockerfile
    target: production # multi-stage target

    # Tags
    tags:
      - myapp:v1.0
      - myapp:latest

    # Build args
    buildArgs:
      NODE_ENV: production

    # Multi-platform
    platform:
      - linux/amd64
      - linux/arm64

    # Caching
    cacheFrom: [myapp:cache]
    cacheTo: [type=registry, ref=myapp:cache]
    noCache: false

    # Push/Load
    push: true # push to registry
    load: true # load into local images

    # Secrets (available as /run/secrets/<id> in Dockerfile)
    secrets:
      npm_token: "{{secrets.NPM_TOKEN}}"

    # Attestations
    provenance: true
    sbom: true

    # Progress
    progress: plain # auto | quiet | plain | tty
```

#### Workflow Step

Invokes another workflow (local or remote).

```yaml
- name: Run Security Scan
  workflow:
    from: github.com/org/workflows/security@v1.0.0 # or ./local/path.yaml
    inputs:
      severity: high
    env:
      SCAN_TARGET: /workflow
    secrets:
      TOKEN: "{{secrets.SCAN_TOKEN}}"
    inherit:
      secrets: all # all | none
      env: none
```

---

### Background Containers

Run services (databases, caches) that persist across steps.

```yaml
sequence:
  - name: Start PostgreSQL
    id: postgres # required for service discovery
    image: postgres:16
    background: true # runs in background
    expose: 5432 # expose port to host
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
    needs: [postgres] # wait for postgres to be healthy
    env:
      DATABASE_URL: postgres://postgres:secret@postgres:5432/db
```

**Expose formats:**

```yaml
expose: 8080                           # single port
expose: [8080, 9229]                   # multiple ports
expose:
  - containerPort: 3000
    hostPort: 80
    protocol: http                     # http | https | tcp | udp
```

Steps reference background services by their `id` as hostname (e.g., `postgres:5432`).

---

### Inputs

Define parameters for your workflow. Pass via `-i file.yaml` or `--input key=value`.

```yaml
inputs:
  # String input
  tag:
    type: string # default type if omitted
    description: Image tag
    default: latest
    required: false
    pattern: "^v[0-9]+" # regex validation
    minLength: 1
    maxLength: 50

  # Number input
  replicas:
    type: number
    description: Number of replicas
    default: 3
    min: 1
    max: 10

  # Boolean input
  dryRun:
    type: boolean
    description: Perform dry run
    default: false

  # Choice input
  environment:
    type: choice
    description: Deploy target
    options:
      - development
      - staging
      - production
    required: true
```

Use in templates: `{{inputs.tag}}`, `{{inputs.replicas}}`

---

### Environment & Secrets

**Environment variables** cascade: workflow → job → step (later overrides earlier).

```yaml
env:
  NODE_ENV: production
  LOG_LEVEL: info

jobs:
  test:
    sequence:
      - name: Test
        image: node:20
        env:
          NODE_ENV: test # overrides workflow-level
```

**Secrets** are encrypted values. Reference with `{{secrets.NAME}}`.

```yaml
secrets:
  # Encrypted (recommended)
  API_KEY:
    secure: v1:YijggfkQPZi...:WxCkaqkNDjqJ2A5t...

  # Plain (auto-encrypted by platforms)
  DB_PASS: mypassword

sequence:
  - name: Deploy
    image: kubectl
    cmd: kubectl apply -f -
    env:
      API_KEY: "{{secrets.API_KEY}}"
```

---

### Template Syntax

Use `{{...}}` to reference dynamic values:

| Syntax               | Description               |
| -------------------- | ------------------------- |
| `{{inputs.name}}`    | Input value               |
| `{{secrets.NAME}}`   | Secret value              |
| `{{env.VAR}}`        | Environment variable      |
| `{{step_id.output}}` | Output from previous step |

Templates work in: `cmd`, `env` values, `image`, build `tags`, and most string fields.

```yaml
- name: Deploy
  image: myapp:{{inputs.tag}}
  cmd: |
    echo "Deploying to {{inputs.environment}}"
    curl -H "Authorization: Bearer {{secrets.TOKEN}}" \
      https://api.example.com/deploy
  env:
    VERSION: "{{inputs.tag}}"
```
