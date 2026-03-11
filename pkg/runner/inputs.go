package runner

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// LoadInputsFile loads inputs from a YAML file.
// The file should contain a flat key-value structure.
// Values are converted to strings for template interpolation.
// If the file doesn't exist, returns an empty map (not an error).
func LoadInputsFile(path string) (map[string]string, error) {
	inputs := make(map[string]string)

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty inputs
			return inputs, nil
		}
		return nil, err
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read inputs file: %w", err)
	}

	// Parse YAML into a generic map
	var rawInputs map[string]interface{}
	if err := yaml.Unmarshal(data, &rawInputs); err != nil {
		return nil, fmt.Errorf("failed to parse inputs file: %w", err)
	}

	// Convert all values to strings
	for key, value := range rawInputs {
		inputs[key] = toString(value)
	}

	return inputs, nil
}

// toString converts a value to its string representation
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		// Check if it's a whole number
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}
