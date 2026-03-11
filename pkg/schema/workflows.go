package schema

// Inheritance specifies how env/secrets should be inherited
type Inheritance string

const (
	InheritNone Inheritance = "none"
	InheritAll  Inheritance = "all"
)

// InheritConfig specifies inheritance settings for a workflow
type InheritConfig struct {
	// Secrets inheritance mode
	Secrets Inheritance `yaml:"secrets,omitempty" json:"secrets" jsonschema:"required,default=none,enum=none,enum=all"`
	// Env inheritance mode
	Env Inheritance `yaml:"env,omitempty" json:"env" jsonschema:"required,default=none,enum=none,enum=all"`
}

// WorkflowConfig represents the configuration for running another workflow
type WorkflowConfig struct {
	// From is the workflow reference (local path or import path like github.com/org/repo/workflow)
	From string `yaml:"from" json:"from" jsonschema:"required"`
	// Env are environment variables to pass to the workflow
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are secrets to pass to the workflow
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Inherit specifies what to inherit from the parent workflow
	Inherit *InheritConfig `yaml:"inherit,omitempty" json:"inherit,omitempty"`
	// Inputs are input values to pass to the workflow
	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

// WorkflowStep represents a step that runs another workflow
type WorkflowStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Workflow is the workflow configuration
	Workflow WorkflowConfig `yaml:"workflow" json:"workflow" jsonschema:"required"`
}
