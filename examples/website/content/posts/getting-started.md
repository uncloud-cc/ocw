---
title: "Getting Started with OCW Volume Mounts"
date: 2026-03-25T10:00:00+01:00
draft: false
tags: ["ocw", "tutorial", "volumes"]
author: "OCW Team"
---

This post demonstrates how OCW volume mounts work with a real static site generator.

## What Are Volume Mounts?

Volume mounts in OCW allow you to:

1. **Access host directories** from within containers
2. **Control access** with read-only (ro) or read-write (rw) modes
3. **Customize mount paths** for flexibility
4. **Inherit volumes** at the job level or specify per-step

## Example Configuration

```yaml
volumes:
  content:
    path: ./content
    mountPath: /src/content  # Default mount path
  
  dist:
    path: ./dist
    mode: rw                 # Read-write for output
    mountPath: /output

jobs:
  build:
    volumes:
      - content               # All steps get this
    sequence:
      - name: Build
        image: hugo:latest
        volumes:
          - dist              # Only this step gets write access
        cmd: hugo --destination /output
      
      - name: Verify
        volumes:
          - name: dist
            readonly: true   # More restrictive for safety!
        cmd: ls -la /output
```

## Why This Matters

By separating content (read-only) from output (read-write), you get:

- **Safety**: Build tools can't accidentally modify source files
- **Clarity**: Explicit volume grants make dependencies clear
- **Flexibility**: Different steps can have different access levels

Happy building! 🚀
