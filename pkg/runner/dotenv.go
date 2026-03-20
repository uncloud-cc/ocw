package runner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotEnv holds parsed environment variables from .env files
type DotEnv struct {
	// Vars contains all KEY=value pairs from .env files
	Vars map[string]string
}

// LoadDotEnv loads environment variables from .env file in the workflow directory.
// If no .env file exists, returns an empty DotEnv (not an error).
// Lines starting with # are comments, empty lines are ignored.
// Format: KEY=value (no export prefix, quotes are preserved as-is)
func LoadDotEnv(workflowDir string) (*DotEnv, error) {
	envFile := filepath.Join(workflowDir, ".env")
	return LoadDotEnvFile(envFile)
}

// LoadDotEnvFile loads environment variables from a specific .env file.
// If the file doesn't exist, returns an empty DotEnv (not an error).
// Lines starting with # are comments, empty lines are ignored.
// Format: KEY=value (no export prefix, quotes are preserved as-is)
func LoadDotEnvFile(path string) (*DotEnv, error) {
	dotenv := &DotEnv{
		Vars: make(map[string]string),
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty dotenv
			return dotenv, nil
		}
		return nil, err
	}

	if err := dotenv.loadFile(path); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", path, err)
	}

	return dotenv, nil
}

// loadFile parses a single .env file and adds its values to the DotEnv
func (d *DotEnv) loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip export prefix if present (e.g., "export FOO=bar")
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
			line = strings.TrimSpace(line)
		}

		// Find the first = sign
		idx := strings.Index(line, "=")
		if idx == -1 {
			// Line without = is invalid, skip it
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := line[idx+1:]

		// Handle quoted values
		value = unquoteValue(value)

		if key != "" {
			d.Vars[key] = value
		}
	}

	return scanner.Err()
}

// unquoteValue removes surrounding quotes from a value if present
// Supports both single and double quotes
func unquoteValue(s string) string {
	s = strings.TrimSpace(s)

	// Check for double quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		// Handle escape sequences in double-quoted strings
		s = strings.ReplaceAll(s, `\"`, `"`)
		s = strings.ReplaceAll(s, `\\`, `\`)
		s = strings.ReplaceAll(s, `\n`, "\n")
		s = strings.ReplaceAll(s, `\t`, "\t")
		return s
	}

	// Check for single quotes (no escape processing)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}

	return s
}

// Get returns a value from the .env, or empty string if not found
func (d *DotEnv) Get(key string) string {
	return d.Vars[key]
}

// Has returns true if a key exists in the .env
func (d *DotEnv) Has(key string) bool {
	_, ok := d.Vars[key]
	return ok
}
