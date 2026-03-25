---
title: "Welcome to OCW Demo"
description: "A demo site showcasing volume mounts"
---

Welcome to this demonstration site built with **OCW (Open Container Workflow)**!

This site showcases the power of volume mounts in OCW workflows:

- **Read-only volumes** for source content (your Markdown files)
- **Read-write volumes** for build output (the generated HTML)
- **Custom mount paths** to organize your workflow
- **Mode restrictions** for safety (making volumes more restrictive)

## How It Works

The `17_hugo.yaml` workflow orchestrates the entire process:

1. **Dev**: Runs _this_ dev server (try changing files!)
2. **Build**: Uses Hugo to generate a static site from Markdown
3. **Verify**: Lists all generated files (mounted read-only for safety)
4. **Deploy**: Syncs the output to S3 (if configured)

## Directory Structure

```
./
├── content/          # Your Markdown content (mounted read-only)
├── site/             # Hugo config & templates (mounted read-only)
└── dist/             # Generated site (mounted read-write)
```

Check out the [posts](/posts/) section for more examples!
