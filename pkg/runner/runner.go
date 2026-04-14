package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

// ExposedService tracks a service that has been exposed to the host
type ExposedService struct {
	StepID        string // ID of the step (used as identifier)
	StepName      string // Human-readable name of the step
	ContainerPort int    // Port inside the container
	HostPort      int    // Port on the host (may differ if preferred port was unavailable)
	RequestedPort int    // Originally requested host port
	Protocol      string // Protocol (http, https, tcp, udp)
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

// ResolvedVolume contains the resolved paths for a workflow volume
type ResolvedVolume struct {
	Name      string
	HostPath  string            // Absolute path on host
	Mode      schema.VolumeMode // "ro" or "rw"
	MountPath string            // Default mount path (empty = /volumes/<name>)
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

// cleanupNetwork removes the job network
func (r *Runner) cleanupNetwork() {
	if r.networkName == "" {
		return
	}
	// Network cleanup is silent - only show errors
	if err := r.docker.RemoveNetwork(context.Background(), r.networkName); err != nil {
		r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to remove network: %v", err)))
	}
	r.networkName = ""
}

// registerExposedService adds a service to the exposed services list
func (r *Runner) registerExposedService(svc ExposedService) {
	r.exposedMu.Lock()
	defer r.exposedMu.Unlock()
	r.exposedServices = append(r.exposedServices, svc)
}

// printExposedServices prints a summary of all exposed services
func (r *Runner) printExposedServices() {
	r.exposedMu.Lock()
	services := make([]ExposedService, len(r.exposedServices))
	copy(services, r.exposedServices)
	r.exposedMu.Unlock()

	if len(services) == 0 {
		return
	}

	r.Output("\n")
	r.Output(r.styles.Header("  Exposed Services"))
	r.Output("\n")
	r.Output(r.styles.Divider(40))
	r.Output("\n")

	for _, svc := range services {
		// Format the URL based on protocol
		var url string
		switch svc.Protocol {
		case "http":
			url = fmt.Sprintf("http://localhost:%d", svc.HostPort)
		case "https":
			url = fmt.Sprintf("https://localhost:%d", svc.HostPort)
		default:
			// For tcp, udp, etc. just show host:port
			url = fmt.Sprintf("localhost:%d", svc.HostPort)
		}

		// Show identifier (prefer ID, fall back to name)
		identifier := svc.StepID
		if identifier == "" {
			identifier = svc.StepName
		}

		// Show if port was reassigned
		if svc.HostPort != svc.RequestedPort {
			r.Output(r.styles.ServiceURL(identifier, url, fmt.Sprintf("%s, requested: %d", svc.Protocol, svc.RequestedPort)))
		} else {
			r.Output(r.styles.ServiceURL(identifier, url, svc.Protocol))
		}
	}
}

// hasBackgroundContainers returns true if there are background containers running
func (r *Runner) hasBackgroundContainers() bool {
	r.backgroundMu.Lock()
	defer r.backgroundMu.Unlock()
	return len(r.backgroundContainers) > 0
}

// waitForInterrupt waits for SIGINT or SIGTERM, keeping background containers running
func (r *Runner) waitForInterrupt() {
	r.Output("\n%s\n", r.styles.Info("Background services running. Press Ctrl+C to stop..."))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	r.Output("\n%s\n", r.styles.Dim("Shutting down..."))
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

// cleanupBackgroundContainers stops and removes all background containers
func (r *Runner) cleanupBackgroundContainers() {
	// Stop reloader first (stops file watchers and pending reloads)
	if r.reloader != nil {
		r.reloader.Stop()
	}

	r.backgroundMu.Lock()
	containers := make([]string, len(r.backgroundContainers))
	copy(containers, r.backgroundContainers)
	r.backgroundContainers = r.backgroundContainers[:0]
	r.backgroundMu.Unlock()

	if len(containers) == 0 {
		// Still clean up network even if no containers
		r.cleanupNetwork()
		return
	}

	r.Output("\n%s\n", r.styles.Dim(fmt.Sprintf("Cleaning up %d background container(s)...", len(containers))))
	ctx := context.Background()
	for _, name := range containers {
		if err := r.docker.StopContainer(ctx, name); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to stop %s: %v", name, err)))
		}
		if err := r.docker.RemoveContainer(ctx, name); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to remove %s: %v", name, err)))
		}
	}

	// Clean up the network after all containers are removed
	r.cleanupNetwork()
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

// applyJobWatchToSteps applies job-level watch config to steps without explicit watch
func (r *Runner) applyJobWatchToSteps(steps []schema.Step, jobWatch *schema.Watch) {
	for i := range steps {
		step := &steps[i]
		if step.RunStep != nil && step.RunStep.Background && step.RunStep.Watch == nil {
			step.RunStep.Watch = jobWatch
		}
		// Recurse into nested steps
		if step.ParallelStep != nil {
			r.applyJobWatchToSteps(step.ParallelStep.Parallel, jobWatch)
		}
		if step.SequenceStep != nil {
			r.applyJobWatchToSteps(step.SequenceStep.Sequence, jobWatch)
		}
	}
}

// runParallel executes steps in parallel
func (r *Runner) runParallel(ctx context.Context, steps []schema.Step) error {
	r.Output(r.styles.SectionHeader(fmt.Sprintf("Running %d steps in parallel", len(steps))))

	var wg sync.WaitGroup
	errCh := make(chan error, len(steps))

	for i := range steps {
		wg.Add(1)
		go func(step *schema.Step) {
			defer wg.Done()
			if err := r.runStep(ctx, step); err != nil {
				errCh <- err
			}
		}(&steps[i])
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("parallel execution had %d errors: %v", len(errs), errs)
	}

	return nil
}

// runSequence executes steps in sequence
func (r *Runner) runSequence(ctx context.Context, steps []schema.Step) error {
	r.Output(r.styles.SectionHeader(fmt.Sprintf("Running %d steps in sequence", len(steps))))

	for i := range steps {
		if err := r.runStep(ctx, &steps[i]); err != nil {
			return fmt.Errorf("step %d failed: %w", i, err)
		}
	}

	return nil
}

// runSwitch executes steps based on switch expression
func (r *Runner) runSwitch(ctx context.Context, switchExpr string, cases map[string]schema.StepOrSteps, defaultCase *schema.StepOrSteps) error {
	// Interpolate the switch expression
	interpolatedExpr, err := r.templateCtx.Interpolate(switchExpr)
	if err != nil {
		return fmt.Errorf("failed to interpolate switch expression: %w", err)
	}

	r.Output(r.styles.SectionHeader(fmt.Sprintf("Switch on: %s", interpolatedExpr)))

	// Match against case values
	caseSteps, ok := cases[interpolatedExpr]
	if !ok {
		if defaultCase != nil {
			r.Output("  %s\n", r.styles.Dim("No matching case, using default"))
			return r.runStepOrSteps(ctx, defaultCase)
		}
		r.Output("  %s\n", r.styles.Dim("No matching case and no default, skipping"))
		return nil
	}

	r.Output("  %s %s\n", r.styles.Dim("Matched case:"), r.styles.Value(interpolatedExpr))
	return r.runStepOrSteps(ctx, &caseSteps)
}

// runStepOrSteps executes a StepOrSteps (single step or array)
func (r *Runner) runStepOrSteps(ctx context.Context, sos *schema.StepOrSteps) error {
	if sos.Single != nil {
		return r.runStep(ctx, sos.Single)
	}
	for i := range sos.Multiple {
		if err := r.runStep(ctx, &sos.Multiple[i]); err != nil {
			return err
		}
	}
	return nil
}

// runStep executes a single step based on its type
func (r *Runner) runStep(ctx context.Context, step *schema.Step) error {
	switch {
	case step.RunStep != nil:
		return r.runRunStep(ctx, step.RunStep)
	case step.BuildStep != nil:
		return r.runBuildStep(ctx, step.BuildStep)
	case step.ParallelStep != nil:
		return r.runParallelStep(ctx, step.ParallelStep)
	case step.SequenceStep != nil:
		return r.runSequenceStep(ctx, step.SequenceStep)
	case step.WorkflowStep != nil:
		return r.runWorkflowStep(ctx, step.WorkflowStep)
	case step.SwitchStep != nil:
		return r.runSwitchStep(ctx, step.SwitchStep)
	default:
		return fmt.Errorf("unknown step type")
	}
}

// runRunStep executes a run step using Podman
func (r *Runner) runRunStep(ctx context.Context, step *schema.RunStep) error {
	name := step.Name
	if name == "" {
		name = "run"
	}

	// If watch mode is enabled, implicitly run as background container
	// (watch mode only works with long-running/background containers)
	if step.Watch != nil && step.Watch.IsEnabled() && !step.Background {
		step.Background = true
	}

	// Extract build step ID from image template if present (for watch mode)
	// Pattern: {{ steps.<id>.image }} or variants with whitespace
	var referencedBuildStepID string
	if step.Image != "" && strings.Contains(step.Image, "steps.") && strings.Contains(step.Image, ".image") {
		// Use a simple parser to extract the build step ID
		// Look for pattern: {{ steps.<id>.image }}
		startIdx := strings.Index(step.Image, "steps.")
		if startIdx != -1 {
			// Find the start of the ID (after "steps.")
			idStart := startIdx + len("steps.")
			// Find the end of the ID (look for ".")
			idEnd := strings.Index(step.Image[idStart:], ".")
			if idEnd != -1 {
				referencedBuildStepID = strings.TrimSpace(step.Image[idStart : idStart+idEnd])
			}
		}
	}

	r.logVerbose("Starting run step: %s", name)
	r.logVerbose("Original image: %s", step.Image)

	// Interpolate template expressions in image name
	image, err := r.templateCtx.Interpolate(step.Image)
	if err != nil {
		return fmt.Errorf("failed to interpolate image: %w", err)
	}
	r.logVerbose("Interpolated image: %s", image)

	// Interpolate command
	cmd := step.Cmd
	if cmd != "" {
		cmd, err = r.templateCtx.Interpolate(cmd)
		if err != nil {
			return fmt.Errorf("failed to interpolate cmd: %w", err)
		}
		r.logVerbose("Interpolated command: %s", cmd)
	}

	// Interpolate entrypoint
	entrypoint := step.Entrypoint
	if entrypoint != "" {
		entrypoint, err = r.templateCtx.Interpolate(entrypoint)
		if err != nil {
			return fmt.Errorf("failed to interpolate entrypoint: %w", err)
		}
	}

	// Interpolate args
	args, err := r.templateCtx.InterpolateSlice(step.Args)
	if err != nil {
		return fmt.Errorf("failed to interpolate args: %w", err)
	}

	// Interpolate workdir
	workdir := step.Workdir
	if workdir != "" {
		workdir, err = r.templateCtx.Interpolate(workdir)
		if err != nil {
			return fmt.Errorf("failed to interpolate workdir: %w", err)
		}
		r.logVerbose("Working directory: %s", workdir)
	}

	// Interpolate platform
	platform := step.Platform
	if platform != "" {
		platform, err = r.templateCtx.Interpolate(platform)
		if err != nil {
			return fmt.Errorf("failed to interpolate platform: %w", err)
		}
	}

	// Interpolate memory
	memory := step.Memory
	if memory != "" {
		memory, err = r.templateCtx.Interpolate(memory)
		if err != nil {
			return fmt.Errorf("failed to interpolate memory: %w", err)
		}
	}

	// Print styled step header
	extra := map[string]string{"Image": image}
	if step.Background {
		extra["Mode"] = "background"
	}
	r.Output(r.styles.StepBox(name, "run", extra))

	// Pull the image first
	r.logVerbose("Pulling/checking image: %s", image)
	if err := r.docker.PullImage(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	r.logVerbose("Image ready: %s", image)

	// Build environment variables map and interpolate values
	// Start with workflow-level env vars, then merge step-level (step overrides workflow)
	env := make(map[string]string)
	for k, v := range r.templateCtx.Env {
		env[k] = v
	}
	if step.RunEnv != nil {
		if step.RunEnv.Map != nil {
			for k, v := range step.RunEnv.Map {
				interpolatedValue, err := r.templateCtx.Interpolate(v)
				if err != nil {
					return fmt.Errorf("failed to interpolate env %s: %w", k, err)
				}
				env[k] = interpolatedValue
			}
		} else if step.RunEnv.Slice != nil {
			for _, e := range step.RunEnv.Slice {
				parts := splitEnvVar(e)
				if len(parts) == 2 {
					interpolatedValue, err := r.templateCtx.Interpolate(parts[1])
					if err != nil {
						return fmt.Errorf("failed to interpolate env %s: %w", parts[0], err)
					}
					env[parts[0]] = interpolatedValue
				}
			}
		}
	}

	// Determine working directory (already interpolated above as workdir)
	workDir := workdir
	if workDir == "" {
		workDir = "/workflow"
	}

	// Determine container name and hostname for networking
	// Rules:
	// - If ID is provided: use ID as hostname (enables DNS resolution)
	// - If no ID but name is provided for background containers: validate name as valid ID
	// - Hostname allows other containers to reach this one by name in the network
	containerName := ""
	hostname := ""

	if step.Background {
		if step.ID != "" {
			// ID provided - use it as hostname for DNS resolution
			containerName = fmt.Sprintf("ocw-%s-%s", r.runID, step.ID)
			hostname = string(step.ID)
			r.Output("  %s %s\n", r.styles.Label("Hostname:"), r.styles.Value(hostname))
		} else if name != "" && name != "run" {
			// No ID, but has a name - try to use it as hostname if valid
			if isValidHostname(name) {
				containerName = fmt.Sprintf("ocw-%s-%s", r.runID, SanitizeName(name))
				hostname = name
				r.Output("  %s %s\n", r.styles.Label("Hostname:"), r.styles.Value(hostname))
			} else {
				// Name is not a valid hostname - only allow this for watch mode
				if step.Watch != nil && step.Watch.IsEnabled() {
					// For watch mode, sanitize the name to make it a valid hostname
					sanitized := SanitizeName(name)
					containerName = fmt.Sprintf("ocw-%s-%s", r.runID, sanitized)
					hostname = sanitized
					r.Output("  %s %s\n", r.styles.Label("Hostname:"), r.styles.Value(hostname))
				} else {
					return fmt.Errorf("background container needs a valid 'id' for networking. "+
						"Name %q is not a valid hostname (use lowercase letters, numbers, and hyphens only). "+
						"Add 'id: <valid-hostname>' to enable container-to-container communication", name)
				}
			}
		} else {
			// No ID and no valid name - generate unique name but warn about networking
			containerName = fmt.Sprintf("ocw-%s-%d", r.runID, time.Now().UnixNano())
			r.Output("  %s\n", r.styles.Warning("Warning: background container has no 'id' - other containers cannot reach it by hostname"))
		}
	}

	// Convert health check config
	var healthCheck *HealthCheckConfig
	if step.HealthCheck != nil {
		// Interpolate health check command
		healthCheckCmd, err := r.templateCtx.Interpolate(step.HealthCheck.Cmd)
		if err != nil {
			return fmt.Errorf("failed to interpolate healthCheck.cmd: %w", err)
		}
		healthCheck = &HealthCheckConfig{
			Cmd:         healthCheckCmd,
			Interval:    parseDuration(step.HealthCheck.Interval, 2*time.Second),
			Timeout:     parseDuration(step.HealthCheck.Timeout, 5*time.Second),
			Retries:     step.HealthCheck.Retries,
			StartPeriod: parseDuration(step.HealthCheck.StartPeriod, 0),
		}
		if healthCheck.Retries == 0 {
			healthCheck.Retries = 10
		}
	}

	// Process port mappings for expose
	var portMappings []PortMapping
	var exposedPorts []ExposedService // Track for summary
	if step.Expose != nil {
		for _, ep := range step.Expose.Ports {
			// Find available host port (preferred port may be in use)
			actualHostPort, err := FindAvailablePort(ep.HostPort)
			if err != nil {
				return fmt.Errorf("failed to find available port for %d: %w", ep.HostPort, err)
			}

			portMappings = append(portMappings, PortMapping{
				ContainerPort: ep.ContainerPort,
				HostPort:      actualHostPort,
				Protocol:      ep.Protocol,
			})

			// Track for summary
			exposedPorts = append(exposedPorts, ExposedService{
				StepID:        string(step.ID),
				StepName:      string(step.Name),
				ContainerPort: ep.ContainerPort,
				HostPort:      actualHostPort,
				RequestedPort: ep.HostPort,
				Protocol:      ep.Protocol,
			})

			if actualHostPort != ep.HostPort {
				r.Output("  %s %s %s\n", r.styles.Label("Expose:"), r.styles.Value(fmt.Sprintf("%d -> localhost:%d", ep.ContainerPort, actualHostPort)), r.styles.Warning(fmt.Sprintf("(requested %d in use)", ep.HostPort)))
			} else {
				r.Output("  %s %s\n", r.styles.Label("Expose:"), r.styles.Value(fmt.Sprintf("%d -> localhost:%d", ep.ContainerPort, actualHostPort)))
			}
		}
	}

	// Set up OUTPUTS env var for step outputs (if step has an ID)
	stepID := string(step.ID)
	if stepID != "" {
		// Path inside container: /workflow/.ocw-outputs/<step-id>
		env["OUTPUTS"] = fmt.Sprintf("/workflow/.ocw-outputs/%s", stepID)
	}

	// Resolve volume mounts for this step
	volumeMounts, err := r.resolveStepVolumes(step.Volumes, r.currentJobVolumes)
	if err != nil {
		return fmt.Errorf("failed to resolve volumes: %w", err)
	}

	// Run the container
	r.logVerbose("Running container with name: %s, hostname: %s", containerName, hostname)
	r.logVerbose("Network: %s", r.networkName)
	r.logVerbose("Volume mounts: %d", len(volumeMounts))
	r.logVerbose("Environment variables: %d", len(env))
	opts := RunContainerOptions{
		Name:         containerName,
		Hostname:     hostname,
		Network:      r.networkName,
		Image:        image,
		Cmd:          cmd,
		Args:         args,
		Entrypoint:   entrypoint,
		Env:          env,
		WorkDir:      workDir,
		WorkflowDir:  r.WorkflowDir,
		VolumeMounts: volumeMounts,
		TTY:          step.TTY,
		Remove:       !step.Background,
		Background:   step.Background,
		HealthCheck:  healthCheck,
		PortMappings: portMappings,
		Force:        r.force,
	}

	r.logVerbose("Executing docker run command...")
	if err := r.docker.RunContainer(ctx, opts); err != nil {
		return fmt.Errorf("container execution failed: %w", err)
	}
	r.logVerbose("Container execution completed")

	// Track background containers for cleanup
	if step.Background && containerName != "" {
		r.registerBackgroundContainer(containerName)
	}

	// Set up watch mode if enabled
	if step.Background && step.Watch != nil && step.Watch.IsEnabled() {
		r.initReloader()

		wc := &WatchedContainer{
			StepID:        string(step.ID),
			StepName:      string(step.Name),
			Image:         image,
			ContainerName: containerName,
			WatchConfig:   step.Watch,
			RunStep:       step,
			BuildStepID:   referencedBuildStepID,
			PortMappings:  portMappings,
			VolumeMounts:  volumeMounts, // Pass volume mounts for path translation
		}

		if err := r.reloader.RegisterContainer(wc); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to set up watch mode: %v", err)))
		} else {
			r.Output("  %s %s\n", r.styles.Label("Watch:"), r.styles.Value("enabled"))
		}
	}

	// Register exposed services for summary
	for _, es := range exposedPorts {
		r.registerExposedService(es)
	}

	// Parse step outputs (if step has an ID and is not a background container)
	if stepID != "" && !step.Background {
		if err := r.parseStepOutputs(stepID); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to parse step outputs: %v", err)))
		}
	}

	r.Output(r.styles.StepComplete(name, true))
	return nil
}

// runBuildStep executes a build step using Podman
func (r *Runner) runBuildStep(ctx context.Context, step *schema.BuildStep) error {
	name := step.Name
	if name == "" {
		name = "build"
	}

	// Interpolate image name (in case it uses templates)
	imageName, err := r.templateCtx.Interpolate(step.Build.Image)
	if err != nil {
		return fmt.Errorf("failed to interpolate image name: %w", err)
	}

	// Interpolate context path
	context := step.Build.Context
	if context != "" {
		context, err = r.templateCtx.Interpolate(context)
		if err != nil {
			return fmt.Errorf("failed to interpolate context: %w", err)
		}
	}

	// Interpolate dockerfile path
	dockerfile := step.Build.Dockerfile
	if dockerfile != "" {
		dockerfile, err = r.templateCtx.Interpolate(dockerfile)
		if err != nil {
			return fmt.Errorf("failed to interpolate dockerfile: %w", err)
		}
	}

	// Interpolate target
	target := step.Build.Target
	if target != "" {
		target, err = r.templateCtx.Interpolate(target)
		if err != nil {
			return fmt.Errorf("failed to interpolate target: %w", err)
		}
	}

	// Print styled step header
	r.Output(r.styles.StepBox(name, "build", map[string]string{"Image": imageName}))

	r.logVerbose("Starting build step: %s", name)
	r.logVerbose("Building image: %s", imageName)
	r.logVerbose("Context: %s", context)
	r.logVerbose("Dockerfile: %s", dockerfile)

	// Interpolate build args
	buildArgs := make(map[string]string)
	for k, v := range step.Build.BuildArgs {
		interpolatedValue, err := r.templateCtx.Interpolate(v)
		if err != nil {
			return fmt.Errorf("failed to interpolate build arg %s: %w", k, err)
		}
		buildArgs[k] = interpolatedValue
	}
	r.logVerbose("Build args: %d", len(buildArgs))

	// Interpolate tags
	tags, err := r.templateCtx.InterpolateSlice(step.Build.Tags)
	if err != nil {
		return fmt.Errorf("failed to interpolate tags: %w", err)
	}

	// Resolve volume mounts for this step
	volumeMounts, err := r.resolveStepVolumes(step.Volumes, r.currentJobVolumes)
	if err != nil {
		return fmt.Errorf("failed to resolve volumes: %w", err)
	}
	r.logVerbose("Volume mounts resolved: %d", len(volumeMounts))

	// Build the image
	opts := BuildImageOptions{
		ImageName:    imageName,
		Context:      context,
		Dockerfile:   dockerfile,
		BuildArgs:    buildArgs,
		Target:       target,
		Tags:         tags,
		WorkflowDir:  r.WorkflowDir,
		VolumeMounts: volumeMounts,
	}

	r.logVerbose("Starting docker build...")
	builtImage, err := r.docker.BuildImage(ctx, opts)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	r.logVerbose("Build completed successfully: %s", builtImage)

	// Register the built image if step has an ID
	// This makes it available as ${{ steps.<id>.image }}
	if step.ID != "" {
		r.registerBuiltImage(string(step.ID), builtImage)
		// Also register in template context for ${{ steps.<id>.image }}
		r.templateCtx.SetStepOutput(string(step.ID), "image", builtImage)
		r.Output("  %s %s\n", r.styles.Dim("Registered:"), r.styles.Value(fmt.Sprintf("steps.%s.image = %s", step.ID, builtImage)))

		// Store build config for watch mode rebuilds
		r.builtImagesMu.Lock()
		r.builtImageConfigs[string(step.ID)] = &step.Build
		r.builtImagesMu.Unlock()
	}

	r.Output(r.styles.StepComplete(name, true))
	return nil
}

// runParallelStep executes a parallel step
func (r *Runner) runParallelStep(ctx context.Context, step *schema.ParallelStep) error {
	name := step.Name
	if name == "" {
		name = "parallel"
	}
	r.Output(r.styles.StepBox(string(name), "parallel", nil))
	return r.runParallel(ctx, step.Parallel)
}

// runSequenceStep executes a sequence step
func (r *Runner) runSequenceStep(ctx context.Context, step *schema.SequenceStep) error {
	name := step.Name
	if name == "" {
		name = "sequence"
	}
	r.Output(r.styles.StepBox(string(name), "sequence", nil))
	return r.runSequence(ctx, step.Sequence)
}

// runWorkflowStep executes a workflow step (mock implementation)
func (r *Runner) runWorkflowStep(ctx context.Context, step *schema.WorkflowStep) error {
	r.Output(r.styles.StepBox("workflow", "workflow", map[string]string{"From": step.Workflow.From}))

	if step.Workflow.Inherit != nil {
		r.Output("  %s %s\n", r.styles.Label("Inherit secrets:"), r.styles.Value(string(step.Workflow.Inherit.Secrets)))
		r.Output("  %s %s\n", r.styles.Label("Inherit env:"), r.styles.Value(string(step.Workflow.Inherit.Env)))
	}

	// TODO: Actually load and run the referenced workflow
	r.Output("  %s\n", r.styles.Warning("Warning: workflow invocation not yet implemented"))
	return nil
}

// runSwitchStep executes a switch step
func (r *Runner) runSwitchStep(ctx context.Context, step *schema.SwitchStep) error {
	name := step.Name
	if name == "" {
		name = "switch"
	}
	r.Output(r.styles.StepBox(string(name), "switch", nil))
	return r.runSwitch(ctx, step.Switch, step.Case, step.Default)
}

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

// sanitizeName creates a safe container name from a step name

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

// resolveVolumes resolves volume paths from the workflow schema
func (r *Runner) resolveVolumes(volumes schema.Volumes) error {
	r.resolvedVolumes = make(map[string]*ResolvedVolume)

	for name, vol := range volumes {
		hostPath := vol.Path

		// Expand ~ to home directory
		if strings.HasPrefix(hostPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("volume %q: failed to get home directory: %w", name, err)
			}
			hostPath = filepath.Join(homeDir, hostPath[2:])
		}

		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(r.WorkflowDir, vol.Path)
		}

		absPath, err := filepath.Abs(hostPath)
		if err != nil {
			return fmt.Errorf("volume %q: %w", name, err)
		}

		// Verify path exists
		if _, err := os.Stat(absPath); err != nil {
			return fmt.Errorf("volume %q path does not exist: %s", name, absPath)
		}

		mode := vol.Mode
		if mode == "" {
			mode = schema.VolumeModeReadOnly
		}

		r.resolvedVolumes[name] = &ResolvedVolume{
			Name:      name,
			HostPath:  absPath,
			Mode:      mode,
			MountPath: vol.MountPath,
		}
	}

	return nil
}

