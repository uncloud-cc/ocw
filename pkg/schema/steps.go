package schema

import "fmt"

// StepBase contains common fields for all step types
type StepBase struct {
	// Name is a human readable name for the step
	Name Name `yaml:"name" json:"name" jsonschema:"required,minLength=1"`
	// ID is an optional identifier to reference this step
	ID ID `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	// Description is an optional human readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`
	// Config is optional step-level configuration
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Env are optional environment variables
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	// EnvFile is one or more environment files from workspace
	// Useful for config assembled by previous steps
	EnvFile *StringOrStringSlice `yaml:"envFile,omitempty" json:"envFile,omitempty"`
	// Secrets are optional secrets
	Secrets map[string]string `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Needs lists service IDs that must be healthy before this step runs.
	// All services are implicitly available to all steps on the internal network.
	// Use Needs only when this step must wait for specific services to be ready.
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
}

// OptionalStepBase is like StepBase but with optional name (for parallel/sequence)
type OptionalStepBase struct {
	// Name is an optional human readable name for the step
	Name Name `yaml:"name,omitempty" json:"name,omitempty" jsonschema:"minLength=1"`
	// ID is an optional identifier to reference this step
	ID ID `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	// Description is an optional human readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`
	// Config is optional step-level configuration
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Env are optional environment variables
	Env Inputs `yaml:"env,omitempty" json:"env,omitempty"`
	// EnvFile is one or more environment files from workspace
	// Useful for config assembled by previous steps
	EnvFile *StringOrStringSlice `yaml:"envFile,omitempty" json:"envFile,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Needs lists service IDs that must be healthy before this step runs.
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
}

// UlimitValue can be either a number or a "soft:hard" string
type UlimitValue = NumberOrString

// Step represents any step type (discriminated union)
type Step struct {
	// RunStep for container run steps
	RunStep *RunStep
	// BuildStep for image build steps
	BuildStep *BuildStep
	// ParallelStep for parallel execution
	ParallelStep *ParallelStep
	// SequenceStep for sequential execution
	SequenceStep *SequenceStep
	// WorkflowStep for running other workflows
	WorkflowStep *WorkflowStep
	// SwitchStep for conditional branching
	SwitchStep *SwitchStep
}

// UnmarshalYAML implements custom unmarshaling for Step
func (s *Step) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Probe for discriminating fields
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return err
	}

	// Check for each step type based on its discriminating field
	if _, ok := probe["parallel"]; ok {
		var step ParallelStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.ParallelStep = &step
		return nil
	}

	if _, ok := probe["sequence"]; ok {
		var step SequenceStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.SequenceStep = &step
		return nil
	}

	if _, ok := probe["workflow"]; ok {
		var step WorkflowStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.WorkflowStep = &step
		return nil
	}

	if _, ok := probe["switch"]; ok {
		var step SwitchStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.SwitchStep = &step
		return nil
	}

	if _, ok := probe["build"]; ok {
		var step BuildStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.BuildStep = &step
		return nil
	}

	// Default to RunStep (has "image" field)
	if _, ok := probe["image"]; ok {
		var step RunStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.RunStep = &step
		return nil
	}

	return fmt.Errorf("unrecognized step type: must have one of 'run', 'build', 'push', 'parallel', or 'workflow' field")
}

// MarshalYAML implements custom marshaling for Step
func (s Step) MarshalYAML() (interface{}, error) {
	if s.RunStep != nil {
		return s.RunStep, nil
	}
	if s.BuildStep != nil {
		return s.BuildStep, nil
	}
	if s.ParallelStep != nil {
		return s.ParallelStep, nil
	}
	if s.SequenceStep != nil {
		return s.SequenceStep, nil
	}
	if s.WorkflowStep != nil {
		return s.WorkflowStep, nil
	}
	if s.SwitchStep != nil {
		return s.SwitchStep, nil
	}
	return nil, nil
}

// StepTypeInfo holds metadata for a step type that participates in the
// discriminated union. It is used by schema generators to auto-register
// all step definitions.
type StepTypeInfo struct {
	Name string
	Type any
}

// StepTypes is the registry of all step types that appear in the Step
// discriminated union. When adding a new step type, register it here.
var StepTypes = []StepTypeInfo{
	{Name: "RunStep", Type: RunStep{}},
	{Name: "BuildStep", Type: BuildStep{}},
	{Name: "ParallelStep", Type: ParallelStep{}},
	{Name: "SequenceStep", Type: SequenceStep{}},
	{Name: "WorkflowStep", Type: WorkflowStep{}},
	{Name: "SwitchStep", Type: SwitchStep{}},
}
