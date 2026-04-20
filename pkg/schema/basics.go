package schema

// SchemaVersion is the current schema version
const SchemaVersion = "0.1.0"

// Name is a human readable name used to communicate feedback about a workflow.
// Choose them in a human friendly way (e.g. "Pull Requests", "Production Deployment" etc.)
// If a name happens to be a valid ID (e.g. "postgres", "build", "e2e", etc.) it can be used
// as an ID to reference this step.
type Name = string

// ID must start with a letter or underscore and can contain letters, underscores
// and numbers (but no whitespace). IDs are optional and only needed when wanting
// to reference a step by its ID.
// Pattern: ^[A-Za-z_][A-Za-z0-9_]*$
type ID = string

// Description is a human readable description
type Description = string

// Outputs are workflow outputs as key-value pairs
type Outputs = map[string]string

// BoolOrString can be either a boolean or a string
type BoolOrString struct {
	Bool   *bool
	String *string
}

// UnmarshalYAML implements custom unmarshaling for BoolOrString
func (b *BoolOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var bl bool
	if err := unmarshal(&bl); err == nil {
		b.Bool = &bl
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		b.String = &s
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for BoolOrString
func (b BoolOrString) MarshalYAML() (interface{}, error) {
	if b.Bool != nil {
		return *b.Bool, nil
	}
	return b.String, nil
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
