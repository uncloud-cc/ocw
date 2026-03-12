# OCW Build Quick Reference

## Path Resolution

All paths are relative to `/workflow` (your workflow root).

| You Write | OCW Understands |
|-----------|-----------------|
| `context: /workflow/dir` | Workflow root + `dir/` |
| `context: ./dir` | Workflow root + `dir/` |
| `context: dir` | Workflow root + `dir/` |
| `dockerfile: dir/Dockerfile` | Workflow root + `dir/Dockerfile` |
| (no context) | Defaults to `/workflow` (workflow root) |

## Two Ways to Access Files

### COPY/ADD (from build context)

```dockerfile
# Only sees files in the build context directory
COPY package.json ./
COPY src/ ./src/
```

- ✅ Fast, well-cached
- ✅ Standard Docker practice
- ❌ Limited to build context
- ❌ Can't see parent directories

### RUN (with /workflow mount)

```dockerfile
# Can access ANY file in workflow directory
RUN cp /workflow/scripts/setup.sh /usr/local/bin/
RUN cp /workflow/config/app.conf /etc/
```

- ✅ Full workflow access
- ✅ Can access any subdirectory
- ✅ Run scripts from anywhere
- ❌ Less granular caching

## Common Patterns

### Pattern: Subdirectory Context

```yaml
build:
  dockerfile: backend/Dockerfile
  context: /workflow/backend
```

```dockerfile
# COPY relative to backend/
COPY package.json ./
```

### Pattern: Root Context

```yaml
build:
  dockerfile: Dockerfiles/Dockerfile
  # context defaults to /workflow
```

```dockerfile
# COPY relative to workflow root
COPY backend/src/ /app/
COPY frontend/dist/ /static/
```

### Pattern: Mix Both Approaches

```yaml
build:
  dockerfile: app/Dockerfile
  context: /workflow/app
```

```dockerfile
# COPY from context (app/)
COPY . /app/

# RUN accesses full workflow
RUN cp /workflow/scripts/entrypoint.sh /app/
RUN /workflow/scripts/generate-config.sh > /app/config.json
```

## Quick Decision Tree

```
Need to copy files?
├─ Files in one directory?
│  └─ Use COPY with context set to that directory
│
├─ Files from multiple directories?
│  ├─ All under one parent?
│  │  └─ Use COPY with context set to parent directory
│  └─ Scattered across workflow?
│     └─ Use RUN with /workflow mount
│
└─ Need to run scripts?
   └─ Use RUN with /workflow mount
```

## Common Issues

| Error | Cause | Fix |
|-------|-------|-----|
| `COPY file.txt: no such file` | File outside context | Change context or use RUN |
| `COPY /workflow/file: no such file` | Using /workflow in COPY | Use RUN instead or adjust context |
| `dockerfile not found` | Wrong dockerfile path | Path should be relative to workflow root |

## Examples

### ✅ Good

```yaml
# Clear, concise, uses relative path
build:
  dockerfile: backend/Dockerfile
  context: ./backend
```

```yaml
# Accesses multiple directories
build:
  dockerfile: Dockerfiles/app.Dockerfile
  context: /workflow
```

```yaml
# Uses /workflow for scripts
build:
  dockerfile: api/Dockerfile
  context: /workflow/api
# Dockerfile can: RUN /workflow/scripts/setup.sh
```

### ❌ Avoid

```yaml
# Don't use absolute host paths
build:
  dockerfile: /Users/jonas/project/Dockerfile  # ❌
  context: /Users/jonas/project/src/           # ❌
```

```dockerfile
# Don't use /workflow in COPY
COPY /workflow/file.txt /app/  # ❌ Won't work

# Do this instead:
RUN cp /workflow/file.txt /app/  # ✅
```
