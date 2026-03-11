package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/opencontainerworkflow/ocw/pkg/runner"
	"github.com/opencontainerworkflow/ocw/pkg/schema"
)

var (
	version = "dev"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags
	validateOnly := flag.Bool("validate", false, "Only validate the workflow file, don't run it")
	workflowFile := flag.String("f", "", "Workflow file to use (default: auto-discover)")
	envFile := flag.String("e", "", "Environment file to load (default: .env)")
	inputsFile := flag.String("i", "", "Inputs file to load (YAML format)")
	showVersion := flag.Bool("version", false, "Show version")
	help := flag.Bool("help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ocw - Open Container Workflow CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ocw <job-name>              Run a job from workflow files in current directory\n")
		fmt.Fprintf(os.Stderr, "  ocw -f <file> <job-name>    Run a job from a specific workflow file\n")
		fmt.Fprintf(os.Stderr, "  ocw -f <file>               Run direct flow control from a workflow file\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ocw dev                     Run the 'dev' job\n")
		fmt.Fprintf(os.Stderr, "  ocw build                   Run the 'build' job\n")
		fmt.Fprintf(os.Stderr, "  ocw -f workflow.yaml dev    Run 'dev' job from workflow.yaml\n")
		fmt.Fprintf(os.Stderr, "  ocw -e staging.env dev      Run 'dev' job with staging.env\n")
		fmt.Fprintf(os.Stderr, "  ocw -i inputs.yaml deploy   Run 'deploy' job with inputs from YAML\n")
		fmt.Fprintf(os.Stderr, "  ocw -validate -f my.yaml    Validate a workflow file\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("ocw version %s\n", version)
		return nil
	}

	if *help {
		flag.Usage()
		return nil
	}

	args := flag.Args()

	// Determine workflow file(s) and job name
	var workflowPath string
	var jobName string
	var workflowDir string

	if *workflowFile != "" {
		// Explicit workflow file specified
		workflowPath = *workflowFile
		if len(args) > 0 {
			jobName = args[0]
		}
	} else if len(args) > 0 {
		// Check if first arg is a file or a job name
		if isYAMLFile(args[0]) {
			workflowPath = args[0]
			if len(args) > 1 {
				jobName = args[1]
			}
		} else {
			// First arg is a job name, auto-discover workflow files
			jobName = args[0]
		}
	}

	// Auto-discover workflow files if no explicit file given
	if workflowPath == "" {
		files, err := discoverWorkflowFiles(".")
		if err != nil {
			return fmt.Errorf("failed to discover workflow files: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no workflow files (*.yaml, *.yml) found in current directory")
		}

		// If no job specified, list available jobs
		if jobName == "" {
			return listAvailableJobs(files)
		}

		// Find the job in workflow files
		workflowPath, err = findJobInFiles(files, jobName)
		if err != nil {
			return err
		}
	}

	// Get the absolute path to the workflow file
	absWorkflowPath, err := filepath.Abs(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow path: %w", err)
	}

	// Get the directory containing the workflow file (this becomes /workflow)
	workflowDir = filepath.Dir(absWorkflowPath)

	// Parse and validate the workflow (silently unless there's an error)
	ocw, err := schema.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Validate
	if err := ocw.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed:\n%w", err)
	}

	if *validateOnly {
		printWorkflowSummary(ocw)
		return nil
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, cancelling...")
		cancel()
	}()

	// Run the workflow or job
	r := runner.NewRunner(workflowDir)
	if *envFile != "" {
		r.WithEnvFile(*envFile)
	}
	if *inputsFile != "" {
		r.WithInputsFile(*inputsFile)
	}

	if jobName != "" {
		return r.RunJob(ctx, ocw, jobName)
	}

	// No job specified, run direct flow control
	if !ocw.HasDirectFlow() {
		// No direct flow, list available jobs
		fmt.Printf("No direct flow control in workflow. Available jobs:\n")
		for _, name := range ocw.GetJobNames() {
			job := ocw.GetJob(name)
			if job.Name != "" {
				fmt.Printf("  - %s (%s)\n", name, job.Name)
			} else {
				fmt.Printf("  - %s\n", name)
			}
		}
		return fmt.Errorf("specify a job name to run")
	}

	return r.Run(ctx, ocw)
}

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
func findJobInFiles(files []string, jobName string) (string, error) {
	for _, file := range files {
		ocw, err := schema.ParseFile(file)
		if err != nil {
			continue // Skip files that fail to parse
		}

		if ocw.GetJob(jobName) != nil {
			return file, nil
		}
	}

	// Job not found, list available jobs
	return "", listAvailableJobsWithError(files, jobName)
}

