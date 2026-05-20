package ocw

import (
	"fmt"
	"regexp"
	"strings"
)

// State stores the state of the workflow
type State struct {
	// Meta stores some meta information about the workflow (e.g. job name)
	Meta map[string]string
	// Env stores the environemnt variables that were declared at the top of the workflow
	Env map[string]string
	// Steps stores the output of steps
	Steps map[string]map[string]string
}

// templatePattern matches {{ ... }} expressions
// Supports: {{ steps.id.output }}, {{ env.NAME }}, {{ workflow.name }}, {{ job.name }} etc.
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// InterpolateTemplate scans for templatePattern and interpolates the templates with values from the state
// If no templatePattern is found, it just returns the original input string unchanged
func (s *State) InterpolateTemplate(input string) (string, error) {
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
