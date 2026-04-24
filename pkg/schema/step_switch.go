package schema

// SwitchStep represents a step that switches on a value
type SwitchStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Switch is the expression to switch on
	Switch string `yaml:"switch" json:"switch" jsonschema:"required"`
	// Case are the case branches
	Case map[string]Step `yaml:"case" json:"case" jsonschema:"required"`
	// Default is the default branch
	Default *Step `yaml:"default,omitempty" json:"default,omitempty"`
}
