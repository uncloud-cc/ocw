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

	"github.com/opencontainerworkflow/ocw/pkg/schema"

	"github.com/goccy/go-yaml"
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
	// EnvFile is the path to the .env file to load (empty means use default .env)
	EnvFile string
	// InputsFile is the path to the inputs YAML file to load
	InputsFile string
	// Output function for logging (defaults to fmt.Printf)
	Output func(format string, args ...any)
	// Podman client for container operations
	podman *Podman
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
		podman:               NewPodman(output, styles, nil),
		builtImages:          make(map[string]string),
		backgroundContainers: make([]string, 0),
		exposedServices:      make([]ExposedService, 0),
		templateCtx:          NewTemplateContext(),
		styles:               styles,
		secretEnvKeys:        make(map[string]bool),
	}
}

// WithEnvFile sets a custom .env file path
func (r *Runner) WithEnvFile(envFile string) *Runner {
	r.EnvFile = envFile
	return r
}

// WithInputsFile sets a custom inputs YAML file path
func (r *Runner) WithInputsFile(inputsFile string) *Runner {
	r.InputsFile = inputsFile
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
			r.templateCtx.Secrets[key] = envVar.Value
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
			// It's a secret - update both env and secrets context
			r.templateCtx.Env[key] = value
			r.templateCtx.Secrets[key] = value
			// Collect .env secret value for masking
			secretValues = append(secretValues, value)
		} else {
			// Regular env var - only update env context
			r.templateCtx.Env[key] = value
		}
	}

	// Store secret values for masking in outputs
	r.secretValues = secretValues

	// Set secrets on podman client for masking (all secret values)
	// Only mask if showSecrets is false
	if !r.showSecrets {
		r.podman.SetSecrets(secretValues)
	} else {
		r.podman.SetSecrets(nil) // No masking
	}

	if len(dotenv.Vars) > 0 {
		r.Output("  %s %s\n", r.styles.Dim(fmt.Sprintf("Loaded %d variable(s) from", len(dotenv.Vars))), r.styles.Value(envFileName))
	}

	return nil
}

