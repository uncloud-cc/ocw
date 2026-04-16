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
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
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
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Needs lists service IDs that must be healthy before this step runs.
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
}

// StringOrStringSlice can be either a single string or a slice of strings
type StringOrStringSlice struct {
	Single   *string
	Multiple []string
}

// UnmarshalYAML implements custom unmarshaling for StringOrStringSlice
func (s *StringOrStringSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		s.Single = &single
		return nil
	}

	var multiple []string
	if err := unmarshal(&multiple); err == nil {
		s.Multiple = multiple
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StringOrStringSlice
func (s StringOrStringSlice) MarshalYAML() (interface{}, error) {
	if s.Single != nil {
		return *s.Single, nil
	}
	return s.Multiple, nil
}

// StringMapOrSlice can be either a map[string]string or []string
type StringMapOrSlice struct {
	Map   map[string]string
	Slice []string
}

// UnmarshalYAML implements custom unmarshaling for StringMapOrSlice
func (s *StringMapOrSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]string
	if err := unmarshal(&m); err == nil {
		s.Map = m
		return nil
	}

	var sl []string
	if err := unmarshal(&sl); err == nil {
		s.Slice = sl
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StringMapOrSlice
func (s StringMapOrSlice) MarshalYAML() (interface{}, error) {
	if s.Map != nil {
		return s.Map, nil
	}
	return s.Slice, nil
}

// NumberOrString can be either a number or a string
type NumberOrString struct {
	Number *float64
	String *string
}

// UnmarshalYAML implements custom unmarshaling for NumberOrString
func (n *NumberOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var num float64
	if err := unmarshal(&num); err == nil {
		n.Number = &num
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		n.String = &s
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for NumberOrString
func (n NumberOrString) MarshalYAML() (interface{}, error) {
	if n.Number != nil {
		return *n.Number, nil
	}
	return n.String, nil
}

// UlimitValue can be either a number or a "soft:hard" string
type UlimitValue = NumberOrString

// ParallelStep represents a step that runs steps in parallel
type ParallelStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Parallel are the steps to run in parallel
	Parallel []Step `yaml:"parallel" json:"parallel" jsonschema:"required"`
}

// SequenceStep represents a step that runs steps in sequence
type SequenceStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Sequence are the steps to run in sequence
	Sequence []Step `yaml:"sequence" json:"sequence" jsonschema:"required"`
}

// StepOrSteps can be a single step or an array of steps
type StepOrSteps struct {
	Single   *Step
	Multiple []Step
}

// UnmarshalYAML implements custom unmarshaling for StepOrSteps
func (s *StepOrSteps) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single Step
	if err := unmarshal(&single); err == nil {
		s.Single = &single
		return nil
	}

	var multiple []Step
	if err := unmarshal(&multiple); err == nil {
		s.Multiple = multiple
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StepOrSteps
func (s StepOrSteps) MarshalYAML() (interface{}, error) {
	if s.Single != nil {
		return s.Single, nil
	}
	return s.Multiple, nil
}

// SwitchStep represents a step that switches on a value
type SwitchStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Switch is the expression to switch on
	Switch string `yaml:"switch" json:"switch" jsonschema:"required"`
	// Case are the case branches
	Case map[string]StepOrSteps `yaml:"case" json:"case" jsonschema:"required"`
	// Default is the default branch
	Default *StepOrSteps `yaml:"default,omitempty" json:"default,omitempty"`
}

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
