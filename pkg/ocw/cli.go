package ocw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// TODO: Figure out how to make this testable

// CLIOptions holds configuration for the CLI.
type CLIOptions struct {
	EnvFiles    []string
	InputsFile  string
	OutputsFile string
	DebugMode   bool
	ShowSecrets bool
	CIMode      bool
	Stdout      io.Writer
	Stderr      io.Writer
	WorkingDir  string
	Runtime     Runtime
}

// CLI orchestrates the entire workflow execution lifecycle.
type CLI struct {
	opts CLIOptions
}

// NewCLI creates a new CLI instance.
func NewCLI(opts CLIOptions) *CLI {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.WorkingDir == "" {
		opts.WorkingDir, _ = os.Getwd()
	}
	return &CLI{opts: opts}
}

// Run executes the CLI with the given arguments.
func (c *CLI) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		bus := NewEventBus()
		cleanup := c.setupEventBusConsumers(bus)
		defer cleanup()
		return c.listAllJobs(bus)
	}

	filePath, jobName, err := c.resolveTarget(args)
	if err != nil {
		return err
	}

	parsed, err := c.parseAndValidate(filePath)
	if err != nil {
		return err
	}

	workflowDir, err := c.resolveWorkflowDir(filePath)
	if err != nil {
		return err
	}

	loadedFiles, err := c.loadEnvFiles(workflowDir)
	if err != nil {
		return err
	}

	state, secretValues, err := c.buildState(parsed, loadedFiles)
	if err != nil {
		return err
	}

	exec, ciMode, err := c.buildRuntime(parsed, workflowDir, state.RunID)
	if err != nil {
		return err
	}

	ctx, cancel := c.withSignalHandling(ctx)
	defer cancel()

	if jobName == "" && !HasDirectFlow(parsed) {
		if HasJobs(parsed) {
			return c.listJobsInFile(filePath, parsed)
		}
		return fmt.Errorf("no workflow flow or jobs found in %s", filePath)
	}

	engine, err := NewEngine(parsed, state, workflowDir, EngineOptions{
		Runtime:     exec,
		JobName:     jobName,
		LoadedFiles: loadedFiles,
	})
	if err != nil {
		exec.Close()
		return err
	}
	defer engine.Close()

	engine.Bus.SetSecrets(c.opts.ShowSecrets, secretValues)
	cleanup := c.setupEventBusConsumers(engine.Bus)
	defer cleanup()

	runErr := engine.Run(ctx)

	rawOutputs := c.selectRawOutputs(parsed, jobName)
	if runErr == nil && len(rawOutputs) > 0 {
		if err := c.resolveAndWriteOutputs(state, rawOutputs, engine.Bus); err != nil {
			return err
		}
	}

	if runErr == nil {
		c.waitForServices(ctx, exec, ciMode, engine.Bus)
	}

	if runErr != nil {
		return fmt.Errorf("run: %w", runErr)
	}
	return nil
}

// setupEventBusConsumers attaches logging consumers to an existing bus.
func (c *CLI) setupEventBusConsumers(bus *EventBus) func() {
	var wg sync.WaitGroup

	if c.opts.DebugMode {
		jsonCh := bus.Subscribe(64)
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := NewJSONLogger(c.opts.Stdout)
			l.Run(jsonCh)
		}()
	} else {
		prettyCh := bus.Subscribe(64)
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := NewPrettyPrinter(c.opts.Stdout)
			p.Run(prettyCh)
		}()
		bus.Debug("Initialized pretty-printer")
	}

	return func() {
		bus.Close()
		wg.Wait()
	}
}

// resolveTarget determines the workflow file and optional job name from args.
func (c *CLI) resolveTarget(args []string) (filePath, jobName string, err error) {
	if len(args) == 1 {
		arg := args[0]
		if strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml") {
			return arg, "", nil
		}
		discovered, err := c.findJobFile(arg)
		if err != nil {
			return "", "", err
		}
		return discovered, arg, nil
	}
	return args[0], args[1], nil
}

// parseAndValidate reads and validates the workflow file.
func (c *CLI) parseAndValidate(filePath string) (*schema.OCW, error) {
	parsed, err := ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := parsed.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return parsed, nil
}