// loadInputs loads inputs from a YAML file and populates the template context
func (r *Runner) loadInputs() error {
	if r.InputsFile == "" {
		// No inputs file specified, nothing to load
		return nil
	}

	inputs, err := LoadInputsFile(r.InputsFile)
	if err != nil {
		return err
	}

	// Load inputs into template context
	for key, value := range inputs {
		r.templateCtx.Inputs[key] = value
	}

	if len(inputs) > 0 {
		r.Output("  %s %s\n", r.styles.Dim(fmt.Sprintf("Loaded %d input(s) from", len(inputs))), r.styles.Value(r.InputsFile))
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
	r.networkName = fmt.Sprintf("ocw-%s-%d", sanitizeName(jobName), time.Now().UnixNano())
	// Network creation is silent - only show errors
	return r.podman.CreateNetwork(ctx, NetworkCreateOptions{
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
	if err := r.podman.RemoveNetwork(context.Background(), r.networkName); err != nil {
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
		if err := r.podman.StopContainer(ctx, name); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to stop %s: %v", name, err)))
		}
		if err := r.podman.RemoveContainer(ctx, name); err != nil {
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
	// Ensure background containers and outputs are cleaned up when done
	defer r.cleanupBackgroundContainers()
	defer r.cleanupOutputsDir()

	// Print styled job header
	r.Output(r.styles.JobBox(string(ocw.Name), "", string(ocw.Description)))
	r.Output("  %s %s\n\n", r.styles.Label("Directory:"), r.styles.Value(r.WorkflowDir))

	// Load .env file if present (passing workflow env as defaults)
	if err := r.loadDotEnv(ocw.Env); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	// Load inputs file if specified
	if err := r.loadInputs(); err != nil {
		return fmt.Errorf("failed to load inputs: %w", err)
	}

	// Set up template context with workflow metadata
	r.templateCtx.Workflow = WorkflowMeta{
		Name:        string(ocw.Name),
		Description: string(ocw.Description),
		ID:          string(ocw.ID),
	}

	// Create outputs directory for step outputs
	if err := r.ensureOutputsDir(); err != nil {
		return fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Create a network for this workflow (enables container-to-container communication)
	workflowName := sanitizeName(ocw.Name)
	if workflowName == "" {
		workflowName = "workflow"
	}
	if err := r.createJobNetwork(ctx, workflowName); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	start := time.Now()

	var err error
	switch ocw.GetFlowType() {
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
	r.Output(r.styles.CompletionBanner(string(ocw.Name), duration.Round(time.Millisecond).String(), err == nil))

	// If there are background containers running and no errors, wait for interrupt
	if err == nil && r.hasBackgroundContainers() {
		r.waitForInterrupt()
	}

	return err
}

// RunJob executes a specific job from an OCW workflow
func (r *Runner) RunJob(ctx context.Context, ocw *schema.OCW, jobName string) error {
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
	if err := r.loadDotEnv(ocw.Env); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	// Load inputs file if specified
	if err := r.loadInputs(); err != nil {
		return fmt.Errorf("failed to load inputs: %w", err)
	}

	// Set up template context with workflow and job metadata
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
	if err := r.ensureOutputsDir(); err != nil {
		return fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Create a network for this job (enables container-to-container communication)
	if err := r.createJobNetwork(ctx, jobName); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	start := time.Now()

	var err error
	switch job.GetFlowType() {
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
	r.Output(r.styles.CompletionBanner(displayName, duration.Round(time.Millisecond).String(), err == nil))

	// If there are background containers running and no errors, wait for interrupt
	if err == nil && r.hasBackgroundContainers() {
		r.waitForInterrupt()
	}

	return err
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

	// Interpolate template expressions in image name
	image, err := r.templateCtx.Interpolate(step.Image)
	if err != nil {
		return fmt.Errorf("failed to interpolate image: %w", err)
	}

	// Interpolate command
	cmd := step.Cmd
	if cmd != "" {
		cmd, err = r.templateCtx.Interpolate(cmd)
		if err != nil {
			return fmt.Errorf("failed to interpolate cmd: %w", err)
		}
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
	if err := r.podman.PullImage(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Build environment variables map and interpolate values
	env := make(map[string]string)
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
			containerName = fmt.Sprintf("ocw-%s", step.ID)
			hostname = string(step.ID)
			r.Output("  %s %s\n", r.styles.Label("Hostname:"), r.styles.Value(hostname))
		} else if name != "" && name != "run" {
			// No ID, but has a name - validate it as a valid hostname
			if isValidHostname(name) {
				containerName = fmt.Sprintf("ocw-%s", sanitizeName(name))
				hostname = name
				r.Output("  %s %s\n", r.styles.Label("Hostname:"), r.styles.Value(hostname))
			} else {
				return fmt.Errorf("background container needs a valid 'id' for networking. "+
					"Name %q is not a valid hostname (use lowercase letters, numbers, and hyphens only). "+
					"Add 'id: <valid-hostname>' to enable container-to-container communication", name)
			}
		} else {
			// No ID and no valid name - generate unique name but warn about networking
			containerName = fmt.Sprintf("ocw-%d", time.Now().UnixNano())
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

	// Run the container
	opts := RunContainerOptions{
		Name:         containerName,
		Hostname:     hostname,
		Network:      r.networkName,
		Image:        image,      // Use interpolated image
		Cmd:          cmd,        // Use interpolated cmd
		Args:         args,       // Use interpolated args
		Entrypoint:   entrypoint, // Use interpolated entrypoint
		Env:          env,        // Already interpolated + OCW_OUTPUT
		WorkDir:      workDir,
		WorkflowDir:  r.WorkflowDir,
		TTY:          step.TTY,
		Remove:       !step.Background,
		Background:   step.Background,
		HealthCheck:  healthCheck,
		PortMappings: portMappings,
		Force:        r.force, // Pass force flag
	}

	if err := r.podman.RunContainer(ctx, opts); err != nil {
		return fmt.Errorf("container execution failed: %w", err)
	}

	// Track background containers for cleanup
	if step.Background && containerName != "" {
		r.registerBackgroundContainer(containerName)
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

	// Interpolate build args
	buildArgs := make(map[string]string)
	for k, v := range step.Build.BuildArgs {
		interpolatedValue, err := r.templateCtx.Interpolate(v)
		if err != nil {
			return fmt.Errorf("failed to interpolate build arg %s: %w", k, err)
		}
		buildArgs[k] = interpolatedValue
	}

	// Interpolate tags
	tags, err := r.templateCtx.InterpolateSlice(step.Build.Tags)
	if err != nil {
		return fmt.Errorf("failed to interpolate tags: %w", err)
	}

	// Build the image
	opts := BuildImageOptions{
		ImageName:   imageName,
		Context:     context,
		Dockerfile:  dockerfile,
		BuildArgs:   buildArgs,
		Target:      target,
		Tags:        tags,
		WorkflowDir: r.WorkflowDir,
	}

	builtImage, err := r.podman.BuildImage(ctx, opts)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Register the built image if step has an ID
	// This makes it available as ${{ steps.<id>.image }}
	if step.ID != "" {
		r.registerBuiltImage(string(step.ID), builtImage)
		// Also register in template context for ${{ steps.<id>.image }}
		r.templateCtx.SetStepOutput(string(step.ID), "image", builtImage)
		r.Output("  %s %s\n", r.styles.Dim("Registered:"), r.styles.Value(fmt.Sprintf("steps.%s.image = %s", step.ID, builtImage)))
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

	if len(step.Workflow.Inputs) > 0 {
		inputsYaml, _ := yaml.Marshal(step.Workflow.Inputs)
		r.Output("  %s\n%s", r.styles.Label("Inputs:"), string(inputsYaml))
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
func sanitizeName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}
	if len(result) == 0 {
		return "container"
	}
	return string(result)
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
