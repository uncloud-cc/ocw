package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// WatchedContainer tracks a container being watched for file changes
type WatchedContainer struct {
	StepID        string
	StepName      string
	Image         string
	ContainerName string
	WatchConfig   *schema.Watch
	RunStep       *schema.RunStep
	BuildStepID   string // ID of the build step if using a built image

	// Volume mounts for translating container paths to host paths
	VolumeMounts []VolumeMount

	// Runtime state
	mu           sync.Mutex
	reloading    bool
	needsReload  bool
	cancelReload context.CancelFunc
	PortMappings []PortMapping // Track allocated ports to reuse them on reload
}

// Reloader manages file watchers and container reloading
type Reloader struct {
	runner     *Runner
	containers map[string]*WatchedContainer // keyed by step ID
	watchers   map[string]*FileWatcher      // keyed by step ID
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewReloader creates a new reloader
func NewReloader(runner *Runner) *Reloader {
	ctx, cancel := context.WithCancel(context.Background())
	return &Reloader{
		runner:     runner,
		containers: make(map[string]*WatchedContainer),
		watchers:   make(map[string]*FileWatcher),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterContainer registers a container for watching
func (r *Reloader) RegisterContainer(wc *WatchedContainer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.containers[wc.StepID] = wc

	// Create a copy of the watch config to avoid modifying the original
	watchConfig := *wc.WatchConfig

	// If a build step is referenced, automatically add its Dockerfile to the watch patterns
	if wc.BuildStepID != "" {
		dockerfile := r.getDockerfileForBuildStep(wc.BuildStepID)
		if dockerfile != "" {
			// Add Dockerfile to the watch patterns
			files := watchConfig.GetFiles()
			if len(files) > 0 {
				// If specific files are already being watched, add the Dockerfile
				watchConfig.Config = &schema.WatchConfig{
					Files:           append(files, dockerfile),
					Ignore:          watchConfig.GetIgnorePatterns(),
					UseGitIgnore:    boolPtr(watchConfig.ShouldUseGitIgnore()),
					UseDockerIgnore: boolPtr(watchConfig.ShouldUseDockerIgnore()),
					Mode:            watchConfig.GetMode(),
				}
			} else {
				// If watching the entire directory, still explicitly add Dockerfile
				watchConfig.Config = &schema.WatchConfig{
					Files:           []string{dockerfile},
					Ignore:          watchConfig.GetIgnorePatterns(),
					UseGitIgnore:    boolPtr(watchConfig.ShouldUseGitIgnore()),
					UseDockerIgnore: boolPtr(watchConfig.ShouldUseDockerIgnore()),
					Mode:            watchConfig.GetMode(),
				}
			}
		}
	}

	// Always watch the workflow file and .env files (like Dockerfile)
	// These files affect container configuration and should trigger reloads
	files := watchConfig.GetFiles()

	// Add workflow file if available
	if r.runner.WorkflowFile != "" {
		files = append(files, r.runner.WorkflowFile)
	}

	// Add .env file (either custom or default)
	if r.runner.EnvFile != "" {
		files = append(files, r.runner.EnvFile)
	} else {
		// Default .env file
		files = append(files, ".env")
	}

	// Translate container paths to host paths using volume mounts
	for i, pattern := range files {
		files[i] = r.translateContainerPathToHost(pattern, wc.VolumeMounts)
	}

	// Update watch config with the new files
	if len(files) > 0 {
		watchConfig.Config = &schema.WatchConfig{
			Files:           files,
			Ignore:          watchConfig.GetIgnorePatterns(),
			UseGitIgnore:    boolPtr(watchConfig.ShouldUseGitIgnore()),
			UseDockerIgnore: boolPtr(watchConfig.ShouldUseDockerIgnore()),
			Mode:            watchConfig.GetMode(),
		}
	}

	// Create file watcher
	watcher, err := NewFileWatcher(&watchConfig, r.runner.WorkflowDir, func(changedFile string) {
		r.handleChange(wc.StepID, changedFile)
	})
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	r.watchers[wc.StepID] = watcher
	watcher.Start()

	return nil
}

// getDockerfileForBuildStep retrieves the Dockerfile path for a build step
func (r *Reloader) getDockerfileForBuildStep(buildStepID string) string {
	config, ok := r.runner.builtImageConfigs[buildStepID]
	if !ok {
		return ""
	}

	// Default Dockerfile path is "Dockerfile" in the context
	dockerfile := config.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	return dockerfile
}

// boolPtr is a helper to create a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// translateContainerPathToHost converts a container path pattern to a host path pattern
// using the volume mounts. If the path doesn't match any volume mount prefix, it returns the original path.
func (r *Reloader) translateContainerPathToHost(containerPath string, mounts []VolumeMount) string {
	if len(mounts) == 0 {
		return containerPath
	}

	// Sort mounts by container path length (longest first) to get most specific match
	sortedMounts := make([]VolumeMount, len(mounts))
	copy(sortedMounts, mounts)
	for i := range sortedMounts {
		for j := i + 1; j < len(sortedMounts); j++ {
			if len(sortedMounts[i].ContainerPath) < len(sortedMounts[j].ContainerPath) {
				sortedMounts[i], sortedMounts[j] = sortedMounts[j], sortedMounts[i]
			}
		}
	}

	// Check if this path matches any volume mount
	for _, mount := range sortedMounts {
		if strings.HasPrefix(containerPath, mount.ContainerPath) {
			// Replace the container path prefix with the host path
			suffix := strings.TrimPrefix(containerPath, mount.ContainerPath)
			return mount.HostPath + suffix
		}
	}

	return containerPath
}

// sanitizeNameForHostname converts a name to a valid hostname (lowercase, hyphens instead of spaces)
func sanitizeNameForHostname(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' {
			result = append(result, c)
		} else if c >= 'A' && c <= 'Z' {
			// Convert uppercase to lowercase
			result = append(result, c+32)
		} else if (c >= '0' && c <= '9') || c == '-' || c == '_' {
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

// handleChange handles a file change event for a container
func (r *Reloader) handleChange(stepID string, changedFile string) {
	r.mu.RLock()
	wc, exists := r.containers[stepID]
	r.mu.RUnlock()

	if !exists {
		return
	}

	wc.mu.Lock()

	// If already reloading, mark for re-trigger after current reload completes
	if wc.reloading {
		wc.needsReload = true
		// Cancel current reload to speed up re-trigger
		if wc.cancelReload != nil {
			wc.cancelReload()
		}
		wc.mu.Unlock()
		msg := fmt.Sprintf("[watch] %s: changes detected (queued for reload)", wc.StepName)
		if changedFile != "" {
			msg = fmt.Sprintf("[watch] %s: file changed: %s (queued for reload)", wc.StepName, changedFile)
		}
		r.runner.Output("  %s\n", r.runner.styles.Info(msg))
		return
	}

	wc.reloading = true
	wc.mu.Unlock()

	msg := fmt.Sprintf("[watch] %s: files changed, reloading...", wc.StepName)
	if changedFile != "" {
		msg = fmt.Sprintf("[watch] %s: file changed: %s, reloading...", wc.StepName, changedFile)
	}
	r.runner.Output("\n  %s\n", r.runner.styles.Info(msg))

	go r.reload(wc)
}

// reload performs the container reload
func (r *Reloader) reload(wc *WatchedContainer) {
	startTime := time.Now()

	defer func() {
		wc.mu.Lock()
		wc.reloading = false
		needsReReload := wc.needsReload
		wc.needsReload = false
		wc.mu.Unlock()

		// If changes occurred during reload, trigger again
		if needsReReload {
			r.handleChange(wc.StepID, "") // Empty string indicates re-trigger after reload
		}
	}()

	// Create a cancellable context for this reload
	ctx, cancel := context.WithCancel(r.ctx)
	wc.mu.Lock()
	wc.cancelReload = cancel
	wc.mu.Unlock()
	defer cancel()

	mode := wc.WatchConfig.GetMode()

	// Stop existing container first
	stopCtx, stopCancel := context.WithTimeout(ctx, 10*time.Second)
	defer stopCancel()

	if err := r.runner.podman.StopContainer(stopCtx, wc.ContainerName); err != nil {
		// Container might already be stopped, continue
	}
	if err := r.runner.podman.RemoveContainer(stopCtx, wc.ContainerName); err != nil {
		// Container might not exist, continue
	}

	// Check if we need to rebuild
	if mode == schema.WatchModeRebuildReload && wc.BuildStepID != "" {
		if err := r.rebuildAndReload(ctx, wc); err != nil {
			r.runner.Output("  %s\n", r.runner.styles.Error(fmt.Sprintf("[watch] %s: rebuild failed: %v", wc.StepName, err)))
			return
		}
		duration := time.Since(startTime)
		r.runner.Output("  %s\n", r.runner.styles.Success(fmt.Sprintf("[watch] %s: rebuilt and reloaded in %v", wc.StepName, duration.Round(time.Millisecond))))
	} else {
		if err := r.justReload(ctx, wc); err != nil {
			r.runner.Output("  %s\n", r.runner.styles.Error(fmt.Sprintf("[watch] %s: reload failed: %v", wc.StepName, err)))
			return
		}
		duration := time.Since(startTime)
		r.runner.Output("  %s\n", r.runner.styles.Success(fmt.Sprintf("[watch] %s: reloaded in %v", wc.StepName, duration.Round(time.Millisecond))))
	}
}

// rebuildAndReload rebuilds the image then reloads the container
func (r *Reloader) rebuildAndReload(ctx context.Context, wc *WatchedContainer) error {
	// Get the stored build step config
	r.runner.builtImagesMu.RLock()
	buildConfig, exists := r.runner.builtImageConfigs[wc.BuildStepID]
	r.runner.builtImagesMu.RUnlock()

	if !exists {
		// Fall back to just reloading
		return r.justReload(ctx, wc)
	}

	r.runner.Output("  %s\n", r.runner.styles.Dim(fmt.Sprintf("[watch] %s: rebuilding image...", wc.StepName)))

	// Rebuild the image
	buildOpts := BuildImageOptions{
		ImageName:   buildConfig.Image,
		Context:     buildConfig.Context,
		Dockerfile:  buildConfig.Dockerfile,
		BuildArgs:   buildConfig.BuildArgs,
		Target:      buildConfig.Target,
		Tags:        buildConfig.Tags,
		WorkflowDir: r.runner.WorkflowDir,
	}

	builtImage, err := r.runner.podman.BuildImage(ctx, buildOpts)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Update tracked image
	r.runner.registerBuiltImage(wc.BuildStepID, builtImage)
	r.runner.templateCtx.SetStepOutput(wc.BuildStepID, "image", builtImage)

	// Update container's image reference
	wc.Image = builtImage

	// Now reload
	return r.justReload(ctx, wc)
}

// justReload restarts the container without rebuilding
func (r *Reloader) justReload(ctx context.Context, wc *WatchedContainer) error {
	// Reload workflow env vars in case workflow file or .env file changed
	if err := r.runner.reloadWorkflowEnv(); err != nil {
		// Log error but continue with old env vars
		r.runner.Output("  %s\n", r.runner.styles.Error(fmt.Sprintf("[watch] %s: failed to reload env: %v", wc.StepName, err)))
	}

	// Build environment
	// Start with workflow-level env vars, then merge step-level (step overrides workflow)
	env := make(map[string]string)
	for k, v := range r.runner.templateCtx.Env {
		env[k] = v
	}
	if wc.RunStep.RunEnv != nil {
		if wc.RunStep.RunEnv.Map != nil {
			for k, v := range wc.RunStep.RunEnv.Map {
				interpolated, _ := r.runner.templateCtx.Interpolate(v)
				env[k] = interpolated
			}
		}
	}

	// Set OUTPUTS env var if step has an ID
	stepID := string(wc.RunStep.ID)
	if stepID != "" {
		env["OUTPUTS"] = fmt.Sprintf("/workflow/.ocw-outputs/%s", stepID)
	}

	// Determine working directory
	workDir := wc.RunStep.Workdir
	if workDir == "" {
		workDir = "/workflow"
	}

	// Interpolate command
	cmd := wc.RunStep.Cmd
	if cmd != "" {
		cmd, _ = r.runner.templateCtx.Interpolate(cmd)
	}

	// Interpolate entrypoint
	entrypoint := wc.RunStep.Entrypoint
	if entrypoint != "" {
		entrypoint, _ = r.runner.templateCtx.Interpolate(entrypoint)
	}

	// Interpolate args
	args, _ := r.runner.templateCtx.InterpolateSlice(wc.RunStep.Args)

	// Determine hostname - extract from container name if possible
	hostname := wc.StepID
	if hostname == "" {
		// Extract hostname from container name (format: ocw-<runid>-<hostname>)
		// For now, use the step name sanitized
		hostname = sanitizeNameForHostname(wc.StepName)
	}

	opts := RunContainerOptions{
		Name:         wc.ContainerName,
		Hostname:     hostname,
		Network:      r.runner.networkName,
		Image:        wc.Image,
		Cmd:          cmd,
		Args:         args,
		Entrypoint:   entrypoint,
		Env:          env,
		WorkDir:      workDir,
		WorkflowDir:  r.runner.WorkflowDir,
		VolumeMounts: wc.VolumeMounts, // Include volume mounts from watched container
		TTY:          wc.RunStep.TTY,
		Remove:       false,
		Background:   true,
		Force:        true,
	}

	// Process port mappings - reuse existing ports if available
	if wc.RunStep.Expose != nil && len(wc.PortMappings) > 0 {
		// Reuse the port mappings from the original container
		opts.PortMappings = wc.PortMappings
	} else if wc.RunStep.Expose != nil {
		// Fallback: find available ports if not cached (shouldn't happen in normal operation)
		for _, ep := range wc.RunStep.Expose.Ports {
			actualHostPort, err := FindAvailablePort(ep.HostPort)
			if err != nil {
				actualHostPort = ep.HostPort
			}
			opts.PortMappings = append(opts.PortMappings, PortMapping{
				ContainerPort: ep.ContainerPort,
				HostPort:      actualHostPort,
				Protocol:      ep.Protocol,
			})
		}
	}

	// Process health check
	if wc.RunStep.HealthCheck != nil {
		healthCmd, _ := r.runner.templateCtx.Interpolate(wc.RunStep.HealthCheck.Cmd)
		opts.HealthCheck = &HealthCheckConfig{
			Cmd:         healthCmd,
			Interval:    parseDuration(wc.RunStep.HealthCheck.Interval, 2*time.Second),
			Timeout:     parseDuration(wc.RunStep.HealthCheck.Timeout, 5*time.Second),
			Retries:     wc.RunStep.HealthCheck.Retries,
			StartPeriod: parseDuration(wc.RunStep.HealthCheck.StartPeriod, 0),
		}
		if opts.HealthCheck.Retries == 0 {
			opts.HealthCheck.Retries = 10
		}
	}

	return r.runner.podman.RunContainer(ctx, opts)
}

// Stop stops all watchers and cancels pending reloads
func (r *Reloader) Stop() {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, watcher := range r.watchers {
		watcher.Stop()
	}
}
