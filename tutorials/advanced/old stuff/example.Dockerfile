FROM nginx:alpine

# ============================================================================
# OCW Build Context & /workflow Mount Guide
# ============================================================================
#
# In OCW, all paths are relative to the workflow root (where your .yaml file is)
# You can think of yourself as being "inside /workflow" when writing paths.
#
# Two ways to access files during build:
#
# 1. COPY/ADD: Uses the build context (set via 'context' parameter)
#    - COPY only sees files within the build context directory
#    - Optimized for layer caching (each file change invalidates only that layer)
#
# 2. RUN: Can access the full /workflow mount
#    - /workflow is always mounted at the workflow root (read-only)
#    - Access ANY file in your workflow directory during RUN commands
#
# ============================================================================

# ----------------------------------------------------------------------------
# Example A: Using COPY with subdirectory context
# ----------------------------------------------------------------------------
# Workflow YAML:
#   context: /workflow/Dockerfiles   (or: ./Dockerfiles or just: Dockerfiles)
#   dockerfile: Dockerfiles/Dockerfile
#
# Build context is: <workflow_root>/Dockerfiles/
# COPY paths are relative to: Dockerfiles/

# COPY website.html /usr/share/nginx/html/index.html
# COPY server.js /app/server.js
# COPY ../README.md /docs/  # Can go up but stays within workflow

# ----------------------------------------------------------------------------
# Example B: Using COPY with workflow root context (default)
# ----------------------------------------------------------------------------
# Workflow YAML:
#   context: /workflow   (or omit context entirely - this is the default)
#   dockerfile: Dockerfiles/Dockerfile
#
# Build context is: <workflow_root>/
# COPY paths are relative to: workflow root

# COPY Dockerfiles/website.html /usr/share/nginx/html/index.html
# COPY README.md /docs/
# COPY scripts/setup.sh /usr/local/bin/

# ----------------------------------------------------------------------------
# Example C: Using RUN with /workflow mount
# ----------------------------------------------------------------------------
# Works regardless of build context!
# /workflow always points to the workflow root

# Copy from any subdirectory
RUN cp /workflow/Dockerfiles/website.html /usr/share/nginx/html/index.html

# Access files from anywhere in the workflow
RUN cp /workflow/README.md /usr/share/nginx/html/
RUN cp /workflow/old\ stuff/config.json /etc/config.json

# Run scripts from the workflow
RUN chmod +x /workflow/scripts/setup.sh && /workflow/scripts/setup.sh

# Process files before copying
RUN cat /workflow/config/*.conf > /etc/combined.conf

# ----------------------------------------------------------------------------
# When to use each approach:
# ----------------------------------------------------------------------------
# Use COPY when:
#   ✓ Files are within or near the build context
#   ✓ You want optimal layer caching (per-file granularity)
#   ✓ Following standard Docker best practices
#   ✓ Building standard containerized applications
#
# Use RUN with /workflow when:
#   ✓ You need files from multiple directories
#   ✓ You need to run scripts from anywhere in the workflow
#   ✓ You want dynamic file selection (wildcards, conditionals)
#   ✓ You need to process/transform files before copying
#   ✓ You're building from a narrow context but need workflow-wide access
