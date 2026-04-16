package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

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
	background := step.Background
	if step.Watch != nil && step.Watch.IsEnabled() && !background {
		background = true
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
	if background {
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

	if background {
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
		Remove:       !background,
		Background:   background,
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
	if background && containerName != "" {
		r.registerBackgroundContainer(containerName)
	}

	// Set up watch mode if enabled
	if background && step.Watch != nil && step.Watch.IsEnabled() {
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
	if stepID != "" && !background {
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

// runPushStep executes a push step (placeholder - not implemented in original)
// Note: This function doesn't exist in the original file, but was mentioned in the plan
// Keeping this comment for reference

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

// runSwitchStep executes a switch step
func (r *Runner) runSwitchStep(ctx context.Context, step *schema.SwitchStep) error {
	name := step.Name
	if name == "" {
		name = "switch"
	}
	r.Output(r.styles.StepBox(string(name), "switch", nil))
	return r.runSwitch(ctx, step.Switch, step.Case, step.Default)
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