// VolumeMount represents a resolved volume mount
type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// resolveStepVolumes resolves volume references for a step
// Returns a list of mount specifications for docker
func (r *Runner) resolveStepVolumes(stepVolumes, jobVolumes schema.VolumeRefs) ([]VolumeMount, error) {
	var mounts []VolumeMount
	seen := make(map[string]bool)

	// Process step-level volumes (highest priority)
	for _, ref := range stepVolumes {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true

		mount, err := r.resolveVolumeRef(ref)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}

	// Process job-level volumes
	for _, ref := range jobVolumes {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true

		mount, err := r.resolveVolumeRef(ref)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}

	return mounts, nil
}

// resolveVolumeRef resolves a single volume reference
func (r *Runner) resolveVolumeRef(ref schema.VolumeRef) (VolumeMount, error) {
	vol, ok := r.resolvedVolumes[ref.Name]
	if !ok {
		return VolumeMount{}, fmt.Errorf("volume %q not defined", ref.Name)
	}

	// Determine mount path (ref override > volume default > /volumes/<name>)
	mountPath := ref.MountPath
	if mountPath == "" {
		mountPath = vol.MountPath
	}
	if mountPath == "" {
		mountPath = "/volumes/" + ref.Name
	}

	// Determine if mount should be read-only
	// Start with volume's mode
	readOnly := vol.Mode == schema.VolumeModeReadOnly || vol.Mode == ""

	// ref.ReadOnly can only make it MORE restrictive (rw -> ro)
	// It cannot make a ro volume writable
	if ref.ReadOnly != nil {
		if *ref.ReadOnly {
			// Always allowed: making mount read-only
			readOnly = true
		} else if readOnly {
			// ERROR: Cannot make a read-only volume writable
			return VolumeMount{}, fmt.Errorf(
				"volume %q is read-only and cannot be mounted as read-write; "+
					"steps can only make volumes more restrictive, not less",
				ref.Name,
			)
		}
		// If ref.ReadOnly is false and volume is rw, readOnly stays false
	}

	return VolumeMount{
		HostPath:      vol.HostPath,
		ContainerPath: mountPath,
		ReadOnly:      readOnly,
	}, nil
}
