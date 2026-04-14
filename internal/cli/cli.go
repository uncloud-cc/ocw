package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/uncloud-cc/ocw/pkg/runner"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// Config holds CLI configuration parsed from flags
type Config struct {
	ValidateOnly bool
	WorkflowFile string
	EnvFile      string
	ShowSecrets  bool
	Force        bool
	Verbose      bool
	JobName      string
}

// Run executes the CLI with the given arguments
func Run(ctx context.Context, args []string, version string) error {
	cfg, err := parseArgs(args, version)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // --help or --version was shown
	}

	return Execute(ctx, cfg)
}

// parseArgs parses command-line arguments and returns the configuration
func parseArgs(args []string, version string) (*Config, error) {
	// Create a new flag set to avoid polluting the global one
	fs := flag.NewFlagSet("ocw", flag.ContinueOnError)

	// Define flags
	validateOnly := fs.Bool("validate", false, "Only validate the workflow file, don't run it")
	workflowFile := fs.String("f", "", "Workflow file to use (default: auto-discover)")
	envFile := fs.String("e", "", "Environment file to load (default: .env)")
	showVersion := fs.Bool("version", false, "Show version")
	help := fs.Bool("help", false, "Show help")
	showSecrets := fs.Bool("show-secrets", false, "Show secret values in output (unmask secrets)")
	force := fs.Bool("force", false, "Force remove existing containers with the same name")
	verbose := fs.Bool("verbose", false, "Enable verbose logging of internal steps")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "ocw - Open Container Workflow CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ocw <job-name>              Run a job from workflow files in current directory\n")
		fmt.Fprintf(os.Stderr, "  ocw -f <file> <job-name>    Run a job from a specific workflow file\n")
		fmt.Fprintf(os.Stderr, "  ocw -f <file>               Run direct flow control from a workflow file\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fs.PrintDefaults()
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
	reorderedArgs := reorderArgs(args)
	if err := fs.Parse(reorderedArgs); err != nil {
		return nil, err
	}

	if *showVersion {
		fmt.Printf("ocw version %s\n", version)
		return nil, nil
	}

	if *help {
		fs.Usage()
		return nil, nil
	}

	positionalArgs := fs.Args()

	// Determine workflow file and job name
	var workflowPath string
	var jobName string

	if *workflowFile != "" {
		// Explicit workflow file specified
		workflowPath = *workflowFile
		if len(positionalArgs) > 0 {
			jobName = positionalArgs[0]
		}
	} else if len(positionalArgs) > 0 {
		// Check if first arg is a file or a job name
		if isYAMLFile(positionalArgs[0]) {
			workflowPath = positionalArgs[0]
			if len(positionalArgs) > 1 {
				jobName = positionalArgs[1]
			}
		} else {
			// First arg is a job name, auto-discover workflow files
			jobName = positionalArgs[0]
		}
	}

	return &Config{
		ValidateOnly: *validateOnly,
		WorkflowFile: workflowPath,
		EnvFile:      *envFile,
		ShowSecrets:  *showSecrets,
		Force:        *force,
		Verbose:      *verbose,
		JobName:      jobName,
	}, nil
}

// Execute runs the workflow based on the parsed configuration
func Execute(ctx context.Context, cfg *Config) error {
	// Resolve workflow file
	workflowPath := cfg.WorkflowFile

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
		if cfg.JobName == "" {
			return listAvailableJobs(files)
		}

		// Find the job in workflow files
		workflowPath, err = findJobInFiles(files, cfg.JobName, cfg.Verbose)
		if err != nil {
			return err
		}
	}

	// Get the absolute path to the workflow file
	absWorkflowPath, err := filepath.Abs(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow path: %w", err)
	}

	// Get the directory containing the workflow file
	workflowDir := filepath.Dir(absWorkflowPath)

	// Parse and validate the workflow
	ocw, err := schema.ParseFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	if err := ocw.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed:\n%w", err)
	}

	if cfg.ValidateOnly {
		printWorkflowSummary(ocw)
		return nil
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, cancelling...")
		cancel()
	}()

	// Initialize runner
	r := runner.NewRunner(workflowDir)
	r.WorkflowFile = absWorkflowPath
	r.WithVerbose(cfg.Verbose)
	if cfg.EnvFile != "" {
		r.WithEnvFile(cfg.EnvFile)
	}
	if cfg.ShowSecrets {
		r.WithShowSecrets(true)
	}
	if cfg.Force {
		r.WithForce(true)
	}

	if cfg.JobName != "" {
		return r.RunJob(ctx, ocw, cfg.JobName)
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
