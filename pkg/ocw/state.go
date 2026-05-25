package ocw

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// State stores the state of the workflow
type State struct {
	// Meta stores some meta information about the workflow (e.g. job name)
	Meta map[string]string
	// Env stores the environemnt variables that were declared at the top of the workflow
	Env map[string]string
	// Steps stores the output of steps
	Steps map[string]map[string]string

	mu sync.RWMutex
}

// templatePattern matches {{ ... }} expressions
// Supports: {{ steps.id.output }}, {{ env.NAME }}, {{ meta.name }} etc.
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// InterpolateTemplate scans for templatePattern and interpolates the templates with values from the state
// If no templatePattern is found, it just returns the original input string unchanged
func (s *State) InterpolateTemplate(input string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var err error

	result := templatePattern.ReplaceAllStringFunc(input, func(match string) string {
		expr := templatePattern.FindStringSubmatch(match)[1]
		expr = strings.TrimSpace(expr)

		parts := strings.Split(expr, ".")
		namespace := parts[0]

		switch namespace {
		case "meta":
			key := parts[1]
			value, exists := s.Meta[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in meta namespace", key)
				return ""
			}
			return value
		case "env":
			key := parts[1]
			value, exists := s.Env[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in env namespace", key)
				return ""
			}
			return value
		case "steps":
			if len(parts) < 3 {
				err = fmt.Errorf("Step output references in templates need three parts: {{ steps.<stepId>.<key> }} (e.g. {{ steps.build.image }})")
				return ""
			}

			stepId := parts[1]
			stepOutputs, exists := s.Steps[stepId]
			if !exists {
				err = fmt.Errorf("Could not any outputs for step '%s'", stepId)
				return ""
			}

			key := parts[2]
			value, exists := stepOutputs[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in step outputs for step '%s'", key, stepId)
				return ""
			}
			return value
		}

		err = fmt.Errorf("Unknown template namespace '%s' - has to be one of meta, env, steps", namespace)
		return ""
	})

	if err != nil {
		return "", err
	}

	return result, nil
}

// SetStepOutput writes a single output key for a given step ID into the shared state.
func (s *State) SetStepOutput(stepID, key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Steps[stepID] == nil {
		s.Steps[stepID] = make(map[string]string)
	}
	s.Steps[stepID][key] = value
}

// SetStepOutputs writes all outputs for a given step ID into the shared state.
// The stepID must be non-empty and outputs must be non-nil; the caller (compiler)
// is responsible for those checks.
func (s *State) SetStepOutputs(stepID string, outputs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Steps[stepID] == nil {
		s.Steps[stepID] = make(map[string]string)
	}
	for k, v := range outputs {
		s.Steps[stepID][k] = v
	}
}

// Clone returns a deep copy of the state safe for independent mutation.
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cloned := &State{
		Meta:  make(map[string]string, len(s.Meta)),
		Env:   make(map[string]string, len(s.Env)),
		Steps: make(map[string]map[string]string, len(s.Steps)),
	}
	for k, v := range s.Meta {
		cloned.Meta[k] = v
	}
	for k, v := range s.Env {
		cloned.Env[k] = v
	}
	for stepID, outputs := range s.Steps {
		cloned.Steps[stepID] = make(map[string]string, len(outputs))
		for k, v := range outputs {
			cloned.Steps[stepID][k] = v
		}
	}
	return cloned
}
