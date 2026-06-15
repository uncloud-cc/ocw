package ocw

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	gonanoid "github.com/matoous/go-nanoid"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// State stores the state of the workflow
type State struct {
	// RunID is a unique identifier for this workflow run.
	RunID string
	// Meta stores some meta information about the workflow (e.g. job name)
	Meta map[string]string
	// Inputs stores the input variables declared in the "inputs" section
	Inputs map[string]string
	// Secrets stores the secrets declared in the "inputs" section
	Secrets map[string]string
	// Steps stores the output of steps
	Steps map[string]map[string]string
	// Env is a snapshot of environment variables available for {{ env.KEY }}
	// templates. It is isolated per workflow so child workflows don't
	// accidentally leak parent environment values.
	Env map[string]string

	mu sync.RWMutex
}

// NewState initializes a State from inputs, environment variables,
// and an optional JSON input file. Environment variables are snapshotted
// into state.Env so {{ env.KEY }} templates are isolated per workflow.
// A RunID is automatically generated for each new State.
func NewState(inputConfig *schema.Inputs, inputFile string) (*State, error) {
	runID, err := gonanoid.Generate(NanoidAlphabet, 12)
	if err != nil {
		return nil, fmt.Errorf("cannot create runID: %w", err)
	}

	state := &State{
		RunID:   runID,
		Meta:    make(map[string]string),
		Inputs:  make(map[string]string),
		Secrets: make(map[string]string),
		Steps:   make(map[string]map[string]string),
		Env:     make(map[string]string),
	}

	// Snapshot current process environment so {{ env }} works
	for _, e := range os.Environ() {
		if i := strings.IndexByte(e, '='); i >= 0 {
			state.Env[e[:i]] = e[i+1:]
		}
	}

	// No inputs declared → return state with just the env snapshot
	if inputConfig == nil || len(*inputConfig) == 0 {
		return state, nil
	}

	// Load optional JSON overrides
	fileValues := make(map[string]string)
	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, fmt.Errorf("read input file %q: %w", inputFile, err)
		}
		if err := json.Unmarshal(data, &fileValues); err != nil {
			return nil, fmt.Errorf("parse input file %q: %w", inputFile, err)
		}
	}

	for key, decl := range *inputConfig {
		var value string

		// Precedence: JSON input file > env var > schema default
		switch {
		case fileValues[key] != "":
			value = fileValues[key]
		case state.Env[key] != "":
			value = state.Env[key]
		case decl.Value != "":
			value = decl.Value
		default:
			return nil, fmt.Errorf(
				"required input %q is not set (provide env var %q, add to input file, or set a default)",
				key, key,
			)
		}

		// Publish inputs into the isolated env snapshot so {{ env.KEY }} works
		state.Env[key] = value

		if decl.IsSecret {
			state.Secrets[key] = value
		} else {
			state.Inputs[key] = value
		}
	}

	return state, nil
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

		case "inputs":
			key := parts[1]
			value, exists := s.Inputs[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in inputs", key)
				return ""
			}
			return value

		case "secrets":
			key := parts[1]
			value, exists := s.Secrets[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in secrets", key)
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

		case "env":
			key := parts[1]
			value, exists := s.Env[key]
			if !exists {
				err = fmt.Errorf("Could not find key '%s' in env", key)
				return ""
			}
			return value
		}

		err = fmt.Errorf("Unknown template namespace '%s' - has to be one of meta, inputs, secrets, steps", namespace)
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

func (s *State) GetSecretValues() []string {
	secretValues := []string{}
	for _, secretValue := range s.Secrets {
		secretValues = append(secretValues, secretValue)
	}

	return secretValues
}

// Clone returns a deep copy of the state safe for independent mutation.
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cloned := &State{
		RunID:   s.RunID,
		Meta:    make(map[string]string, len(s.Meta)),
		Inputs:  make(map[string]string, len(s.Inputs)),
		Secrets: make(map[string]string, len(s.Secrets)),
		Steps:   make(map[string]map[string]string, len(s.Steps)),
		Env:     make(map[string]string, len(s.Env)),
	}
	for k, v := range s.Meta {
		cloned.Meta[k] = v
	}
	for k, v := range s.Inputs {
		cloned.Inputs[k] = v
	}
	for k, v := range s.Secrets {
		cloned.Secrets[k] = v
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

// ResolveOutputs interpolates a raw outputs map (from schema) against the
// current workflow state. It returns a resolved map ready for display.
func (s *State) ResolveOutputs(outputs map[string]string) (map[string]string, error) {
	if len(outputs) == 0 {
		return nil, nil
	}
	resolved := make(map[string]string, len(outputs))
	for key, template := range outputs {
		value, err := s.InterpolateTemplate(template)
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", key, err)
		}
		resolved[key] = value
	}
	return resolved, nil
}
