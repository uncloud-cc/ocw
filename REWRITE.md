# TDD based rewrite
> It doesn't have to be perfect. Just good enough to use in production.

Comprehensive rewrite of the first version of ocw.

## TODOs
- [x] Implement basic ability to run and build containers
- [x] Implement build step outputs
- [x] Implement run step outputs
- [x] Implement `/workflow` mount
- [ ] Implement volume mounts
- [ ] Implement service (background) containers
- [ ] Implement imported workflows
- [ ] Implement Github Action

**Out of scope for v1**
- [ ] Step by step debugger
- [ ] Debugging containers
- [ ] Reloading containers on file changes
- [ ] Observing filesystem changes (to audit workflows)
- [ ] Outgoing network firewall
- [ ] Configs / Middleware / Plugins

## Notes
- "env" doesn't make any sense for the workflow inputs since they shadow the real env vars that exist inside each container 👉🏻 Instead we'll do inputs which can be defined and can be defined as plain-text inputs available via `{{ input.<name> }}` or via `{{ secret.<name> }}`
- The "printer" is a first UI for the NDJSON event-stream that comes out of the ocw CLI - the source of truth here is NDJSON (the pretty-printer is documentation for that event stream format + an example UI implementation) (extensibility of ocw for now is limited to custom UIs that can launch ocw as a child-process and then pretty-print the --json view) 👉🏻 BTW: The JSON output is a great way to e2e test ocw!
- Down the line, the extensibility of ocw might be extended to be able to run request / response hooks and to be able to interact with a running instance of ocw directly through some sort of API that runs on the localhost 👉🏻 In that case it could also make sense though to introduce contexts whereby users can login their ocw instance into any service provider in the background
  - **For now the extensibility is provided through the library layer of `ocw`** - Keep in mind that the entire core functionality of `ocw` is made available as a library. Like that I can use `ocw` to load & parse a file, then set custom data before I compile and run it 👉🏻 That way I can already control secrets handling etc.

---------
- [x] Basic structure (cleanup schema, add parsing files to OCW)

**Workflow engine**
- [ ] Basic workflow engine that can run an OCW workflow with a dummy implementation
  - Should turn OCW into a flat "workflow" structure
  - The flat workflow structure is essentially a linear collection of steps - some steps are individual steps, some are an array of steps
  - Steps should be actually swappable -> Yes we're using ocw now to build and run containers - but who knows? Might be different steps down the line
  - Workflow just coordinates which steps go out next, prepares input and merges output
- [ ] Workflow engine that can launch services ("background" containers)
- [ ] Workflow engine is done when it can run a simple workflow + setup, healthcheck and teardown services - all with a dummy implementation (no docker yet)

👉🏻 Should be simple enough & yet abstracted enough that we can use the same thing later to build a step-by-step debugger

**Docker implementation**
- [ ] Add to the dummy workflow executioner (or whatever) that we used in the previous step a Docker implementation

**File watcher**
- [ ] Implement the file watcher thing

**CLI implementation**
- [ ] Implement CLI which parses a workflow file, turns it into a workflow, executes it (based on its args) and can watch files to automatically reload containers
