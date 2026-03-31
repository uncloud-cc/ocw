package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/uncloud-cc/ocw/pkg/runner"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

var (
	version = "dev"
)

// reorderArgs reorders command line arguments so that flags come before positional args.
// This allows flags to be placed anywhere: ocw myjob --show-secrets works the same as ocw --show-secrets myjob
func reorderArgs(args []string) []string {
	var flags []string
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		// Check if it's a flag (starts with -)
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Check if this flag takes a value (next arg doesn't start with -)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
		i++
	}

	return append(flags, positional...)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags first (before parsing)
	validateOnly := flag.Bool("validate", false, "Only validate the workflow file, don't run it")
	workflowFile := flag.String("f", "", "Workflow file to use (default: auto-discover)")
	envFile := flag.String("e", "", "Environment file to load (default: .env)")
	showVersion := flag.Bool("version", false, "Show version")
	help := flag.Bool("help", false, "Show help")
	showSecrets := flag.Bool("show-secrets", false, "Show secret values in output (unmask secrets)")
	force := flag.Bool("force", false, "Force remove existing containers with the same name")
	verbose := flag.Bool("verbose", false, "Enable verbose logging of internal steps")

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
		fmt.Fprintf(os.Stderr, "  ocw -validate -f my.yaml    Validate a workflow file\n")
		fmt.Fprintf(os.Stderr, "  ocw -show-secrets dev       Run 'dev' job showing secret values\n")
		fmt.Fprintf(os.Stderr, "  ocw -force dev              Run 'dev' job, replacing existing containers\n")
		fmt.Fprintf(os.Stderr, "  ocw -verbose dev            Run 'dev' job with verbose logging\n")
	}

	// Reorder args to support flags anywhere in the command line
	// Move all flags (starting with -) before positional arguments
	reorderedArgs := reorderArgs(os.Args[1:])
	flag.CommandLine.Parse(reorderedArgs)

	// Helper for early verbose output
	verboseLog := func(format string, args ...interface{}) {
		if *verbose {
			fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
		}
	}

	// Print system information if verbose mode is enabled
	if *verbose {
		printSystemInfo()
	}

	if *showVersion {
		fmt.Printf("ocw version %s\n", version)
		return nil
	}

	if *help {
		flag.Usage()
		return nil
	}

	args := flag.Args()
	verboseLog("Starting ocw with args: %v", args)

	// Determine workflow file(s) and job name
	var workflowPath string
	var jobName string
	var workflowDir string

	if *workflowFile != "" {
		// Explicit workflow file specified
		verboseLog("Using explicit workflow file: %s", *workflowFile)
		workflowPath = *workflowFile
		if len(args) > 0 {
			jobName = args[0]
			verboseLog("Job name from args: %s", jobName)
		}
	} else if len(args) > 0 {
		// Check if first arg is a file or a job name
		if isYAMLFile(args[0]) {
			verboseLog("First arg is a YAML file: %s", args[0])
			workflowPath = args[0]
			if len(args) > 1 {
				jobName = args[1]
				verboseLog("Job name from args: %s", jobName)
			}
		} else {
			// First arg is a job name, auto-discover workflow files
			verboseLog("First arg is job name: %s", args[0])
			jobName = args[0]
		}
	}

	// Auto-discover workflow files if no explicit file given
	if workflowPath == "" {
		verboseLog("Auto-discovering workflow files in current directory...")
		files, err := discoverWorkflowFiles(".")
		if err != nil {
			return fmt.Errorf("failed to discover workflow files: %w", err)
		}
		verboseLog("Found %d workflow file(s): %v", len(files), files)
		if len(files) == 0 {
			return fmt.Errorf("no workflow files (*.yaml, *.yml) found in current directory")
		}

		// If no job specified, list available jobs
		if jobName == "" {
			verboseLog("No job specified, listing available jobs")
			return listAvailableJobs(files)
		}

		// Find the job in workflow files
		verboseLog("Searching for job '%s' in workflow files...", jobName)
		workflowPath, err = findJobInFiles(files, jobName, *verbose)
		if err != nil {
			return err
		}
		verboseLog("Found job in workflow file: %s", workflowPath)
	}

	// Get the absolute path to the workflow file
	verboseLog("Resolving absolute path for: %s", workflowPath)
	absWorkflowPath, err := filepath.Abs(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow path: %w", err)
	}
	verboseLog("Absolute workflow path: %s", absWorkflowPath)

	// Get the directory containing the workflow file (this becomes /workflow)
	workflowDir = filepath.Dir(absWorkflowPath)
	verboseLog("Workflow directory: %s", workflowDir)

	// Parse and validate the workflow (silently unless there's an error)
	verboseLog("Parsing workflow file...")
	ocw, err := schema.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}
	verboseLog("Workflow parsed successfully: name=%s, schemaVersion=%s", ocw.Name, ocw.SchemaVersion)

	// Validate
	verboseLog("Validating workflow...")
	if err := ocw.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed:\n%w", err)
	}
	verboseLog("Workflow validation passed")

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
	verboseLog("Creating runner...")
	r := runner.NewRunner(workflowDir)
	r.WorkflowFile = absWorkflowPath
	r.WithVerbose(*verbose)
	if *envFile != "" {
		verboseLog("Using custom env file: %s", *envFile)
		r.WithEnvFile(*envFile)
	}
	if *showSecrets {
		r.WithShowSecrets(true)
	}
	if *force {
		r.WithForce(true)
	}

	if jobName != "" {
		verboseLog("Running job: %s", jobName)
		return r.RunJob(ctx, ocw, jobName)
	}

	// No job specified, run direct flow control
	verboseLog("No job specified, checking for direct flow control...")
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

	verboseLog("Running direct flow control")
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
func findJobInFiles(files []string, jobName string, verbose bool) (string, error) {
	for _, file := range files {
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose] Checking file for job '%s': %s\n", jobName, file)
		}
		ocw, err := schema.ParseFile(file)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   Failed to parse, skipping: %v\n", err)
			}
			continue // Skip files that fail to parse
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   Parsed successfully, found %d job(s)\n", len(ocw.GetJobNames()))
		}

		if ocw.GetJob(jobName) != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[verbose]   Found job '%s' in this file\n", jobName)
			}
			return file, nil
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[verbose]   Job '%s' not found in this file\n", jobName)
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

// printSystemInfo prints system diagnostic information when verbose mode is enabled
func printSystemInfo() {
	fmt.Fprintf(os.Stderr, "[verbose] === System Information ===\n")
	fmt.Fprintf(os.Stderr, "[verbose] Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stderr, "[verbose] Go version: %s\n", runtime.Version())

	// Current user
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	if user == "" {
		user = "unknown"
	}
	fmt.Fprintf(os.Stderr, "[verbose] Current user: %s\n", user)

	// Working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = "unknown"
	}
	fmt.Fprintf(os.Stderr, "[verbose] Working directory: %s\n", wd)

	// Docker version (with timeout to avoid hanging)
	fmt.Fprintf(os.Stderr, "[verbose] Checking Docker version...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "[verbose] Docker version check timed out (5s)\n")
		} else {
			fmt.Fprintf(os.Stderr, "[verbose] Docker version: unavailable (%v)\n", err)
		}
	} else {
		version := strings.TrimSpace(string(output))
		if version == "" {
			version = "unknown"
		}
		fmt.Fprintf(os.Stderr, "[verbose] Docker version: %s\n", version)
	}

	fmt.Fprintf(os.Stderr, "[verbose] ======================\n")
}