// resolveWorkflowDir returns the absolute directory of the workflow file.
func (c *CLI) resolveWorkflowDir(filePath string) (string, error) {
	workflowDir, err := filepath.Abs(filepath.Dir(filePath))
	if err != nil {
		return "", fmt.Errorf("resolve workflow directory: %w", err)
	}
	return workflowDir, nil
}

// loadEnvFiles auto-loads .env files and explicit --env-file flags.
func (c *CLI) loadEnvFiles(workflowDir string) ([]string, error) {
	loadedSet := make(map[string]struct{})
	var loadedFiles []string

	for _, dotenvPath := range []string{
		filepath.Join(workflowDir, ".env"),
		".env",
	} {
		if _, err := os.Stat(dotenvPath); err == nil {
			_ = godotenv.Overload(dotenvPath)
			absPath, _ := filepath.Abs(dotenvPath)
			if _, ok := loadedSet[absPath]; !ok {
				loadedSet[absPath] = struct{}{}
				loadedFiles = append(loadedFiles, dotenvPath)
			}
		}
	}

	for _, ef := range c.opts.EnvFiles {
		if err := godotenv.Overload(ef); err != nil {
			return nil, fmt.Errorf("load env file %q: %w", ef, err)
		}
		absPath, _ := filepath.Abs(ef)
		if _, ok := loadedSet[absPath]; !ok {
			loadedSet[absPath] = struct{}{}
			loadedFiles = append(loadedFiles, ef)
		}
	}

	return loadedFiles, nil
}

// buildState creates the workflow state from parsed inputs and environment.
func (c *CLI) buildState(parsed *schema.OCW, loadedFiles []string) (*State, []string, error) {
	state, err := NewState(&parsed.Inputs, c.opts.InputsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("inputs: %w", err)
	}
	state.Meta["name"] = parsed.Name
	state.Steps = make(map[string]map[string]string)

	if c.opts.InputsFile != "" {
		loadedFiles = append(loadedFiles, c.opts.InputsFile)
	}

	var secretValues []string
	for _, v := range state.Secrets {
		secretValues = append(secretValues, v)
	}

	return state, secretValues, nil
}

// buildRuntime creates the execution runtime (Docker by default) and detects CI mode.
func (c *CLI) buildRuntime(parsed *schema.OCW, workflowDir string, runID string) (Runtime, bool, error) {
	var exec Runtime
	if c.opts.Runtime != nil {
		exec = c.opts.Runtime
	} else {
		var err error
		exec, err = NewDockerRuntime(parsed.Volumes, workflowDir, runID)
		if err != nil {
			return nil, false, fmt.Errorf("runtime: %w", err)
		}
	}

	ciMode := c.opts.CIMode
	if !ciMode && !IsTerminal(int(os.Stdout.Fd())) {
		ciMode = true
	}

	return exec, ciMode, nil
}

// withSignalHandling wraps the context with Ctrl+C / SIGTERM cancellation.
func (c *CLI) withSignalHandling(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}



// selectRawOutputs chooses the output map from either the job or the top-level schema.
func (c *CLI) selectRawOutputs(parsed *schema.OCW, jobName string) map[string]string {
	if jobName != "" {
		job := GetJob(parsed, jobName)
		if job != nil {
			return job.Outputs
		}
	}
	return parsed.Outputs
}

// resolveAndWriteOutputs resolves template expressions and optionally writes to disk.
func (c *CLI) resolveAndWriteOutputs(state *State, rawOutputs map[string]string, bus *EventBus) error {
	resolved, err := state.ResolveOutputs(rawOutputs)
	if err != nil {
		return fmt.Errorf("resolve outputs: %w", err)
	}
	bus.Event(&WorkflowOutputs{
		Title:   "Outputs",
		Outputs: resolved,
	})

	if c.opts.OutputsFile != "" {
		data, err := json.MarshalIndent(resolved, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal outputs: %w", err)
		}
		if err := os.WriteFile(c.opts.OutputsFile, data, 0644); err != nil {
			return fmt.Errorf("write outputs file: %w", err)
		}
	}
	return nil
}

