package schema

// Job represents a named job that can be run independently.
// Jobs can contain parallel, sequence, or switch flow control,
// or a single step.
//
// Jobs are the primary way to organize workflows. A single workflow
// file can contain multiple jobs that can be invoked by name:
//
//	ocw build    # runs the "build" job
//	ocw test     # runs the "test" job
//	ocw dev      # runs the "dev" job
type Job struct {
	// Name is an optional display name for the job
	Name Name `yaml:"name,omitempty" json:"name,omitempty" jsonschema:"minLength=1"`
	// Description is an optional human-readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`

	// Outputs defines values to expose after the job completes.
	// Values can reference step outputs using template syntax: {{ <step-id>.<output> }}
	Outputs map[string]string `yaml:"outputs,omitempty" json:"outputs,omitempty"`

	// Flow control - exactly one of these should be set
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

	// Single step (alternative to flow control for simple jobs)
	// Step is a single step to run (mutually exclusive with parallel/sequence/switch)
	Step *Step `yaml:"-" json:"-"` // Handled via custom unmarshaling
}

// GetFlowType returns the flow control type used in this job
func (j *Job) GetFlowType() string {
	if len(j.Parallel) > 0 {
		return "parallel"
	}
	if len(j.Sequence) > 0 {
		return "sequence"
	}
	if j.Switch != nil {
		return "switch"
	}
	if j.Step != nil {
		return "step"
	}
	return ""
}

// GetSteps returns all top-level steps regardless of flow type
func (j *Job) GetSteps() []Step {
	if len(j.Parallel) > 0 {
		return j.Parallel
	}
	if len(j.Sequence) > 0 {
		return j.Sequence
	}
	if j.Step != nil {
		return []Step{*j.Step}
	}
	return nil
}

// UnmarshalYAML implements custom unmarshaling for Job to handle single steps
func (j *Job) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First try to unmarshal as a full job structure
	type jobAlias Job
	var alias jobAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}

	*j = Job(alias)

	// If no flow control is set, check if this is a single step
	if j.GetFlowType() == "" {
		// Try to unmarshal as a step
		var probe map[string]interface{}
		if err := unmarshal(&probe); err != nil {
			return err
		}

		// Check for step discriminators
		if _, hasImage := probe["image"]; hasImage {
			var step RunStep
			if err := unmarshal(&step); err != nil {
				return err
			}
			j.Step = &Step{RunStep: &step}
		} else if _, hasBuild := probe["build"]; hasBuild {
			var step BuildStep
			if err := unmarshal(&step); err != nil {
				return err
			}
			j.Step = &Step{BuildStep: &step}
		}
	}

	return nil
}

// Jobs is a map of job ID to job configuration.
// The job ID (map key) is used to invoke the job via CLI: `ocw <job-id>`
type Jobs map[string]Job
