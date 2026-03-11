package schema

// InputType represents the type of an input
type InputType string

const (
	InputTypeString  InputType = "string"
	InputTypeNumber  InputType = "number"
	InputTypeBoolean InputType = "boolean"
	InputTypeChoice  InputType = "choice"
)

// InputBase contains common fields for all input types
type InputBase struct {
	// Description is a human-readable description of the input
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Required indicates whether the input is required
	Required bool `yaml:"required,omitempty" json:"required" jsonschema:"default=false"`
}

// StringInput represents a string input with optional validation
type StringInput struct {
	InputBase `yaml:",inline" json:",inline"`
	// Type is always "string" for string inputs
	Type InputType `yaml:"type,omitempty" json:"type" jsonschema:"required,const=string,default=string"`
	// Default is the default value
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	// Pattern is a regex pattern for validation
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	// MinLength is the minimum allowed length
	MinLength *int `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	// MaxLength is the maximum allowed length
	MaxLength *int `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
}

// NumberInput represents a numeric input with optional bounds
type NumberInput struct {
	InputBase `yaml:",inline" json:",inline"`
	// Type is always "number" for number inputs
	Type InputType `yaml:"type" json:"type" jsonschema:"required,const=number"`
	// Default is the default value
	Default *float64 `yaml:"default,omitempty" json:"default,omitempty"`
	// Min is the minimum allowed value
	Min *float64 `yaml:"min,omitempty" json:"min,omitempty"`
	// Max is the maximum allowed value
	Max *float64 `yaml:"max,omitempty" json:"max,omitempty"`
}

// BooleanInput represents a boolean input
type BooleanInput struct {
	InputBase `yaml:",inline" json:",inline"`
	// Type is always "boolean" for boolean inputs
	Type InputType `yaml:"type" json:"type" jsonschema:"required,const=boolean"`
	// Default is the default value
	Default *bool `yaml:"default,omitempty" json:"default,omitempty"`
}

// ChoiceInput represents a choice input (dropdown/select)
type ChoiceInput struct {
	InputBase `yaml:",inline" json:",inline"`
	// Type is always "choice" for choice inputs
	Type InputType `yaml:"type" json:"type" jsonschema:"required,const=choice"`
	// Options is the list of available options (must have at least one)
	Options []string `yaml:"options" json:"options" jsonschema:"required,minItems=1"`
	// Default is the default selected option
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Input represents any input type (discriminated union by "type" field)
type Input struct {
	// String input fields
	StringInput *StringInput
	// Number input fields
	NumberInput *NumberInput
	// Boolean input fields
	BooleanInput *BooleanInput
	// Choice input fields
	ChoiceInput *ChoiceInput
}

// UnmarshalYAML implements custom unmarshaling for Input
func (i *Input) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First, get the type field to determine which variant to use
	var typeProbe struct {
		Type InputType `yaml:"type"`
	}
	if err := unmarshal(&typeProbe); err != nil {
		return err
	}

	// Default to string type if not specified
	if typeProbe.Type == "" {
		typeProbe.Type = InputTypeString
	}

	switch typeProbe.Type {
	case InputTypeString:
		var s StringInput
		if err := unmarshal(&s); err != nil {
			return err
		}
		s.Type = InputTypeString
		i.StringInput = &s
	case InputTypeNumber:
		var n NumberInput
		if err := unmarshal(&n); err != nil {
			return err
		}
		i.NumberInput = &n
	case InputTypeBoolean:
		var b BooleanInput
		if err := unmarshal(&b); err != nil {
			return err
		}
		i.BooleanInput = &b
	case InputTypeChoice:
		var c ChoiceInput
		if err := unmarshal(&c); err != nil {
			return err
		}
		i.ChoiceInput = &c
	}

	return nil
}

// MarshalYAML implements custom marshaling for Input
func (i Input) MarshalYAML() (interface{}, error) {
	if i.StringInput != nil {
		return i.StringInput, nil
	}
	if i.NumberInput != nil {
		return i.NumberInput, nil
	}
	if i.BooleanInput != nil {
		return i.BooleanInput, nil
	}
	if i.ChoiceInput != nil {
		return i.ChoiceInput, nil
	}
	return nil, nil
}

// GetType returns the input type
func (i Input) GetType() InputType {
	if i.StringInput != nil {
		return InputTypeString
	}
	if i.NumberInput != nil {
		return InputTypeNumber
	}
	if i.BooleanInput != nil {
		return InputTypeBoolean
	}
	if i.ChoiceInput != nil {
		return InputTypeChoice
	}
	return InputTypeString
}

// Inputs is a map of input names to their definitions
type Inputs = map[string]Input