// listAvailableJobs prints available jobs from workflow files
func listAvailableJobs(files []string) error {
	fmt.Printf("Available jobs:\n\n")

	jobsByFile := make(map[string][]string)

	for _, file := range files {
		ocw, err := schema.ParseFile(file)
		if err != nil {
			continue
		}

		for _, name := range ocw.GetJobNames() {
			job := ocw.GetJob(name)
			displayName := name
			if job.Name != "" {
				displayName = fmt.Sprintf("%s (%s)", name, job.Name)
			}
			jobsByFile[file] = append(jobsByFile[file], displayName)
		}
	}

	if len(jobsByFile) == 0 {
		fmt.Printf("  No jobs found in workflow files.\n")
		return fmt.Errorf("no jobs available")
	}

	// Sort files for consistent output
	sortedFiles := make([]string, 0, len(jobsByFile))
	for f := range jobsByFile {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Strings(sortedFiles)

	for _, file := range sortedFiles {
		jobs := jobsByFile[file]
		fmt.Printf("  %s:\n", file)
		for _, job := range jobs {
			fmt.Printf("    - %s\n", job)
		}
	}

	fmt.Printf("\nUsage: ocw <job-name>\n")
	return fmt.Errorf("specify a job name to run")
}

// listAvailableJobsWithError lists available jobs and returns an error for missing job
func listAvailableJobsWithError(files []string, missingJob string) error {
	fmt.Printf("Job %q not found.\n\n", missingJob)
	listAvailableJobs(files)
	return fmt.Errorf("job %q not found", missingJob)
}

func printWorkflowSummary(ocw *schema.OCW) {
	fmt.Printf("Workflow Summary:\n")
	fmt.Printf("  Name: %s\n", ocw.Name)
	if ocw.ID != "" {
		fmt.Printf("  ID: %s\n", ocw.ID)
	}
	if ocw.Description != "" {
		fmt.Printf("  Description: %s\n", ocw.Description)
	}
	fmt.Printf("  Schema Version: %s\n", ocw.SchemaVersion)

	if ocw.HasDirectFlow() {
		fmt.Printf("  Flow Type: %s\n", ocw.GetFlowType())
		steps := ocw.GetSteps()
		if steps != nil {
			fmt.Printf("  Top-level Steps: %d\n", len(steps))
		}
	}

	if ocw.HasJobs() {
		fmt.Printf("  Jobs: %d\n", len(ocw.Jobs))
		for name, job := range ocw.Jobs {
			if job.Name != "" {
				fmt.Printf("    - %s (%s)\n", name, job.Name)
			} else {
				fmt.Printf("    - %s\n", name)
			}
		}
	}

	if len(ocw.Inputs) > 0 {
		fmt.Printf("  Inputs: %d\n", len(ocw.Inputs))
		for name, input := range ocw.Inputs {
			fmt.Printf("    - %s (%s)\n", name, input.GetType())
		}
	}

	if len(ocw.Env) > 0 {
		fmt.Printf("  Environment Variables: %d\n", len(ocw.Env))
	}

	if len(ocw.Secrets) > 0 {
		fmt.Printf("  Secrets: %d\n", len(ocw.Secrets))
	}

	if len(ocw.Outputs) > 0 {
		fmt.Printf("  Outputs: %d\n", len(ocw.Outputs))
	}
}