// waitForServices shows active background services and blocks in interactive mode.
func (c *CLI) waitForServices(ctx context.Context, exec Runtime, ciMode bool, bus *EventBus) {
	if exec.HasActiveServices() {
		bus.Event(&ServicesOverview{Services: exec.ListServices()})
		if !ciMode {
			bus.Event(&Waiting{Message: "Press Ctrl+C to stop"})
			<-ctx.Done()
		}
	}
}

func (c *CLI) listAllJobs(bus *EventBus) error {
	bus.Debug("Starting listAllJobs()")

	files := c.findWorkflowFiles()
	if len(files) == 0 {
		bus.Debug("No workflow files found in current directory")
		return fmt.Errorf("no workflow files found in current directory")
	}

	bus.Debug(fmt.Sprintf("Found %d workflow files: %v", len(files), files))
	type jobInfo struct {
		file string
		name string
	}
	var allJobs []jobInfo
	var parsedFiles int

	for _, file := range files {
		schema, err := ParseFile(file)
		if err != nil {
			bus.Debug(fmt.Sprintf("[SKIPPING] Could not parse %s as ocw workflow file: %v", file, err))
			continue
		}
		if !HasJobs(schema) && !HasDirectFlow(schema) {
			bus.Debug(fmt.Sprintf("[SKIPPING] File does not contain any jobs or direct flow statements: %s", file))
			continue
		}
		parsedFiles++
		if HasJobs(schema) {
			jobNames := GetJobNames(schema)
			bus.Debug(fmt.Sprintf("Found jobs in file %s: %v", file, jobNames))

			for _, name := range jobNames {
				allJobs = append(allJobs, jobInfo{file: file, name: name})
			}
		}
	}

	if parsedFiles == 0 {
		bus.Debug("No valid ocw workflow files found in current directory")
		return fmt.Errorf("no workflow files found in current directory")
	}
	if len(allJobs) == 0 {
		bus.Debug("No jobs found in any workflow file in current directory")
		return fmt.Errorf("no jobs found in any workflow file")
	}

	bus.Debug(fmt.Sprintf("Found %d jobs in current directory: %v", len(allJobs), allJobs))
	fmt.Fprintln(c.opts.Stdout, "Available jobs:")
	fmt.Fprintln(c.opts.Stdout)

	currentFile := ""
	for _, job := range allJobs {
		if job.file != currentFile {
			currentFile = job.file
			fmt.Fprintf(c.opts.Stdout, "  %s:\n", job.file)
		}
		fmt.Fprintf(c.opts.Stdout, "    - %s\n", job.name)
	}

	fmt.Fprintln(c.opts.Stdout)
	fmt.Fprintln(c.opts.Stdout, "Run a job with:\n  ocw <job name>\n  ocw <file.yaml> <job name>")

	return nil
}

func (c *CLI) listJobsInFile(filePath string, schema *schema.OCW) error {
	if !HasJobs(schema) {
		return fmt.Errorf("no jobs found in %s", filePath)
	}
	fmt.Fprintf(c.opts.Stdout, "Available jobs in %s:\n\n", filePath)
	for _, name := range GetJobNames(schema) {
		fmt.Fprintf(c.opts.Stdout, "  - %s\n", name)
	}
	fmt.Fprintln(c.opts.Stdout)
	fmt.Fprintf(c.opts.Stdout, "Run a job with:\n  ocw <job name>\n  ocw %s <job name>\n\n", filePath)
	return nil
}

func (c *CLI) findJobFile(jobName string) (string, error) {
	files := c.findWorkflowFiles()
	for _, file := range files {
		schema, err := ParseFile(file)
		if err != nil {
			continue
		}
		if HasJobs(schema) {
			if job := GetJob(schema, jobName); job != nil {
				return file, nil
			}
		}
	}
	return "", fmt.Errorf("job %q not found in any workflow file", jobName)
}

func (c *CLI) findWorkflowFiles() []string {
	files, _ := filepath.Glob(filepath.Join(c.opts.WorkingDir, "*.yaml"))
	ymlFiles, _ := filepath.Glob(filepath.Join(c.opts.WorkingDir, "*.yml"))
	all := append(files, ymlFiles...)
	var result []string
	for _, f := range all {
		if !strings.HasPrefix(filepath.Base(f), ".") {
			result = append(result, f)
		}
	}
	return result
}
