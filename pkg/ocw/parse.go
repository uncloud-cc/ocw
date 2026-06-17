// Package parser provides functionality for parsing OCW workflow files.
package ocw

import (
	"fmt"
	"os"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ParseFile reads an OCW workflow file and parses it into an OCW instance.
// Returns an error if the file cannot be read or if the YAML is invalid.
func ParseFile(path string) (*schema.OCW, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("workflow file not found: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file: %w", err)
	}

	return ParseBytes(data)
}

// ParseBytes parses OCW workflow data from a byte slice.
// Returns an error if the YAML is invalid or missing required fields.
func ParseBytes(data []byte) (*schema.OCW, error) {
	ocw, err := schema.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow: %w", err)
	}

	return ocw, nil
}

// ParseString parses OCW workflow data from a string.
// Convenience wrapper around ParseBytes.
func ParseString(data string) (*schema.OCW, error) {
	return ParseBytes([]byte(data))
}

// ValidateAndParseFile parses a YAML file and validates the result.
// Returns an error if the file cannot be read, parsed, or validated.
func ValidateAndParseFile(path string) (*schema.OCW, error) {
	ocw, err := ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if err := ocw.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return ocw, nil
}
