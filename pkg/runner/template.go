package runner

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TemplateContext holds the data available for template interpolation
type TemplateContext struct {
	// Steps contains outputs from completed steps, keyed by step ID
	// Each step can have multiple outputs (e.g., "image" for build steps)
	Steps map[string]map[string]string

	// Secrets contains secret values, keyed by secret name
	Secrets map[string]string

	// Env contains environment variables (workflow-level + system)
	Env map[string]string

	// Inputs contains input values, keyed by input name
	Inputs map[string]string

	// Workflow contains workflow metadata
	Workflow WorkflowMeta

	// Job contains current job metadata
	Job JobMeta
}

// WorkflowMeta contains workflow-level metadata
type WorkflowMeta struct {
	Name        string
	Description string
	ID          string
}

// JobMeta contains job-level metadata
type JobMeta struct {
	Name        string
	Description string
	ID          string
}

// NewTemplateContext creates a new empty template context
func NewTemplateContext() *TemplateContext {
	return &TemplateContext{
		Steps:   make(map[string]map[string]string),
		Secrets: make(map[string]string),
		Env:     make(map[string]string),
		Inputs:  make(map[string]string),
	}
}

// SetStepOutput sets an output value for a step
func (tc *TemplateContext) SetStepOutput(stepID, key, value string) {
	if tc.Steps[stepID] == nil {
		tc.Steps[stepID] = make(map[string]string)
	}
	tc.Steps[stepID][key] = value
}

// GetStepOutput gets an output value for a step
func (tc *TemplateContext) GetStepOutput(stepID, key string) (string, bool) {
	if step, ok := tc.Steps[stepID]; ok {
		if value, ok := step[key]; ok {
			return value, true
		}
	}
	return "", false
}

// templatePattern matches {{ ... }} expressions
// Supports: {{ steps.id.output }}, {{ secrets.NAME }}, {{ env.NAME }}, etc.
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// Interpolate replaces template expressions in a string with their values
func (tc *TemplateContext) Interpolate(s string) (string, error) {
	var lastErr error

	result := templatePattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the expression inside {{ }}
		expr := templatePattern.FindStringSubmatch(match)[1]
		expr = strings.TrimSpace(expr)

		value, err := tc.evaluateExpression(expr)
		if err != nil {
			lastErr = err
			return match // Keep original if error
		}
		return value
	})

	return result, lastErr
}

// evaluateExpression evaluates a single template expression
func (tc *TemplateContext) evaluateExpression(expr string) (string, error) {
	parts := strings.Split(expr, ".")

	if len(parts) < 2 {
		return "", fmt.Errorf("invalid template expression: %s", expr)
	}

	switch parts[0] {
	case "steps":
		// {{ steps.<id>.<output> }}
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid steps expression: %s (expected steps.<id>.<output>)", expr)
		}
		stepID := parts[1]
		outputKey := parts[2]
		if value, ok := tc.GetStepOutput(stepID, outputKey); ok {
			return value, nil
		}
		return "", fmt.Errorf("step output not found: steps.%s.%s", stepID, outputKey)

	case "secrets":
		// {{ secrets.<name> }}
		secretName := parts[1]
		if value, ok := tc.Secrets[secretName]; ok {
			return value, nil
		}
		// Also check environment variables for secrets
		if value := os.Getenv(secretName); value != "" {
			return value, nil
		}
		return "", fmt.Errorf("secret not found: %s", secretName)

	case "env":
		// {{ env.<name> }}
		envName := parts[1]
		if value, ok := tc.Env[envName]; ok {
			return value, nil
		}
		// Fall back to system environment
		if value := os.Getenv(envName); value != "" {
			return value, nil
		}
		return "", fmt.Errorf("environment variable not found: %s", envName)

	case "inputs":
		// {{ inputs.<name> }}
		inputName := parts[1]
		if value, ok := tc.Inputs[inputName]; ok {
			return value, nil
		}
		return "", fmt.Errorf("input not found: %s", inputName)

	case "workflow":
		// {{ workflow.<field> }}
		field := parts[1]
		switch field {
		case "name":
			return tc.Workflow.Name, nil
		case "description":
			return tc.Workflow.Description, nil
		case "id":
			return tc.Workflow.ID, nil
		default:
			return "", fmt.Errorf("unknown workflow field: %s", field)
		}

	case "job":
		// {{ job.<field> }}
		field := parts[1]
		switch field {
		case "name":
			return tc.Job.Name, nil
		case "description":
			return tc.Job.Description, nil
		case "id":
			return tc.Job.ID, nil
		default:
			return "", fmt.Errorf("unknown job field: %s", field)
		}

	default:
		return "", fmt.Errorf("unknown template namespace: %s", parts[0])
	}
}

// InterpolateMap interpolates all string values in a map
func (tc *TemplateContext) InterpolateMap(m map[string]string) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		interpolated, err := tc.Interpolate(v)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate %s: %w", k, err)
		}
		result[k] = interpolated
	}
	return result, nil
}

// InterpolateSlice interpolates all strings in a slice
func (tc *TemplateContext) InterpolateSlice(s []string) ([]string, error) {
	if s == nil {
		return nil, nil
	}

	result := make([]string, len(s))
	for i, v := range s {
		interpolated, err := tc.Interpolate(v)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate item %d: %w", i, err)
		}
		result[i] = interpolated
	}
	return result, nil
}

// HasTemplates checks if a string contains template expressions
func HasTemplates(s string) bool {
	return templatePattern.MatchString(s)
}
