package schema

// SequenceStep represents a step that runs steps in sequence
type SequenceStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Sequence are the steps to run in sequence
	Sequence []Step `yaml:"sequence" json:"sequence" jsonschema:"required"`
}
