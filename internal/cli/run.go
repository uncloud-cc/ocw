package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"github.com/uncloud-cc/ocw/pkg/ocw"
)

// RunConfig holds the configuration for running a workflow
type RunConfig struct {
	WorkflowFile string
	FileArg      string // File argument passed directly (e.g., ocw myfile.yaml)
	EnvFile      string
	JobName      string
	ValidateOnly bool
	ShowSecrets  bool
	Force        bool
	Verbose      bool
	JSONOutput   bool
}

// RunWorkflow executes a workflow with the given configuration
func RunWorkflow(ctx context.Context, config *RunConfig) error {
	// Determine what to run
	var file string
	var jobName string

	// If FileArg is provided (e.g., ocw myfile.yaml), use it as the file
	if config.FileArg != "" {
		file = config.FileArg
		jobName = config.JobName
	} else if config.WorkflowFile != "" {
		// If -f flag is provided, use it
		file = config.WorkflowFile
		jobName = config.JobName
	} else if config.JobName != "" {
		// If a job name is provided, search for it in all YAML files
		foundFile, err := findJobInWorkflowFiles(config.JobName)
		if err != nil {
			return err
		}
		file = foundFile
		jobName = config.JobName
	} else {
		// No file or job specified - list all available jobs
		return listAllJobs()
	}

	// Get absolute path for better error messages
	absFile, err := filepath.Abs(file)
	if err != nil {
		absFile = file
	}

	// Parse the workflow file
	ocwSchema, err := ocw.ParseFile(file)
	if err != nil {
		return fmt.Errorf("failed to parse workflow file %s: %w", absFile, err)
	}

	// Validate if requested
	if config.ValidateOnly || viper.GetBool("validate") {
		if err := ocwSchema.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		fmt.Printf("✓ Workflow file is valid: %s\n", absFile)
		return nil
	}

	// Create logger
	logger := createLogger(config)

	// For now, use DummyRuntime - the Runtime wraps it
	dummy := ocw.NewDummyRuntime(logger)
	rt := ocw.NewRuntime(dummy, logger)

	// Run the workflow
	start := time.Now()
	logger.Printf("Running workflow: %s", absFile)
	if jobName != "" {
		logger.Printf("Job: %s", jobName)
	}

	result, err := rt.RunWorkflow(ctx, ocwSchema, jobName)
	duration := time.Since(start)

	// Shutdown services
	if shutdownErr := rt.Shutdown(ctx); shutdownErr != nil {
		logger.Printf("Warning: error during shutdown: %v", shutdownErr)
	}

	// Report results
	if err != nil {
		return fmt.Errorf("workflow failed after %v: %w", duration, err)
	}

	if result.Status == ocw.StatusFailed {
		return fmt.Errorf("workflow failed after %v", duration)
	}

	logger.Printf("Workflow completed successfully in %v", duration)
	return nil
}

// listAllJobs lists all available jobs from all YAML files in the current directory
func listAllJobs() error {
	// Get all YAML files in the current directory
	files, err := filepath.Glob("*.yaml")
	if err != nil {
		return fmt.Errorf("failed to list YAML files: %w", err)
	}

	ymlFiles, err := filepath.Glob("*.yml")
	if err != nil {
		return fmt.Errorf("failed to list YML files: %w", err)
	}

	files = append(files, ymlFiles...)

	if len(files) == 0 {
		return fmt.Errorf("no YAML files found in current directory")
	}

	// Collect all jobs from all files
	type jobInfo struct {
		file string
		name string
	}
	var allJobs []jobInfo

	for _, file := range files {
		ocwSchema, err := ocw.ParseFile(file)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		if ocw.HasJobs(ocwSchema) {
			jobNames := ocw.GetJobNames(ocwSchema)
			for _, name := range jobNames {
				allJobs = append(allJobs, jobInfo{file: file, name: name})
			}
		}
	}

	if len(allJobs) == 0 {
		return fmt.Errorf("no jobs found in any YAML file")
	}

	// Print the jobs
	fmt.Println("Available jobs:")
	fmt.Println()

	currentFile := ""
	for _, job := range allJobs {
		if job.file != currentFile {
			currentFile = job.file
			fmt.Printf("  %s:\n", job.file)
		}
		fmt.Printf("    - %s\n", job.name)
	}

	fmt.Println()
	fmt.Println("Run a job with: ocw <job-name>")

	return nil
}

// findJobInWorkflowFiles searches all YAML files in the current directory for a job with the given name
func findJobInWorkflowFiles(jobName string) (string, error) {
	// Get all YAML files in the current directory
	files, err := filepath.Glob("*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to list YAML files: %w", err)
	}

	ymlFiles, err := filepath.Glob("*.yml")
	if err != nil {
		return "", fmt.Errorf("failed to list YML files: %w", err)
	}

	files = append(files, ymlFiles...)

	if len(files) == 0 {
		return "", fmt.Errorf("no YAML files found in current directory")
	}

	// Search each file for the job
	for _, file := range files {
		ocwSchema, err := ocw.ParseFile(file)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		if ocw.HasJobs(ocwSchema) {
			job := ocw.GetJob(ocwSchema, jobName)
			if job != nil {
				return file, nil
			}
		}
	}

	return "", fmt.Errorf("job %q not found in any YAML file", jobName)
}
