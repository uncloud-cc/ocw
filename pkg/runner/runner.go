package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// StepResult represents the result of a step execution
type StepResult struct {
	Name     string
	Status   StepStatus
	Duration time.Duration
	Error    error
}

// StepStatus represents the status of a step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// BuiltImage tracks images built during workflow execution
type BuiltImage struct {
	StepID    string
	ImageName string
}

// Runner executes OCW workflows
type Runner struct {
	// WorkflowDir is the directory containing the workflow file (mounted as /workflow)
	WorkflowDir string
	// WorkflowFile is the path to the workflow YAML file
	WorkflowFile string
	// EnvFile is the path to the .env file to load (empty means use default .env)
	EnvFile string
	// Output function for logging (defaults to fmt.Printf)
	Output func(format string, args ...any)
	// Verbose enables detailed logging of internal operations
	verbose bool
	// Docker client for container operations
	docker *Docker
	// builtImages tracks images built by build steps (keyed by step ID)
	builtImages map[string]string
	// builtImagesMu protects builtImages map
	builtImagesMu sync.RWMutex
	// backgroundContainers tracks running background containers for cleanup
	backgroundContainers []string
	// backgroundMu protects backgroundContainers slice
	backgroundMu sync.Mutex
	// networkName is the network created for this job (for container-to-container communication)
	networkName string
	// exposedServices tracks services that have been exposed to the host
	exposedServices []ExposedService
	// exposedMu protects exposedServices slice
	exposedMu sync.Mutex
	// templateCtx holds template interpolation context
	templateCtx *TemplateContext
	// styles provides styled output formatting
	styles *Styles
	// secretEnvKeys tracks which env keys are marked as secrets (for masking)
	secretEnvKeys map[string]bool
	// secretValues stores the actual secret values for masking
	secretValues []string
	// showSecrets disables secret masking when true
	showSecrets bool
	// force removes existing containers with the same name
	force bool
	// runID is a unique identifier for this workflow execution (enables parallel runs)
	runID string
	// reloader manages watched containers for watch mode
	reloader *Reloader
	// builtImageConfigs stores build configs for rebuilds in watch mode
	builtImageConfigs map[string]*schema.BuildConfig

	// resolvedVolumes stores resolved workflow volumes
	resolvedVolumes map[string]*ResolvedVolume

	// currentJobVolumes stores the current job's volume references during execution
	currentJobVolumes schema.VolumeRefs
}

// NewRunner creates a new workflow runner
func NewRunner(workflowDir string) *Runner {
	styles := NewStyles()
	output := func(format string, args ...any) {
		fmt.Printf(format, args...)
	}
	return &Runner{
		WorkflowDir:          workflowDir,
		Output:               output,
		docker:               NewDocker(output, styles, nil),
		builtImages:          make(map[string]string),
		builtImageConfigs:    make(map[string]*schema.BuildConfig),
		backgroundContainers: make([]string, 0),
		exposedServices:      make([]ExposedService, 0),
		templateCtx:          NewTemplateContext(),
		styles:               styles,
		secretEnvKeys:        make(map[string]bool),
	}
}

// WithVerbose enables or disables verbose logging
func (r *Runner) WithVerbose(verbose bool) *Runner {
	r.verbose = verbose
	r.docker.WithVerbose(verbose)
	return r
}

// WithEnvFile sets a custom .env file path
func (r *Runner) WithEnvFile(envFile string) *Runner {
	r.EnvFile = envFile
	return r
}

// WithShowSecrets sets whether to show secret values in output (disable masking)
func (r *Runner) WithShowSecrets(show bool) *Runner {
	r.showSecrets = show
	return r
}

// WithForce sets whether to force remove existing containers with the same name
func (r *Runner) WithForce(force bool) *Runner {
	r.force = force
	return r
}

// logVerbose logs a message if verbose mode is enabled
func (r *Runner) logVerbose(format string, args ...any) {
	if r.verbose {
		r.Output("  %s %s\n", r.styles.Dim("[verbose]"), r.styles.Dim(fmt.Sprintf(format, args...)))
	}
}

// initReloader initializes the reloader if not already done
func (r *Runner) initReloader() {
	if r.reloader == nil {
		r.reloader = NewReloader(r)
	}
}

