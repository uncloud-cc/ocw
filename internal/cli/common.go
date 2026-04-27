package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// discoverWorkflowFile attempts to find a workflow file in the current directory
// It looks for any *.yaml or *.yml file and returns the first one found
func discoverWorkflowFile() (string, error) {
	// Look for any yaml/yml files
	patterns := []string{"*.yaml", "*.yml"}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(files) > 0 {
			return files[0], nil
		}
	}

	return "", fmt.Errorf("no workflow file found (looked for: *.yaml, *.yml)")
}

// loadEnvFile loads environment variables from a file
func loadEnvFile(path string) (map[string]string, error) {
	env := make(map[string]string)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			if len(value) > 1 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			env[key] = value
		}
	}

	return env, nil
}
