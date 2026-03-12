# OCW Build Context Examples

## Understanding Paths in OCW

In OCW, **all paths are conceptually relative to `/workflow`** (the workflow root, where your `.yaml` file is located).

### Key Concepts

1. **Workflow Root**: The directory containing your `.yaml` workflow file
2. **Build Context**: The directory that COPY/ADD instructions can access (set via `context`)
3. **`/workflow` Mount**: The workflow root mounted during RUN commands (always available)

## Path Resolution Examples

Assume your workflow file is at: `/Users/jonas/project/build.yaml`

| Path in YAML | Resolves To |
|--------------|-------------|
| `context: /workflow` | `/Users/jonas/project/` |
| `context: /workflow/backend` | `/Users/jonas/project/backend/` |
| `context: ./backend` | `/Users/jonas/project/backend/` |
| `context: backend` | `/Users/jonas/project/backend/` |
| `dockerfile: Dockerfiles/Dockerfile` | `/Users/jonas/project/Dockerfiles/Dockerfile` |
| `dockerfile: /workflow/Dockerfiles/Dockerfile` | `/Users/jonas/project/Dockerfiles/Dockerfile` |
| `dockerfile: ./Dockerfiles/Dockerfile` | `/Users/jonas/project/Dockerfiles/Dockerfile` |

## Pattern 1: Subdirectory Context with COPY

**Use when**: Your Dockerfile and related files are in a subdirectory

```yaml
# build.yaml
sequence:
  - name: Build API
    build:
      image: myapp/api
      dockerfile: backend/Dockerfile
      context: /workflow/backend  # or: ./backend or: backend
```

```dockerfile
# backend/Dockerfile
FROM node:20

# COPY is relative to context (/workflow/backend)
COPY package.json package-lock.json ./
RUN npm ci

COPY src/ ./src/
COPY tsconfig.json ./

RUN npm run build
```

**Directory Structure:**
```
project/
├── build.yaml
└── backend/
    ├── Dockerfile
    ├── package.json
    ├── src/
    └── tsconfig.json
```

## Pattern 2: Workflow Root Context

**Use when**: You want to access files from multiple directories

```yaml
# build.yaml
sequence:
  - name: Build with root context
    build:
      image: myapp/combined
      dockerfile: Dockerfiles/combined.Dockerfile
      context: /workflow  # or omit (this is default)
```

```dockerfile
# Dockerfiles/combined.Dockerfile
FROM nginx:alpine

# COPY is relative to workflow root
COPY frontend/dist/ /usr/share/nginx/html/
COPY config/nginx.conf /etc/nginx/nginx.conf
COPY docs/README.md /usr/share/nginx/html/docs/
```

**Directory Structure:**
```
project/
├── build.yaml
├── Dockerfiles/
│   └── combined.Dockerfile
├── frontend/
│   └── dist/
├── config/
│   └── nginx.conf
└── docs/
    └── README.md
```

## Pattern 3: Using /workflow Mount

**Use when**: You need maximum flexibility or want to access files outside the build context

```yaml
# build.yaml
sequence:
  - name: Build with /workflow mount
    build:
      image: myapp/flexible
      dockerfile: Dockerfiles/flexible.Dockerfile
      context: /workflow/Dockerfiles  # Narrow context
```

```dockerfile
# Dockerfiles/flexible.Dockerfile
FROM alpine:latest

# COPY only works within context (Dockerfiles/)
COPY setup.sh /tmp/

# RUN can access anything in /workflow
RUN cp /workflow/config/app.conf /etc/app.conf
RUN cp /workflow/scripts/init.sh /usr/local/bin/
RUN /workflow/scripts/generate-config.sh > /etc/generated.conf

# Access multiple directories
RUN cat /workflow/templates/*.txt > /app/combined.txt

# Conditionally copy files
RUN if [ -f /workflow/secrets/.env ]; then \
      cp /workflow/secrets/.env /app/.env; \
    fi
```

**Directory Structure:**
```
project/
├── build.yaml
├── Dockerfiles/
│   ├── flexible.Dockerfile
│   └── setup.sh
├── config/
│   └── app.conf
├── scripts/
│   ├── init.sh
│   └── generate-config.sh
├── templates/
│   ├── header.txt
│   └── footer.txt
└── secrets/
    └── .env
```

## Pattern 4: Multi-Stage Build with Different Contexts

**Use when**: Building complex applications with multiple components

```yaml
# build.yaml
sequence:
  - name: Build images
    parallel:
      - name: Build frontend
        build:
          image: myapp/frontend
          dockerfile: frontend/Dockerfile
          context: /workflow/frontend
      
      - name: Build backend
        build:
          image: myapp/backend
          dockerfile: backend/Dockerfile
          context: /workflow/backend
      
      - name: Build final image
        build:
          image: myapp/full
          dockerfile: Dockerfiles/combined.Dockerfile
          context: /workflow
```

```dockerfile
# Dockerfiles/combined.Dockerfile
FROM scratch

# Use /workflow mount to assemble from different parts
RUN --mount=type=bind,source=/,target=/workflow \
    cp -r /workflow/frontend/dist /app/frontend && \
    cp -r /workflow/backend/build /app/backend && \
    cp /workflow/config/* /app/config/
```

## Best Practices

### ✅ Do

- Use relative paths (`./backend`, `Dockerfiles/...`) for readability
- Set `context` to the narrowest directory that contains what you need
- Use COPY for standard file copying (better caching)
- Use `/workflow` mount when you need cross-directory access
- Keep Dockerfiles near the code they build

### ❌ Don't

- Mix absolute host paths (like `/Users/...`) - always use `/workflow` or relative paths
- Copy the entire workflow root if you only need a subdirectory (bad for caching)
- Forget that COPY can't see outside the build context
- Use `/workflow` mount in COPY instructions (it won't work)

## Troubleshooting

### "No such file or directory" with COPY

**Problem**: `COPY file.txt /app/` fails

**Solution**: Check that:
1. `file.txt` is within your build context
2. The path is relative to the build context, not the workflow root
3. If the file is outside the context, use RUN with `/workflow` mount instead

### "COPY failed: stat /workflow/file.txt: no such file or directory"

**Problem**: Trying to use `/workflow` in COPY instruction

**Solution**: COPY doesn't see the `/workflow` mount. Either:
1. Adjust your `context` to include the file
2. Use `RUN cp /workflow/file.txt /dest/` instead

### Dockerfile path not found

**Problem**: OCW can't find your Dockerfile

**Solution**: The `dockerfile` path should be relative to the workflow root:
- ✅ `dockerfile: backend/Dockerfile`
- ✅ `dockerfile: /workflow/backend/Dockerfile`
- ❌ `dockerfile: Dockerfile` (when Dockerfile is in a subdirectory)
