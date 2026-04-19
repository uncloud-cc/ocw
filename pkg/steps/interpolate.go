package steps

import (
	"fmt"
	"regexp"
	"strings"
)

// templatePattern matches {{ expression }} patterns.
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate resolves template expressions in a string.
// Supported syntax:
//   - {{ env.VAR_NAME }}        - Environment variable
//   - {{ secrets.SECRET_NAME }} - Secret value
//   - {{ inputs.INPUT_NAME }}   - Workflow input
//   - {{ steps.STEP_ID.KEY }}   - Output from a previous step
//   - {{ config.ns.key }}       - Configuration value
//
// Returns the interpolated string and any error encountered.
func Interpolate(template string, stepContext *StepContext) (string, error) {
	var firstErr error

	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		if firstErr != nil {
			return match
		}

		// Extract expression from {{ expr }}
		submatch := templatePattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		expr := strings.TrimSpace(submatch[1])

		value, err := resolveExpression(expr, stepContext)
		if err != nil {
			firstErr = &InterpolationError{
				Template:   template,
				Expression: expr,
				Reason:     err.Error(),
			}
			return match
		}

		return value
	})

	return result, firstErr
}

func resolveExpression(expr string, stepContext *StepContext) (string, error) {
	parts := strings.Split(expr, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid expression: %s", expr)
	}

	switch parts[0] {
	case "env":
		if len(parts) != 2 {
			return "", fmt.Errorf("env requires exactly one key: env.VAR_NAME")
		}
		if val, ok := stepContext.Env[parts[1]]; ok {
			return val, nil
		}
		return "", fmt.Errorf("env variable %q not found", parts[1])

	case "secrets":
		if len(parts) != 2 {
			return "", fmt.Errorf("secrets requires exactly one key: secrets.NAME")
		}
		if val, ok := stepContext.Secrets[parts[1]]; ok {
			return val, nil
		}
		return "", fmt.Errorf("secret %q not found", parts[1])

	case "inputs":
		if len(parts) != 2 {
			return "", fmt.Errorf("inputs requires exactly one key: inputs.NAME")
		}
		if val, ok := stepContext.Inputs[parts[1]]; ok {
			return val, nil
		}
		return "", fmt.Errorf("input %q not found", parts[1])

	case "steps":
		if len(parts) != 3 {
			return "", fmt.Errorf("steps requires step ID and key: steps.STEP_ID.KEY")
		}
		stepID, key := parts[1], parts[2]
		if outputs, ok := stepContext.Steps[stepID]; ok {
			if val, ok := outputs[key]; ok {
				return val, nil
			}
			return "", fmt.Errorf("step %q has no output %q", stepID, key)
		}
		return "", fmt.Errorf("step %q not found", stepID)

	case "config":
		// Navigate nested config: config.namespace.key
		var current any = stepContext.Config
		for i := 1; i < len(parts); i++ {
			if m, ok := current.(map[string]any); ok {
				current = m[parts[i]]
			} else {
				return "", fmt.Errorf("config path %q not found", strings.Join(parts[:i+1], "."))
			}
		}
		if current == nil {
			return "", fmt.Errorf("config %q not found", expr)
		}
		return fmt.Sprintf("%v", current), nil

	default:
		return "", fmt.Errorf("unknown reference type: %s", parts[0])
	}
}

// InterpolateMap interpolates all values in a map.
func InterpolateMap(m map[string]string, stepContext *StepContext) (map[string]string, error) {
	result := make(map[string]string, len(m))
	for k, v := range m {
		interpolated, err := Interpolate(v, stepContext)
		if err != nil {
			return nil, fmt.Errorf("interpolating %q: %w", k, err)
		}
		result[k] = interpolated
	}
	return result, nil
}

// InterpolateSlice interpolates all values in a slice.
func InterpolateSlice(s []string, stepContext *StepContext) ([]string, error) {
	result := make([]string, len(s))
	for i, v := range s {
		interpolated, err := Interpolate(v, stepContext)
		if err != nil {
			return nil, fmt.Errorf("interpolating index %d: %w", i, err)
		}
		result[i] = interpolated
	}
	return result, nil
}

// InterpolationError represents a template interpolation failure.
type InterpolationError struct {
	Template   string
	Expression string
	Reason     string
}

// Error returns the error message.
func (e *InterpolationError) Error() string {
	return fmt.Sprintf("interpolation failed for %q: %s (in template: %s)", e.Expression, e.Reason, e.Template)
}
