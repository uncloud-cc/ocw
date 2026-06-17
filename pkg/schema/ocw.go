package schema

import (
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
	// Inputs are optional workflow inputs & secrets
	Inputs Inputs `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	// Outputs are optional workflow outputs
	Outputs Outputs `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	// Volumes are named volumes for host filesystem access
	Volumes Volumes `yaml:"volumes,omitempty" json:"volumes,omitempty"`

	// Jobs are named entry points that can be run via `ocw <job-name>`
	Jobs Jobs `yaml:"jobs,omitempty" json:"jobs,omitempty"`

	// Flow control - one of these can be set for a default/unnamed job
	// Parallel runs steps in parallel
	Parallel []Step `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	// Sequence runs steps in sequence
	Sequence []Step `yaml:"sequence,omitempty" json:"sequence,omitempty"`
	// Switch conditionally executes steps
	Switch string `yaml:"switch,omitempty" json:"switch,omitempty"`
	// Case are the switch case branches
	Case map[string]Step `yaml:"case,omitempty" json:"case,omitempty"`
	// Default is the switch default branch
	Default *Step `yaml:"default,omitempty" json:"default,omitempty"`
}

// Parse parses a YAML byte slice into an OCW schema.
// This is used by pkg/ocw/parse.go - use that package for file parsing instead.
func Parse(data []byte) (*OCW, error) {
	var ocw OCW
	if err := yaml.Unmarshal(data, &ocw); err != nil {
		return nil, err
	}
	return &ocw, nil
}

// Marshal serializes an OCW schema to YAML
func (o *OCW) Marshal() ([]byte, error) {
	return yaml.Marshal(o)
}
