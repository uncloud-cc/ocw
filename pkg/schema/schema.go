package schema

import (
	"os"

	"github.com/goccy/go-yaml"
)

// OCW represents the main Open Container Workflow schema.
// It can contain jobs (named entry points) and/or direct flow control steps.
type OCW struct {
	// SchemaVersion is the schema version
	SchemaVersion string `yaml:"schemaVersion" json:"schemaVersion" jsonschema:"required,const=0.1.0"`
	// Name is the workflow name
	Name Name `yaml:"name" json:"name" jsonschema:"required,minLength=1"`
	// ID is an optional workflow identifier
	ID ID `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	// Description is an optional human readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`
	// Config is optional workflow-level configuration
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Inputs are optional workflow inputs
	Inputs Inputs `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	// Env are optional environment variables
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Outputs are optional workflow outputs
	Outputs Outputs `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	// Jobs are named entry points that can be run via `ocw <job-name>`
	Jobs Jobs `yaml:"jobs,omitempty" json:"jobs,omitempty"`

	// Flow control - one of these can be set for a default/unnamed job
	// Parallel runs steps in parallel
	Parallel []Step `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	// Sequence runs steps in sequence
	Sequence []Step `yaml:"sequence,omitempty" json:"sequence,omitempty"`
	// Switch conditionally executes steps
	Switch *string `yaml:"switch,omitempty" json:"switch,omitempty"`
	// Case are the switch case branches
	Case map[string]StepOrSteps `yaml:"case,omitempty" json:"case,omitempty"`
	// Default is the switch default branch
	Default *StepOrSteps `yaml:"default,omitempty" json:"default,omitempty"`
}

// GetFlowType returns the flow control type used in this workflow (for direct execution)
func (o *OCW) GetFlowType() string {
	if len(o.Parallel) > 0 {
		return "parallel"
	}
	if len(o.Sequence) > 0 {
		return "sequence"
	}
	if o.Switch != nil {
		return "switch"
	}
	return ""
}

// HasDirectFlow returns true if the workflow has direct flow control (not just jobs)
func (o *OCW) HasDirectFlow() bool {
	return o.GetFlowType() != ""
}

// HasJobs returns true if the workflow has named jobs
func (o *OCW) HasJobs() bool {
	return len(o.Jobs) > 0
}

// GetJob returns a job by name, or nil if not found
func (o *OCW) GetJob(name string) *Job {
	if o.Jobs == nil {
		return nil
	}
	job, ok := o.Jobs[name]
	if !ok {
		return nil
	}
	return &job
}

// GetJobNames returns a list of all job names
func (o *OCW) GetJobNames() []string {
	if o.Jobs == nil {
		return nil
	}
	names := make([]string, 0, len(o.Jobs))
	for name := range o.Jobs {
		names = append(names, name)
	}
	return names
}

// GetSteps returns all top-level steps regardless of flow type
func (o *OCW) GetSteps() []Step {
	if len(o.Parallel) > 0 {
		return o.Parallel
	}
	if len(o.Sequence) > 0 {
		return o.Sequence
	}
	return nil
}

// Parse parses a YAML byte slice into an OCW schema
func Parse(data []byte) (*OCW, error) {
	var ocw OCW
	if err := yaml.Unmarshal(data, &ocw); err != nil {
		return nil, err
	}
	return &ocw, nil
}

// ParseFile parses a YAML file into an OCW schema
func ParseFile(path string) (*OCW, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Marshal serializes an OCW schema to YAML
func (o *OCW) Marshal() ([]byte, error) {
	return yaml.Marshal(o)
}
