# OCW Documentation

This directory contains the Hugo + Hextra documentation site for OCW.

## Local Development

### Prerequisites

- [Hugo](https://gohugo.io/installation/) (extended version, 0.129.0 or later)
- Go 1.25 or later

### Install Hugo

On macOS:
```bash
brew install hugo
```

For other platforms, see the [Hugo installation guide](https://gohugo.io/installation/).

### Run locally

```bash
cd docs
hugo server
```

Or with OCW:
```bash
cd docs
ocw dev
```

Open http://localhost:1313 in your browser.

### Build

```bash
cd docs
hugo --minify
```

The built site will be in `docs/public/`.

## Adding Content

Content lives in `docs/content/`. Create new Markdown files there. Each page should include front matter:

```yaml
---
title: "Page Title"
---
```

## Theme

This site uses the [Hextra](https://imfing.github.io/hextra/) theme, installed as a Hugo module.

## Deployment

The site is automatically deployed to GitHub Pages via [`.github/workflows/docs.yml`](https://github.com/uncloud-cc/ocw/blob/main/.github/workflows/docs.yml) when changes are pushed to the `main` branch.

Live site: https://uncloud-cc.github.io/ocw/
