package steps

// Scope contains the data available for template interpolation.
// It is built up as steps execute and passed to parsers.
type Scope struct {
	// Env contains environment variables (workflow + job + step level merged).
	Env map[string]string

	// Secrets contains resolved secret values.
	Secrets map[string]string

	// Inputs contains workflow inputs (from CLI or parent workflow).
	Inputs map[string]string

	// Steps contains outputs from previous steps, keyed by step ID.
	// Access pattern: Steps["stepID"]["outputKey"]
	Steps map[string]map[string]string

	// Config contains workflow configuration values.
	Config map[string]any
}

// NewScope creates a new empty scope.
func NewScope() *Scope {
	return &Scope{
		Env:     make(map[string]string),
		Secrets: make(map[string]string),
		Inputs:  make(map[string]string),
		Steps:   make(map[string]map[string]string),
		Config:  make(map[string]any),
	}
}

// Clone creates a deep copy of the scope.
func (s *Scope) Clone() *Scope {
	clone := NewScope()

	for k, v := range s.Env {
		clone.Env[k] = v
	}
	for k, v := range s.Secrets {
		clone.Secrets[k] = v
	}
	for k, v := range s.Inputs {
		clone.Inputs[k] = v
	}
	for stepID, outputs := range s.Steps {
		clone.Steps[stepID] = make(map[string]string, len(outputs))
		for k, v := range outputs {
			clone.Steps[stepID][k] = v
		}
	}
	for k, v := range s.Config {
		clone.Config[k] = v
	}

	return clone
}

// WithStepOutputs returns a new scope with the given step's outputs added.
func (s *Scope) WithStepOutputs(stepID string, outputs map[string]string) *Scope {
	clone := s.Clone()
	clone.Steps[stepID] = make(map[string]string, len(outputs))
	for k, v := range outputs {
		clone.Steps[stepID][k] = v
	}
	return clone
}
