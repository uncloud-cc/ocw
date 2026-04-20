# Rewrite using TDAID
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
