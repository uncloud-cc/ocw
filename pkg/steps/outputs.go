package steps

import "sync"

// OutputStore provides thread-safe storage for step outputs.
// Outputs are stored as map[stepID]map[key]value.
type OutputStore struct {
	mu      sync.RWMutex
	outputs map[string]map[string]string
}

// NewOutputStore creates a new output store.
func NewOutputStore() *OutputStore {
	return &OutputStore{
		outputs: make(map[string]map[string]string),
	}
}

// Get retrieves an output value: store.Get("stepID", "key").
func (s *OutputStore) Get(stepID, key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stepOutputs, ok := s.outputs[stepID]
	if !ok {
		return "", false
	}

	value, ok := stepOutputs[key]
	return value, ok
}

// GetAll retrieves all outputs for a step.
func (s *OutputStore) GetAll(stepID string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stepOutputs, ok := s.outputs[stepID]
	if !ok {
		return nil, false
	}

	// Return a copy
	result := make(map[string]string, len(stepOutputs))
	for k, v := range stepOutputs {
		result[k] = v
	}
	return result, true
}

// Set stores outputs for a step.
func (s *OutputStore) Set(stepID string, outputs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.outputs[stepID] = outputs
}

// Snapshot returns a copy of all outputs (for building scope).
func (s *OutputStore) Snapshot() map[string]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]string, len(s.outputs))
	for stepID, outputs := range s.outputs {
		stepCopy := make(map[string]string, len(outputs))
		for k, v := range outputs {
			stepCopy[k] = v
		}
		result[stepID] = stepCopy
	}
	return result
}
