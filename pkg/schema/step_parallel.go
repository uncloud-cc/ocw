package schema

// ParallelStep represents a step that runs steps in parallel
type ParallelStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Parallel are the steps to run in parallel
	Parallel []Step `yaml:"parallel" json:"parallel" jsonschema:"required"`
}
