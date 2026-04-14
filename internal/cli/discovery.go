package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// isYAMLFile checks if a path looks like a YAML file
func isYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// discoverWorkflowFiles finds all YAML files in a directory
func discoverWorkflowFiles(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isYAMLFile(name) {
			files = append(files, filepath.Join(dir, name))
		}
	}

	return files, nil
}

// findJobInFiles searches for a job name across multiple workflow files
func findJobInFiles(files []string, jobName string, verbose bool) (string, error) {
	for _, file := range files {
		ocw, err := schema.ParseFile(file)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", file, err)
			}
			continue // Skip files that fail to parse
		}

		if ocw.GetJob(jobName) != nil {
			return file, nil
		}
	}

	// Job not found, list available jobs
	return "", listAvailableJobsWithError(files, jobName)
}