// loadDotEnv loads .env file from the workflow directory and populates
// the template context with env vars and secrets.
// Workflow env vars are loaded first as defaults, then .env overrides them.
func (r *Runner) loadDotEnv(workflowEnv schema.Env) error {
	// Collect all secret values for masking (defaults + overrides)
	var secretValues []string

	// First, load workflow env vars as defaults
	for key, envVar := range workflowEnv {
		r.templateCtx.Env[key] = envVar.Value
		if envVar.IsSecret {
			r.secretEnvKeys[key] = true
			// Collect default secret value for masking (if not empty)
			if envVar.Value != "" {
				secretValues = append(secretValues, envVar.Value)
			}
		}
	}

	// Load .env file to override workflow defaults
	var dotenv *DotEnv
	var err error
	var envFileName string

	if r.EnvFile != "" {
		// Custom env file specified
		dotenv, err = LoadDotEnvFile(r.EnvFile)
		envFileName = r.EnvFile
	} else {
		// Default: load .env from workflow directory
		dotenv, err = LoadDotEnv(r.WorkflowDir)
		envFileName = ".env"
	}

	if err != nil {
		return err
	}

	// Override workflow defaults with .env values
	for key, value := range dotenv.Vars {
		// Check if this key was marked as a secret in workflow
		if r.secretEnvKeys[key] {
			// It's a secret - update env and collect for masking
			r.templateCtx.Env[key] = value
			// Collect .env secret value for masking
			secretValues = append(secretValues, value)
		} else {
			// Regular env var - only update env context
			r.templateCtx.Env[key] = value
		}
	}

	// Store secret values for masking in outputs
	r.secretValues = secretValues

	// Set secrets on docker client for masking (all secret values)
	// Only mask if showSecrets is false
	if !r.showSecrets {
		r.docker.SetSecrets(secretValues)
	} else {
		r.docker.SetSecrets(nil) // No masking
	}

	if len(dotenv.Vars) > 0 {
		r.Output("  %s %s\n", r.styles.Dim(fmt.Sprintf("Loaded %d variable(s) from", len(dotenv.Vars))), r.styles.Value(envFileName))
	}

	return nil
}

// reloadWorkflowEnv re-parses the workflow file and reloads environment variables
// This is called during watch mode reloads when the workflow file or .env file changes
func (r *Runner) reloadWorkflowEnv() error {
	if r.WorkflowFile == "" {
		return nil // No workflow file to reload
	}

	// Re-parse the workflow file
	ocw, err := schema.ParseFile(r.WorkflowFile)
	if err != nil {
		return fmt.Errorf("failed to re-parse workflow file: %w", err)
	}

	// Reload .env file with new workflow env as defaults
	if err := r.loadDotEnv(ocw.Env); err != nil {
		return fmt.Errorf("failed to reload .env: %w", err)
	}

	return nil
}

// registerBuiltImage registers an image built by a step
func (r *Runner) registerBuiltImage(stepID, imageName string) {
	r.builtImagesMu.Lock()
	defer r.builtImagesMu.Unlock()
	r.builtImages[stepID] = imageName
}

// getBuiltImage returns the image name for a step ID
func (r *Runner) getBuiltImage(stepID string) (string, bool) {
	r.builtImagesMu.RLock()
	defer r.builtImagesMu.RUnlock()
	img, ok := r.builtImages[stepID]
	return img, ok
}

// registerBackgroundContainer adds a container to the cleanup list
func (r *Runner) registerBackgroundContainer(name string) {
	r.backgroundMu.Lock()
	defer r.backgroundMu.Unlock()
	r.backgroundContainers = append(r.backgroundContainers, name)
}

// createJobNetwork creates a network for the current job
func (r *Runner) createJobNetwork(ctx context.Context, jobName string) error {
	r.networkName = fmt.Sprintf("ocw-%s-%d", SanitizeName(jobName), time.Now().UnixNano())
	// Network creation is silent - only show errors
	return r.docker.CreateNetwork(ctx, NetworkCreateOptions{
		Name:   r.networkName,
		Driver: "bridge",
	})
}

// printOutputs interpolates and prints workflow/job outputs
func (r *Runner) printOutputs(outputs map[string]string) error {
	if len(outputs) == 0 {
		return nil
	}

	// Interpolate all outputs first
	interpolatedOutputs := make(map[string]string)
	for key, valueExpr := range outputs {
		value, err := r.templateCtx.Interpolate(valueExpr)
		if err != nil {
			interpolatedOutputs[key] = fmt.Sprintf("<error: %v>", err)
			continue
		}
		// Mask secrets in output values unless showSecrets is enabled
		if !r.showSecrets {
			value = r.maskSecretsInString(value)
		}
		interpolatedOutputs[key] = value
	}

	r.Output(r.styles.OutputsBox("Outputs", interpolatedOutputs))
	return nil
}

// maskSecretsInString replaces secret values with [secret]
func (r *Runner) maskSecretsInString(text string) string {
	result := text
	for _, secret := range r.secretValues {
		if secret != "" {
			result = strings.ReplaceAll(result, secret, "[secret]")
		}
	}
	return result
}

