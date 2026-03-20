# Open Container Workflow (ocw) Basics

ocw is a simple to use workflow engine that lets you build workflows out of containers.

ocw workflows essentially do just two things: **Build containers** and **run containers** - that's pretty much it. Everything else around it is just to make your life easier.

ocw is also designed for security with _immutable workspaces_, _outgoing network filters_, is configured to be _jailbreak-proof_, runs _rootless_ and it built on top of [Podman](https://podman.io/) which is an open-source container runtime built with security in mind.

> **At the moment ocw is still in early stages** and many of the security features aren't implemented yet! Checkout the [ocw roadmap](https://github.com/orgs/opencontainerworkflow/projects/1) for details.

## Setup

Before we jump in, you need to install three things on your local machine to get started:

- [Podman](https://podman.io/docs/installation) it's an open-source container runtime that's secure by design
- [Go](https://go.dev/dl/) for being able to run the `ocw cli`
- [ocw cli](../../README.md#install) for running ocw workflows → Just run `go install github.com/opencontainerworkflow/ocw/cmd`

## Hello World!

**Now let's get to the good stuff!** Go ahead and create your first ocw workflow file - they're simple YAML files:

```yaml
# hello-world.yaml
name: Hello World!
sequence:
  - name: hello
    image: alpine:latest
    cmd: echo "Hello OCW World! 🎉"
```

Go ahead and run it: `ocw hello-world.yaml` - you should see something like this 👇🏻

```bash
────────────────────────────────────────────────────────────
  Hello World!
────────────────────────────────────────────────────────────
  Directory: /Users/dev/ocw/tutorials/basics

>>> Running 1 steps in sequence

▶ hello [run]
  Image: alpine:latest
  Image exists: alpine:latest
  │ Hello OCW World! 🎉
✓ hello completed

────────────────────────────────────────────────────────────
  ✓ Hello World! completed successfully (329ms)
────────────────────────────────────────────────────────────
```

Congrats! You just created and ran your first ocw workflow ✨

Workflows can run steps as a `sequence`, in `parallel` or based on conditions using `switch / case`. And yes you can nest these as needed.

But we're getting ahead of ourselves - let's build a container next!

> **Example not working?** Double check that you added podman & go to your `$PATH` by checking your `~/.bashrc` or `~/.zshrc` for options similar to this:
>
> ```bash
> # Your ~/.bashrc or ~/.zshrc should contain something like this:
> # Podman
> export PATH="/opt/podman/bin:$PATH"
>
> # Go
> export PATH="$PATH:$HOME/go/bin"
> ```
>
> You might have to run `source ~/.bashrc` or `source ~/.zshrc` again or restart your terminal. You know they are setup correctly when `go version` and `podman version` output correct values.

## Building a container

Another key element of ocw is to _build_ containers. Let's first create a simple Dockerfile - it will just output "Hello World" again:

```Dockerfile
FROM alpine:latest
ENV NAME=World
CMD ["sh", "-c", "echo Hello $NAME ✨"]
```

Now go ahead and create your second ocw workflow - this will build our container:

```yaml
# build.yaml
name: Building a container
sequence:
  - name: Build the container
    build:
      image: my-first-ocw-container
      dockerfile: Dockerfile
```

Run the workflow: `ocw build.yaml` - look for this line confirming it all worked:

```bash
Successfully tagged localhost/my-first-ocw-container:latest
```

You can now run the container locally (we'll run it by hand first and as part of a ocw workflow next!):

```bash
podman run --rm my-first-ocw-container
# Hello World ✨
```

## Building & Running containers

As a last step, let's see the the two steps together in action - building & running containers.

We'll reuse the Dockerfile from the previous step, but this time we're running it inside of our workflow:

```yaml
# build-and-run.yaml
name: Build & run containers
sequence:
  - name: Build the container
    id: build
    build:
      image: my-first-ocw-container
      dockerfile: Dockerfile

  - name: Run the container
    image: "{{ steps.build.image }}"
    env:
      NAME: ocw 🤖
```

Go ahead and run it 👉🏻 `ocw build-and-run.yaml`.

Couple of new things to notice here: We added an `id` to the build-step, allowing us to reference it in the next step using `{{ steps.build.image }}` (more on templating in the advanced tutorial).

Notice how we're also setting an environment variable for this container using `env`. Go ahead and set a different `NAME` and rerun the example!

## What's Next?

Good job, working through the basics. Next, go to the [advanced tutorial](../advanced/README.md) to learn about:

- Templating
- Parallel steps
- Conditionals (switch / case)
- Building reusable jobs
- Exposing containers (for local development)
- And more!

**Got feedback?**\
[Join the community on Github](https://github.com/opencontainerworkflow/ocw/discussions) to ask questions, get help and share feedback.
