package workflow

// -----------------------------------------------------------------------------
// Step Context
// -----------------------------------------------------------------------------

// StepContext holds all runtime state for step execution and template resolution.
type StepContext struct {
	// Template resolution data
	Env      map[string]string            // Environment variables
	Secrets  map[string]string            // Secret values (resolved)
	Inputs   map[string]string            // Workflow inputs
	Steps    map[string]map[string]string // step-id -> outputs
	Workflow WorkflowMeta                 // Current workflow metadata

	// Running services (caller updates ContainerID after starting)
	Services map[string]*ServiceInfo

	// Container runtime (provided by caller)
	Runtime ContainerRuntime

	// Factory creates Step instances from schema definitions.
	// Shared across cloned contexts (like Services and Runtime).
	Factory StepFactory
}

// Clone creates a deep copy of the context (for parallel branches).
// Services map and Factory are shared (global to the workflow).
func (c *StepContext) Clone() *StepContext {
	if c == nil {
		return nil
	}

	clone := &StepContext{
		Env:      make(map[string]string, len(c.Env)),
		Secrets:  make(map[string]string, len(c.Secrets)),
		Inputs:   make(map[string]string, len(c.Inputs)),
		Steps:    make(map[string]map[string]string, len(c.Steps)),
		Workflow: c.Workflow,
		Services: c.Services, // Shared reference (services are global)
		Runtime:  c.Runtime,
		Factory:  c.Factory, // Shared reference (factory is global)
	}

	for k, v := range c.Env {
		clone.Env[k] = v
	}
	for k, v := range c.Secrets {
		clone.Secrets[k] = v
	}
	for k, v := range c.Inputs {
		clone.Inputs[k] = v
	}
	for k, v := range c.Steps {
		// Shallow copy of inner map is fine for immutable outputs
		innerCopy := make(map[string]string, len(v))
		for ik, iv := range v {
			innerCopy[ik] = iv
		}
		clone.Steps[k] = innerCopy
	}

	return clone
}
