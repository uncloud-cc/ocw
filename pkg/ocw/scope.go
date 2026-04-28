package ocw

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Scope: the interpolation context that accumulates as steps complete
// ---------------------------------------------------------------------------

// StepOutput holds the key-value outputs from a completed step.
type StepOutput struct {
	Values map[string]string
}

// WorkflowMeta holds metadata about the current workflow.
type WorkflowMeta struct {
	Name string
}

// JobMeta holds metadata about the current job.
type JobMeta struct {
	Name string
}

// Scope is the interpolation context that flows through the workflow.
// Steps read from it and contribute outputs back.
type Scope struct {
	Env      map[string]string
	Secrets  map[string]string
	Steps    map[string]StepOutput // keyed by step ID
	Workflow WorkflowMeta
	Job      JobMeta
	Logger   *log.Logger // for interpolation warnings
}

// NewScope creates a Scope with initialized maps.
func NewScope() *Scope {
	return &Scope{
		Env:     make(map[string]string),
		Secrets: make(map[string]string),
		Steps:   make(map[string]StepOutput),
	}
}

// Clone returns a deep copy. Parallel branches receive cloned scopes
// so they cannot see each other's mutations.
func (s *Scope) Clone() *Scope {
	c := &Scope{
		Env:      make(map[string]string, len(s.Env)),
		Secrets:  make(map[string]string, len(s.Secrets)),
		Steps:    make(map[string]StepOutput, len(s.Steps)),
		Workflow: s.Workflow,
		Job:      s.Job,
		Logger:   s.Logger,
	}
	for k, v := range s.Env {
		c.Env[k] = v
	}
	for k, v := range s.Secrets {
		c.Secrets[k] = v
	}
	for id, out := range s.Steps {
		vals := make(map[string]string, len(out.Values))
		for k, v := range out.Values {
			vals[k] = v
		}
		c.Steps[id] = StepOutput{Values: vals}
	}
	return c
}

// Merge adds the outputs of a completed step into this scope.
func (s *Scope) Merge(id string, output StepOutput) {
	if id != "" {
		s.Steps[id] = output
	}
}

// templatePattern matches {{ ... }} with arbitrary internal whitespace.
var templatePattern = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

// Interpolate performs template substitution on a string.
//
// Supported references:
//   - {{ env.VAR }}         environment variable (warns if unresolved)
//   - {{ secrets.NAME }}    secret value
//   - {{ steps.ID.KEY }}    output from a completed step
//   - {{ workflow.name }}   workflow name
//   - {{ job.name }}        current job name
//
// Whitespace inside {{ }} is flexible: {{ env.X }}, {{env.X}}, {{  env.X  }}
// all work. Unresolved references return an error, except for env references
// where the variable exists in the host environment but not in the workflow
// env block -- those produce a warning and leave the template text in place.
// Env references that are neither in scope nor in the host environment are
// hard errors.
func (s *Scope) Interpolate(tmpl string) (string, error) {
	var errors []string

	result := templatePattern.ReplaceAllStringFunc(tmpl, func(match string) string {
		// Extract the inner expression, trimming delimiters and whitespace.
		inner := templatePattern.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		expr := strings.TrimSpace(inner[1])
		parts := strings.SplitN(expr, ".", 2)

		if len(parts) < 2 {
			errors = append(errors, fmt.Sprintf("invalid template expression %q: expected namespace.key", expr))
			return match
		}

		namespace := parts[0]
		key := parts[1]

		switch namespace {
		case "env":
			if v, ok := s.Env[key]; ok {
				return v
			}
			// Not in scope -- check if it exists in the host environment.
			if _, exists := os.LookupEnv(key); exists {
				// Set in the host but not in the OCW env block: warn, leave as-is.
				if s.Logger != nil {
					s.Logger.Printf("warning: {{ env.%s }} is not declared in the workflow env block but is set in the host environment", key)
				}
				return match
			}
			// Not in scope and not set anywhere: hard error.
			errors = append(errors, fmt.Sprintf("environment variable %q is not set", key))
			return match

		case "secrets":
			if v, ok := s.Secrets[key]; ok {
				return v
			}
			errors = append(errors, fmt.Sprintf("unresolved secret %q", key))
			return match

		case "steps":
			// key is "ID.FIELD" -- split once more.
			stepParts := strings.SplitN(key, ".", 2)
			if len(stepParts) < 2 {
				errors = append(errors, fmt.Sprintf("invalid step reference %q: expected steps.ID.key", expr))
				return match
			}
			stepID, field := stepParts[0], stepParts[1]
			if out, ok := s.Steps[stepID]; ok {
				if v, ok := out.Values[field]; ok {
					return v
				}
				errors = append(errors, fmt.Sprintf("step %q has no output %q", stepID, field))
				return match
			}
			errors = append(errors, fmt.Sprintf("step %q not found (referenced by {{ %s }})", stepID, expr))
			return match

		case "workflow":
			switch key {
			case "name":
				return s.Workflow.Name
			default:
				errors = append(errors, fmt.Sprintf("unknown workflow property %q", key))
				return match
			}

		case "job":
			switch key {
			case "name":
				return s.Job.Name
			default:
				errors = append(errors, fmt.Sprintf("unknown job property %q", key))
				return match
			}

		default:
			errors = append(errors, fmt.Sprintf("unknown template namespace %q in {{ %s }}", namespace, expr))
			return match
		}
	})

	if len(errors) > 0 {
		return result, fmt.Errorf("template interpolation: %s", strings.Join(errors, "; "))
	}
	return result, nil
}