// outputsDir returns the path to the .ocw-outputs directory
func (r *Runner) outputsDir() string {
	return filepath.Join(r.WorkflowDir, ".ocw-outputs")
}

// ensureOutputsDir creates the .ocw-outputs directory if it doesn't exist
func (r *Runner) ensureOutputsDir() error {
	dir := r.outputsDir()
	return os.MkdirAll(dir, 0755)
}

// cleanupOutputsDir removes the .ocw-outputs directory
func (r *Runner) cleanupOutputsDir() {
	dir := r.outputsDir()
	if err := os.RemoveAll(dir); err != nil {
		r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to clean up outputs directory: %v", err)))
	}
}

// getStepOutputPath returns the path to the output file for a step
func (r *Runner) getStepOutputPath(stepID string) string {
	return filepath.Join(r.outputsDir(), stepID)
}

// parseStepOutputs reads the output file for a step and registers the outputs
func (r *Runner) parseStepOutputs(stepID string) error {
	outputPath := r.getStepOutputPath(stepID)

	file, err := os.Open(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No outputs file - that's fine, step just didn't write any outputs
			return nil
		}
		return fmt.Errorf("failed to open outputs file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value format
		idx := strings.Index(line, "=")
		if idx == -1 {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: invalid output format at line %d: %s", lineNum, line)))
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		if key == "" {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: empty key at line %d", lineNum)))
			continue
		}

		r.templateCtx.SetStepOutput(stepID, key, value)
		r.Output("  %s %s%s%s\n", r.styles.Dim("Output:"), r.styles.OutputKey(key), r.styles.Dim("="), r.styles.OutputValue(value))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading outputs file: %w", err)
	}

	return nil
}

// Run executes an OCW workflow (direct flow control, not a specific job)
func (r *Runner) Run(ctx context.Context, ocw *schema.OCW) error {
	r.logVerbose("Starting workflow execution")

	// Generate unique run ID for this workflow execution (enables parallel runs)
	r.runID = gonanoid.Must(5)
	r.logVerbose("Generated run ID: %s", r.runID)

	// Ensure background containers and outputs are cleaned up when done
	defer r.cleanupBackgroundContainers()
	defer r.cleanupOutputsDir()

	// Print styled job header
	r.Output(r.styles.JobBox(string(ocw.Name), "", string(ocw.Description)))
	r.Output("  %s %s\n\n", r.styles.Label("Directory:"), r.styles.Value(r.WorkflowDir))

	// Load .env file if present (passing workflow env as defaults)
	r.logVerbose("Loading environment variables")
	if err := r.loadDotEnv(ocw.Env); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	// Set up template context with workflow metadata
	r.logVerbose("Setting up template context")
	r.templateCtx.Workflow = WorkflowMeta{
		Name:        string(ocw.Name),
		Description: string(ocw.Description),
		ID:          string(ocw.ID),
	}

	// Create outputs directory for step outputs
	r.logVerbose("Creating outputs directory: %s", r.outputsDir())
	if err := r.ensureOutputsDir(); err != nil {
		return fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Create a network for this workflow (enables container-to-container communication)
	workflowName := SanitizeName(ocw.Name)
	if workflowName == "" {
		workflowName = "workflow"
	}
	r.logVerbose("Creating job network for workflow: %s", workflowName)
	if err := r.createJobNetwork(ctx, workflowName); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	r.logVerbose("Network created: %s", r.networkName)

	// Resolve workflow volumes
	if len(ocw.Volumes) > 0 {
		r.logVerbose("Resolving %d workflow volumes", len(ocw.Volumes))
		if err := r.resolveVolumes(ocw.Volumes); err != nil {
			return fmt.Errorf("failed to resolve volumes: %w", err)
		}
	}

	start := time.Now()
	flowType := ocw.GetFlowType()
	r.logVerbose("Starting flow execution (type: %s)", flowType)

	var err error
	switch flowType {
	case "parallel":
		err = r.runParallel(ctx, ocw.Parallel)
	case "sequence":
		err = r.runSequence(ctx, ocw.Sequence)
	case "switch":
		err = r.runSwitch(ctx, *ocw.Switch, ocw.Case, ocw.Default)
	default:
		err = fmt.Errorf("no direct flow control found (use 'ocw <job-name>' to run a specific job)")
	}

	// Print exposed services summary
	r.printExposedServices()

	// Print workflow outputs (if any and no error)
	if err == nil {
		r.printOutputs(ocw.Outputs)
	}

	duration := time.Since(start)
	r.logVerbose("Workflow execution completed in %v", duration.Round(time.Millisecond))
	r.Output(r.styles.CompletionBanner(string(ocw.Name), duration.Round(time.Millisecond).String(), err == nil))

	// If there are background containers running and no errors, wait for interrupt
	if err == nil && r.hasBackgroundContainers() {
		r.waitForInterrupt()
	}

	return err
}

// RunJob executes a specific job from an OCW workflow
func (r *Runner) RunJob(ctx context.Context, ocw *schema.OCW, jobName string) error {
	r.logVerbose("Starting job execution: %s", jobName)

	// Generate unique run ID for this workflow execution (enables parallel runs)
	r.runID = gonanoid.Must(5)
	r.logVerbose("Generated run ID: %s", r.runID)

	// Ensure background containers and outputs are cleaned up when done
	defer r.cleanupBackgroundContainers()
	defer r.cleanupOutputsDir()

	job := ocw.GetJob(jobName)
	if job == nil {
		return fmt.Errorf("job %q not found in workflow", jobName)
	}

	displayName := jobName
	if job.Name != "" {
		displayName = string(job.Name)
	}

	// Print styled job header
	r.Output(r.styles.JobBox(displayName, string(ocw.Name), string(job.Description)))
	r.Output("  %s %s\n\n", r.styles.Label("Directory:"), r.styles.Value(r.WorkflowDir))

	// Load .env file if present (passing workflow env as defaults)
	r.logVerbose("Loading environment variables")
	if err := r.loadDotEnv(ocw.Env); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	// Set up template context with workflow and job metadata
	r.logVerbose("Setting up template context")
	r.templateCtx.Workflow = WorkflowMeta{
		Name:        string(ocw.Name),
		Description: string(ocw.Description),
		ID:          string(ocw.ID),
	}
	r.templateCtx.Job = JobMeta{
		Name:        string(job.Name),
		Description: string(job.Description),
		ID:          jobName,
	}

	// Create outputs directory for step outputs
	r.logVerbose("Creating outputs directory: %s", r.outputsDir())
	if err := r.ensureOutputsDir(); err != nil {
		return fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Create a network for this job (enables container-to-container communication)
	r.logVerbose("Creating job network for job: %s", jobName)
	if err := r.createJobNetwork(ctx, jobName); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	r.logVerbose("Network created: %s", r.networkName)

	// Resolve workflow volumes
	if len(ocw.Volumes) > 0 {
		r.logVerbose("Resolving %d workflow volumes", len(ocw.Volumes))
		if err := r.resolveVolumes(ocw.Volumes); err != nil {
			return fmt.Errorf("failed to resolve volumes: %w", err)
		}
	}

	// Store job-level volumes for step resolution
	r.currentJobVolumes = job.Volumes

	// Apply job-level watch config to background steps that don't have explicit watch
	if job.Watch != nil && job.Watch.IsEnabled() {
		r.logVerbose("Applying job-level watch configuration")
		r.applyJobWatchToSteps(job.Parallel, job.Watch)
		r.applyJobWatchToSteps(job.Sequence, job.Watch)
	}

	start := time.Now()
	flowType := job.GetFlowType()
	r.logVerbose("Starting job flow execution (type: %s)", flowType)

	var err error
	switch flowType {
	case "parallel":
		err = r.runParallel(ctx, job.Parallel)
	case "sequence":
		err = r.runSequence(ctx, job.Sequence)
	case "switch":
		err = r.runSwitch(ctx, *job.Switch, job.Case, job.Default)
	case "step":
		err = r.runStep(ctx, job.Step)
	default:
		err = fmt.Errorf("job has no flow control defined")
	}

	// Print exposed services summary
	r.printExposedServices()

	// Print job outputs (if any and no error)
	if err == nil {
		r.printOutputs(job.Outputs)
	}

	duration := time.Since(start)
	r.logVerbose("Job execution completed in %v", duration.Round(time.Millisecond))
	r.Output(r.styles.CompletionBanner(displayName, duration.Round(time.Millisecond).String(), err == nil))

	// If there are background containers running and no errors, wait for interrupt
	if err == nil && r.hasBackgroundContainers() {
		r.waitForInterrupt()
	}

	return err
}

// Helper functions

// splitEnvVar splits an environment variable string like "KEY=value" into parts
func splitEnvVar(s string) []string {
	idx := indexOf(s, '=')
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

// indexOf returns the index of the first occurrence of c in s, or -1 if not found
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// isValidHostname checks if a string is a valid hostname for container networking
// Valid hostnames: lowercase letters, numbers, hyphens; must start with letter; max 63 chars
func isValidHostname(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	// Must start with a lowercase letter
	if !(name[0] >= 'a' && name[0] <= 'z') {
		return false
	}
	// Can only contain lowercase letters, numbers, and hyphens
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	// Cannot end with a hyphen
	if name[len(name)-1] == '-' {
		return false
	}
	return true
}

// parseDuration parses a duration string, returning defaultVal if empty or invalid
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}
