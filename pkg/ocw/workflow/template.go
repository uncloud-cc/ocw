package workflow

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// -----------------------------------------------------------------------------
// Template Resolution
// -----------------------------------------------------------------------------

var templatePattern = regexp.MustCompile(`\$?\{\{\s*(.+?)\s*\}\}`)

// Interpolate resolves all {{ expr }} templates in a step's fields
// using values from the StepContext.
func Interpolate(step Step, ctx *StepContext) error {
	if step == nil {
		return fmt.Errorf("step is nil")
	}

	original := step.Original()
	if original == nil {
		return nil
	}

	var errs []error
	resolve := func(s string) string {
		resolved, err := ResolveString(s, ctx)
		if err != nil {
			errs = append(errs, err)
		}
		return resolved
	}

	v := reflect.ValueOf(original).Elem()
	interpolateValue(v, resolve)

	if len(errs) > 0 {
		return fmt.Errorf("interpolation failed: %w", errs[0])
	}
	return nil
}

// interpolateValue recursively walks a reflect.Value and resolves templates
// in all string fields, map values, and slice elements.
func interpolateValue(v reflect.Value, resolve func(string) string) {
	if !v.IsValid() {
		return
	}

	// Dereference pointers to get to the actual value
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	if !v.IsValid() {
		return
	}

	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			resolved := resolve(v.String())
			if resolved != v.String() {
				v.SetString(resolved)
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			interpolateValue(v.Index(i), resolve)
		}

	case reflect.Map:
		if v.IsNil() {
			return
		}
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)

			// Resolve string keys
			resolvedKey := key
			if key.Kind() == reflect.String {
				resolvedKeyStr := resolve(key.String())
				if resolvedKeyStr != key.String() {
					resolvedKey = reflect.ValueOf(resolvedKeyStr)
				}
			}

			// Handle map values
			if val.Kind() == reflect.String {
				resolvedVal := resolve(val.String())
				if resolvedKey != key {
					v.SetMapIndex(key, reflect.Value{})
				}
				v.SetMapIndex(resolvedKey, reflect.ValueOf(resolvedVal))
			} else {
				// For non-string values, create a settable copy
				ptr := reflect.New(val.Type())
				ptr.Elem().Set(val)
				interpolateValue(ptr.Elem(), resolve)
				interpolatedVal := ptr.Elem()

				if resolvedKey != key {
					v.SetMapIndex(key, reflect.Value{})
				}
				v.SetMapIndex(resolvedKey, interpolatedVal)
			}
		}

	case reflect.Struct:
		// Skip fields that contain deferred steps (will be interpolated at execution time)
		switch v.Type() {
		case reflect.TypeOf(schema.SequenceStep{}):
			// Skip Sequence field - child steps will be interpolated when executed
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				fieldType := v.Type().Field(i)
				if fieldType.Name == "Sequence" {
					continue // Skip child steps
				}
				if field.CanSet() {
					interpolateValue(field, resolve)
				}
			}
			return

		case reflect.TypeOf(schema.ParallelStep{}):
			// Skip Parallel field - child steps will be interpolated when executed
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				fieldType := v.Type().Field(i)
				if fieldType.Name == "Parallel" {
					continue // Skip child steps
				}
				if field.CanSet() {
					interpolateValue(field, resolve)
				}
			}
			return

		case reflect.TypeOf(schema.SwitchStep{}):
			// Skip Case and Default fields - branch steps will be interpolated when executed
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				fieldType := v.Type().Field(i)
				if fieldType.Name == "Case" || fieldType.Name == "Default" {
					continue // Skip branch steps
				}
				if field.CanSet() {
					interpolateValue(field, resolve)
				}
			}
			return
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanSet() {
				interpolateValue(field, resolve)
			}
		}

	case reflect.Interface:
		if !v.IsNil() {
			interpolateValue(v.Elem(), resolve)
		}
	}
}

// ResolveString resolves template expressions in a single string.
// Supports: {{ env.VAR }}, {{ secrets.NAME }}, {{ steps.ID.key }}, {{ inputs.NAME }}, {{ workflow.name }}
func ResolveString(s string, ctx *StepContext) (string, error) {
	if ctx == nil {
		return s, nil
	}

	var lastErr error
	result := templatePattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract expression: {{ expr }} -> expr
		matches := templatePattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		expr := matches[1]

		value, err := evaluateExpr(expr, ctx)
		if err != nil {
			lastErr = err
			return match // Keep original on error
		}
		return value
	})

	return result, lastErr
}

// evaluateExpr evaluates a simple expression like "env.FOO" or "steps.build.image".
func evaluateExpr(expr string, ctx *StepContext) (string, error) {
	parts := strings.Split(expr, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid expression: %s", expr)
	}

	switch parts[0] {
	case "env":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid env expression: %s", expr)
		}
		if v, ok := ctx.Env[parts[1]]; ok {
			return v, nil
		}
		return "", fmt.Errorf("env variable not found: %s", parts[1])

	case "secrets":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid secrets expression: %s", expr)
		}
		if v, ok := ctx.Secrets[parts[1]]; ok {
			return v, nil
		}
		return "", fmt.Errorf("secret not found: %s", parts[1])

	case "steps":
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid steps expression: %s (expected steps.ID.key)", expr)
		}
		stepID, key := parts[1], parts[2]
		if outputs, ok := ctx.Steps[stepID]; ok {
			if v, ok := outputs[key]; ok {
				return v, nil
			}
			return "", fmt.Errorf("step %s has no output %s", stepID, key)
		}
		return "", fmt.Errorf("step not found: %s", stepID)

	case "inputs":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid inputs expression: %s", expr)
		}
		if v, ok := ctx.Inputs[parts[1]]; ok {
			return v, nil
		}
		return "", fmt.Errorf("input not found: %s", parts[1])

	case "workflow":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid workflow expression: %s", expr)
		}
		switch parts[1] {
		case "name":
			return ctx.Workflow.Name, nil
		case "id":
			return ctx.Workflow.ID, nil
		default:
			return "", fmt.Errorf("unknown workflow property: %s", parts[1])
		}

	default:
		return "", fmt.Errorf("unknown expression namespace: %s", parts[0])
	}
}
