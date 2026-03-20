# Open Container Workflow (ocw)

Container-native workflows for local development & CI/CD.

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

Every workflow needs a `name`. Use either direct flow control (`sequence`/`parallel`/`switch`) or `jobs` for multiple entry points.

```yaml
name: My Workflow

env:
  NODE_ENV: production
  API_KEY:
    secret: true       # masked in output

inputs:
  tag:
    default: latest

# Choose ONE: direct flow OR jobs
sequence: [...]        # or parallel: [...] or switch: "..."
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
- name: Run Tests
  id: tests              # optional, for referencing outputs
  image: node:20-alpine
  cmd: npm test
  workdir: /workflow/app # default: /workflow
  env:
    NODE_ENV: test
```

Additional options: `args`, `entrypoint`, `envFile`, `memory`, `cpus`, `gpus`, `pull`, `platform`, `quiet`, `tty`.

#### Build Step

Builds a container image using Podman/Buildah.

```yaml
- name: Build Image
  build:
    image: myapp:latest
    dockerfile: Dockerfile    # default: Dockerfile
    context: /workflow        # default: /workflow
    target: production        # multi-stage target
    buildArgs:
      NODE_ENV: production
```

Additional options: `tags`, `platform`, `cacheFrom`, `cacheTo`, `noCache`, `push`, `load`, `secrets`, `provenance`, `sbom`, `progress`.

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
      TOKEN: "{{env.SCAN_TOKEN}}"
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

Define parameters for your workflow. Pass via `--input key=value`.

```yaml
inputs:
  tag:
    type: string          # string | number | boolean | choice
    default: latest
    description: Image tag

  environment:
    type: choice
    options: [dev, staging, production]
    required: true
```

Use in templates: `{{inputs.tag}}`, `{{inputs.environment}}`

Additional options: `pattern`, `minLength`, `maxLength`, `min`, `max`.

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

**Secrets** are env vars marked with `secret: true`. They're masked as `[secret]` in output.

```yaml
env:
  APP_NAME: my-app              # Regular env var

  DB_PASSWORD:
    secret: true                # Masked in output
    default: changeme           # Optional default

  API_KEY:
    secret: true                # No default - must be in .env file

sequence:
  - name: Deploy
    image: kubectl
    cmd: kubectl apply -f -
    env:
      API_KEY: "{{env.API_KEY}}"
```

Use `--show-secrets` to reveal actual values in output. Load env files with `-e filename.env`.

---

### Template Syntax

Use `{{...}}` to reference dynamic values:

| Syntax                | Description               |
| --------------------- | ------------------------- |
| `{{inputs.name}}`     | Input value               |
| `{{env.VAR}}`         | Environment variable      |
| `{{steps.id.output}}` | Output from previous step |
| `{{workflow.name}}`   | Workflow metadata         |

Templates work in: `cmd`, `env` values, `image`, build `tags`, and most string fields.

```yaml
- name: Deploy
  image: myapp:{{inputs.tag}}
  cmd: |
    echo "Deploying to {{inputs.environment}}"
    curl -H "Authorization: Bearer {{env.API_TOKEN}}" \
      https://api.example.com/deploy
  env:
    VERSION: "{{inputs.tag}}"
```
