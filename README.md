# Open Container Workflow (ocw)

Container-native CI/CD workflows that (actually) run locally.

> ocw is currently in a pre-production state (see [launch roadmap](https://github.com/orgs/uncloud-cc/projects/6/views/1)). Feel free to test it out and share your feedback 🎉

## Why?

Github Actions, Gitlab piplines, CircleCI jobs - they all lack one thing: It's ~~hard~~ impossible to fully run them locally.

At the same time, many of the CI/CD workflows you rely on in the cloud, are also needed in your local development workflow (building your apps, running tests, etc.).

ocw does both: Through a simple YAML syntax, it's easy to build local development workflows that you can also run inside Github Actions as your CI/CD pipelines.

## Examples
**Run a local development server**
```javascript
// server.js
require('http').createServer((_, res) => {
  res.end(`Hello world!`);
}).listen(8888);
```

```yaml
# hello.yaml
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

**Test & Build a container in Github Actions**
```yaml
# ci.yaml
env:
  DOCKER_USERNAME:
  DOCKER_PASSWORD:
    secret: true

sequence:
  - name: Run tests
    image: node:25-alpine
    cmd: npm test

  - name: Get git hash & branch
    id: git
    image: alpine:latest
    cmd: |
      echo "hash=$(git rev-parse --short HEAD)" >> $OUTPUTS
      echo "branch=$(git branch --show-current)" >> $OUTPUTS

  - name: Build & Push image
    build:
      image: example/myapp
      registry: docker.io
      tags:
        - "{{ steps.git.branch }}"
        - "{{ steps.git.hash }}"
      dockerfile: Dockerfile
      # 🚧 Pushing images doesn't work yet
      push: true
      env:
        DOCKER_USERNAME: {{ secrets.DOCKER_USERNAME }}
        DOCKER_PASSWORD: {{ secrets.DOCKER_PASSWORD }}
```

Now run your ocw workflow in Github Actions for Pull Requests and new commits in the `main` branch:

```yaml
# .github/workflows/ci.yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Test & Build with OCW
        uses: uncloud-cc/ocw@main
        with:
          file: 'ci.yaml'
        env:
          DOCKER_USERNAME: ${{ secrets.DOCKER_USERNAME }}
          DOCKER_PASSWORD: ${{ secrets.DOCKER_PASSWORD }}
```

Notice how you didn't even need to `checkout` the code - that's part of the Github Action. See [Github Action docs](/docs/GithubAction.md) for details.

## Install
```bash
go install github.com/uncloud-cc/ocw/cmd/ocw@latest
```

Check if it worked by running `ocw --help`

## `uncloud-cc/ocw` Github Action
You can run ocw workflows inside Github Actions with our custom-action. Checkout the [Github Actions docs](/docs/GithubAction.md) for more.

## Getting started

- Go through the [basics tutorial](./tutorials/basics.md)
- Check out advanced features in the [advanced tutorial](./tutorials/advanced.md)
- Or check out the [examples](./examples/)

## Feedback

[Join the community](https://github.com/uncloud-cc/ocw/discussions) to ask questions, get help and share feedback.

## License

MIT License - see [LICENSE](LICENSE) file.
