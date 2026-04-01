package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prefixWriter wraps a writer and prefixes each line with a styled prefix
// It also masks any secret values from the output
type prefixWriter struct {
	w       io.Writer
	prefix  string
	buffer  bytes.Buffer
	secrets []string // Secret values to mask
}

// newPrefixWriter creates a new prefixWriter
func newPrefixWriter(w io.Writer, prefix string, secrets []string) *prefixWriter {
	return &prefixWriter{w: w, prefix: prefix, secrets: secrets}
}

// maskSecrets replaces secret values with [secret]
func (pw *prefixWriter) maskSecrets(line []byte) []byte {
	result := line
	for _, secret := range pw.secrets {
		if secret != "" {
			result = bytes.ReplaceAll(result, []byte(secret), []byte("[secret]"))
		}
	}
	return result
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	// Add to buffer
	pw.buffer.Write(p)

	// Process complete lines
	for {
		line, err := pw.buffer.ReadBytes('\n')
		if err != nil {
			// No complete line, put it back
			pw.buffer.Write(line)
			break
		}
		// Mask secrets and write prefixed line
		maskedLine := pw.maskSecrets(line)
		fmt.Fprint(pw.w, pw.prefix)
		pw.w.Write(maskedLine)
	}

	return n, nil
}

// Flush writes any remaining buffered content
func (pw *prefixWriter) Flush() {
	if pw.buffer.Len() > 0 {
		fmt.Fprint(pw.w, pw.prefix)
		masked := pw.maskSecrets(pw.buffer.Bytes())
		pw.w.Write(masked)
		fmt.Fprintln(pw.w)
		pw.buffer.Reset()
	}
}

// Docker wraps docker CLI commands
type Docker struct {
	// Output function for logging
	Output func(format string, args ...any)
	// styles provides styled output formatting
	styles *Styles
	// secrets contains sensitive values to mask in output
	secrets []string
	// verbose enables detailed logging of docker operations
	verbose bool
}

// NetworkCreateOptions holds options for creating a network
type NetworkCreateOptions struct {
	Name   string // Network name
	Driver string // Network driver (default: bridge)
}

// CreateNetwork creates a Docker network
func (d *Docker) CreateNetwork(ctx context.Context, opts NetworkCreateOptions) error {
	if d.verbose {
		d.Output("  %s Checking if network exists: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.Name))
	}

	// Check if network already exists
	if d.NetworkExists(ctx, opts.Name) {
		if d.verbose {
			d.Output("  %s Network already exists: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.Name))
		}
		// Network exists, silently continue
		return nil
	}

	driver := opts.Driver
	if driver == "" {
		driver = "bridge"
	}

	args := []string{"network", "create", "--driver", driver, opts.Name}
	if d.verbose {
		d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	// Suppress network creation output - not interesting for users
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr

	if d.verbose {
		d.Output("  %s Starting docker network create...\n", d.styles.Dim("[verbose]"))
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create network %s: %w", opts.Name, err)
	}
	if d.verbose {
		d.Output("  %s Network created successfully\n", d.styles.Dim("[verbose]"))
	}

	return nil
}

// RemoveNetwork removes a Docker network
func (d *Docker) RemoveNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", name)
	return cmd.Run()
}

// NetworkExists checks if a network exists
func (d *Docker) NetworkExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", name)
	return cmd.Run() == nil
}

// PortMapping represents a container port to host port mapping
type PortMapping struct {
	ContainerPort int    // Port inside the container
	HostPort      int    // Port on the host
	Protocol      string // Protocol (http, https, tcp, udp)
}

// IsPortAvailable checks if a port is available on the host
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FindAvailablePort finds an available port, preferring the requested port.
// If the requested port is in use, it finds a random available port.
// Returns the actual port that will be used.
func FindAvailablePort(preferredPort int) (int, error) {
	// Try the preferred port first
	if IsPortAvailable(preferredPort) {
		return preferredPort, nil
	}

	// Preferred port is in use, find a random available port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// NewDocker creates a new Docker wrapper
func NewDocker(output func(format string, args ...any), styles *Styles, secrets []string) *Docker {
	return &Docker{Output: output, styles: styles, secrets: secrets}
}

// SetSecrets updates the secrets to mask in output
func (d *Docker) SetSecrets(secrets []string) {
	d.secrets = secrets
}

// WithVerbose enables or disables verbose logging
func (d *Docker) WithVerbose(verbose bool) *Docker {
	d.verbose = verbose
	return d
}

// PullImage pulls an image if not present locally
func (d *Docker) PullImage(ctx context.Context, imageName string) error {
	if d.verbose {
		d.Output("  %s Checking if image exists: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(imageName))
	}

	// Check if image exists locally
	if d.ImageExists(ctx, imageName) {
		d.Output("  %s %s\n", d.styles.Dim("Image exists:"), d.styles.Value(imageName))
		return nil
	}

	d.Output("  %s %s\n", d.styles.Info("Pulling:"), d.styles.Value(imageName))
	if d.verbose {
		d.Output("  %s Executing: docker pull %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(imageName))
	}

	// Create prefixed writers for pull output
	logPrefix := d.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, d.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, d.secrets)

	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if d.verbose {
		d.Output("  %s Starting docker pull command...\n", d.styles.Dim("[verbose]"))
	}
	if err := cmd.Run(); err != nil {
		stdoutWriter.Flush()
		stderrWriter.Flush()
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	if d.verbose {
		d.Output("  %s Docker pull completed successfully\n", d.styles.Dim("[verbose]"))
	}

	stdoutWriter.Flush()
	stderrWriter.Flush()
	return nil
}

// HealthCheckConfig holds health check configuration
type HealthCheckConfig struct {
	Cmd         string        // Command to run for health check
	Interval    time.Duration // Time between health checks
	Timeout     time.Duration // Timeout for each health check
	Retries     int           // Number of retries before failing
	StartPeriod time.Duration // Grace period before starting health checks
}

// RunContainerOptions holds options for running a container
type RunContainerOptions struct {
	Name             string             // Container name (optional, auto-generated if empty)
	Hostname         string             // Hostname for the container (for DNS resolution in network)
	Network          string             // Network to connect to (empty = default docker network)
	Image            string             // Image to run
	Cmd              string             // Command string (will be passed to shell)
	Args             []string           // Command arguments (if Cmd is empty)
	Entrypoint       string             // Override entrypoint
	Env              map[string]string  // Environment variables
	WorkDir          string             // Working directory inside container
	WorkflowDir      string             // Host path to mount as /workflow
	VolumeMounts     []VolumeMount      // Additional volume mounts (for explicit host access)
	TTY              bool               // Allocate TTY
	Remove           bool               // Remove container after exit (default true for non-background)
	Background       bool               // Run in background (detached)
	HealthCheck      *HealthCheckConfig // Health check for background containers
	PortMappings     []PortMapping      // Ports to expose from container to host
	Force            bool               // Force remove existing container with same name
	DebugImage       string             // Debug sidecar image (empty = no debug sidecar)
	DebugContainer   string             // Debug sidecar container name
	DebugAttach      bool               // Immediately attach to debug container (for CLI debug mode)
	OnContainerStart func(string)       // Callback when container starts (for debug sidecar)
	OnVolumeCreate   func(string)       // Callback when debug volume is created (for filesystem sidecar)
}

// RunContainer runs a container and waits for it to complete
func (d *Docker) RunContainer(ctx context.Context, opts RunContainerOptions) error {
	args := []string{"run"}

	if d.verbose {
		d.Output("  %s Preparing container: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.Name))
	}

	// For background containers, we don't use --rm (we manage cleanup ourselves)
	// For foreground containers, always remove after exit
	if !opts.Background {
		args = append(args, "--rm")
	}

	// Container name - generate one if not provided (needed for background containers)
	containerName := opts.Name
	if containerName == "" && opts.Background {
		containerName = fmt.Sprintf("ocw-%d", time.Now().UnixNano())
	}

	// Handle existing containers
	if containerName != "" && d.ContainerExists(ctx, containerName) {
		if opts.Force {
			// Force flag: remove existing container regardless of state
			if d.verbose {
				d.Output("  %s Force flag set, removing existing container: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(containerName))
			}
			if err := d.RemoveExistingContainer(ctx, containerName); err != nil {
				return fmt.Errorf("failed to remove existing container: %w", err)
			}
		} else if !d.IsContainerRunning(ctx, containerName) {
			// Auto-cleanup: remove stopped containers automatically
			if d.verbose {
				d.Output("  %s Auto-removing stopped container: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(containerName))
			}
			d.Output("  %s\n", d.styles.Dim(fmt.Sprintf("Auto-removing stopped container '%s'...", containerName)))
			if err := d.RemoveContainer(ctx, containerName); err != nil {
				return fmt.Errorf("failed to remove stopped container: %w", err)
			}
		}
		// If container is running and force is not set, let docker fail with a clear error
	}

	if containerName != "" {
		args = append(args, "--name", containerName)
	}

	// Network - connect to specified network
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
		if d.verbose {
			d.Output("  %s Network: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.Network))
		}
	}

	// Hostname - for DNS resolution within the network
	if opts.Hostname != "" {
		args = append(args, "--hostname", opts.Hostname)
		// Also add network alias so other containers can reach this one by hostname
		if opts.Network != "" {
			args = append(args, "--network-alias", opts.Hostname)
		}
	}

	// Detached mode for background containers
	if opts.Background {
		args = append(args, "-d")
		if d.verbose {
			d.Output("  %s Background mode enabled\n", d.styles.Dim("[verbose]"))
		}
	}

	// Port mappings
	for _, pm := range opts.PortMappings {
		args = append(args, "-p", fmt.Sprintf("%d:%d", pm.HostPort, pm.ContainerPort))
		if d.verbose {
			d.Output("  %s Port mapping: %d:%d\n", d.styles.Dim("[verbose]"), pm.HostPort, pm.ContainerPort)
		}
	}

	// Enable shareable IPC namespace when debug sidecar will be attached
	if opts.DebugImage != "" {
		args = append(args, "--ipc=shareable")
		if d.verbose {
			d.Output("  %s IPC: shareable (for debug sidecar)\n", d.styles.Dim("[verbose]"))
		}
	}

	// TTY
	if opts.TTY {
		args = append(args, "-t")
	}

	// Environment variables
	if d.verbose && len(opts.Env) > 0 {
		d.Output("  %s Environment variables: %d\n", d.styles.Dim("[verbose]"), len(opts.Env))
	}
	for key, value := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Working directory
	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}

	// Mount workflow directory as /workflow (read-write)
	if opts.WorkflowDir != "" {
		absPath, err := filepath.Abs(opts.WorkflowDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for workflow dir: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/workflow:rw", absPath))
		if d.verbose {
			d.Output("  %s Mount: %s:/workflow:rw\n", d.styles.Dim("[verbose]"), absPath)
		}
	}

	// Mount explicit volumes
	for _, mount := range opts.VolumeMounts {
		mode := "rw"
		if mount.ReadOnly {
			mode = "ro"
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s:%s",
			mount.HostPath,
			mount.ContainerPath,
			mode))
		if d.verbose {
			d.Output("  %s Volume mount: %s:%s:%s\n", d.styles.Dim("[verbose]"), mount.HostPath, mount.ContainerPath, mode)
		}
	}

	// Entrypoint override
	if opts.Entrypoint != "" {
		args = append(args, "--entrypoint", opts.Entrypoint)
		if d.verbose {
			d.Output("  %s Entrypoint: %s\n", d.styles.Dim("[verbose]"), opts.Entrypoint)
		}
	}

	// Image
	args = append(args, opts.Image)

	// Command - if Cmd is set, run it through shell
	if opts.Cmd != "" {
		args = append(args, "/bin/sh", "-c", opts.Cmd)
		if d.verbose {
			d.Output("  %s Command: /bin/sh -c %s\n", d.styles.Dim("[verbose]"), opts.Cmd)
		}
	} else if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
		if d.verbose {
			d.Output("  %s Args: %v\n", d.styles.Dim("[verbose]"), opts.Args)
		}
	}

	if d.verbose {
		d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
	}

	// Create prefixed writers for container output
	logPrefix := d.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, d.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, d.secrets)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if !opts.Background {
		cmd.Stdin = os.Stdin
	}

	if d.verbose {
		d.Output("  %s Starting docker run command...\n", d.styles.Dim("[verbose]"))
	}
	if err := cmd.Run(); err != nil {
		// Flush any remaining output
		stdoutWriter.Flush()
		stderrWriter.Flush()
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("container exited with code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run container: %w", err)
	}
	if d.verbose {
		d.Output("  %s Container execution completed\n", d.styles.Dim("[verbose]"))
	}

	// Flush any remaining output
	stdoutWriter.Flush()
	stderrWriter.Flush()

	// For background containers with debug mode, spawn sidecar immediately
	// We must do this FAST before the target container exits
	if opts.Background && opts.DebugImage != "" && containerName != "" {
		d.Output("  %s\n", d.styles.Dim(fmt.Sprintf("Creating debug sidecar (%s)...", opts.DebugImage)))

		// Try to create sidecar immediately - don't wait
		if err := d.RunDebugSidecar(ctx, DebugSidecarOptions{
			TargetContainer:  containerName,
			DebugContainer:   opts.DebugContainer,
			DebugImage:       opts.DebugImage,
			Network:          opts.Network,
			OnContainerStart: opts.OnContainerStart,
			OnVolumeCreate:   opts.OnVolumeCreate,
		}); err != nil {
			d.Output("  %s\n", d.styles.Warning(fmt.Sprintf("Warning: failed to create debug sidecar: %v", err)))
		} else {
			d.Output("  %s %s\n", d.styles.Label("Debug container:"), d.styles.Value(opts.DebugContainer))
			// Call the callback to register the debug container
			if opts.OnContainerStart != nil {
				opts.OnContainerStart(opts.DebugContainer)
			}

			// If immediate attach is requested, drop into the debug container
			if opts.DebugAttach {
				d.Output("\n%s\n", d.styles.Info("Attaching to debug container..."))
				d.Output("  %s %s\n", d.styles.Dim("Target processes:"), d.styles.Value("visible via ps, top, etc."))
				d.Output("  %s %s\n", d.styles.Dim("Target filesystem:"), d.styles.Value("/proc/1/root/"))
				d.Output("  %s %s\n", d.styles.Dim("Volume mounts:"), d.styles.Value("/target/<mount-path>"))
				d.Output("  %s\n", d.styles.Dim("Press Ctrl+D or type 'exit' to detach\n"))

				// Run interactive shell in the debug container
				attachCmd := exec.CommandContext(ctx, "docker", "exec", "-it", opts.DebugContainer, "/bin/bash")
				attachCmd.Stdin = os.Stdin
				attachCmd.Stdout = os.Stdout
				attachCmd.Stderr = os.Stderr
				if err := attachCmd.Run(); err != nil {
					// Don't fail the whole workflow if user exits the shell
					if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
						d.Output("  %s\n", d.styles.Dim("Debug shell exited"))
					}
				}
			} else {
				d.Output("  %s %s\n", d.styles.Dim("Attach with:"), d.styles.Value(fmt.Sprintf("docker exec -it %s /bin/bash", opts.DebugContainer)))
				d.Output("  %s %s\n", d.styles.Dim("Target filesystem:"), d.styles.Value("/proc/1/root/"))
				d.Output("  %s %s\n", d.styles.Dim("Volume mounts:"), d.styles.Value("/target/<mount-path>"))
				d.Output("  %s\n", d.styles.Dim("Note: Debug sidecar remains accessible even if main container crashes"))
			}
		}
	}

	// For background containers, wait for health check if configured
	if opts.Background && opts.HealthCheck != nil {
		d.Output("  %s\n", d.styles.Dim("Waiting for health check..."))
		if err := d.waitForHealthy(ctx, containerName, opts.HealthCheck); err != nil {
			// Don't clean up if debug sidecar is running - let user inspect
			if opts.DebugImage == "" {
				// Clean up the container if health check fails and no debug mode
				d.StopContainer(context.Background(), containerName)
				d.RemoveContainer(context.Background(), containerName)
			}
			return fmt.Errorf("health check failed: %w", err)
		}
		d.Output("  %s\n", d.styles.Success("Container healthy"))
	} else if opts.Background {
		// No health check, just wait a moment for container to start
		time.Sleep(500 * time.Millisecond)

		// Verify container is still running (unless debug mode is enabled - then it's ok if it crashes)
		if !d.IsContainerRunning(ctx, containerName) && opts.DebugImage == "" {
			// Get logs to help debug
			logs, _ := d.GetContainerLogs(ctx, containerName, 20)
			d.RemoveContainer(context.Background(), containerName)
			return fmt.Errorf("background container exited immediately. Logs:\n%s", logs)
		}
		d.Output("  %s\n", d.styles.Success("Container started"))
	}

	return nil
}

// waitForHealthy waits for a container to become healthy
func (d *Docker) waitForHealthy(ctx context.Context, containerName string, hc *HealthCheckConfig) error {
	interval := hc.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}

	timeout := hc.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	retries := hc.Retries
	if retries == 0 {
		retries = 10
	}

	startPeriod := hc.StartPeriod
	if startPeriod > 0 {
		d.Output("Waiting %s before starting health checks...\n", startPeriod)
		select {
		case <-time.After(startPeriod):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	for i := 0; i < retries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// First check if container is still running
		if !d.IsContainerRunning(ctx, containerName) {
			logs, _ := d.GetContainerLogs(ctx, containerName, 20)
			return fmt.Errorf("container exited before becoming healthy. Logs:\n%s", logs)
		}

		// Run health check command inside the container
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(checkCtx, "docker", "exec", containerName, "/bin/sh", "-c", hc.Cmd)
		err := cmd.Run()
		cancel()

		if err == nil {
			return nil // Health check passed
		}

		d.Output("  %s\n", d.styles.Dim(fmt.Sprintf("Health check %d/%d failed, retrying...", i+1, retries)))

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("health check failed after %d attempts", retries)
}

// IsContainerRunning checks if a container is running
func (d *Docker) IsContainerRunning(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// GetContainerLogs returns the last n lines of container logs
func (d *Docker) GetContainerLogs(ctx context.Context, containerName string, lines int) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StopContainer stops a running container
func (d *Docker) StopContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", "5", containerName)
	return cmd.Run()
}

// RemoveContainer removes a container
func (d *Docker) RemoveContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	return cmd.Run()
}

// ContainerExists checks if a container with the given name exists (running or stopped)
func (d *Docker) ContainerExists(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", containerName)
	err := cmd.Run()
	return err == nil
}

// RemoveExistingContainer removes an existing container with the given name if it exists
func (d *Docker) RemoveExistingContainer(ctx context.Context, containerName string) error {
	if !d.ContainerExists(ctx, containerName) {
		return nil // Container doesn't exist, nothing to do
	}

	d.Output("  %s\n", d.styles.Warning(fmt.Sprintf("Removing existing container '%s'...", containerName)))

	// Stop the container if running
	if d.IsContainerRunning(ctx, containerName) {
		if err := d.StopContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to stop existing container: %w", err)
		}
	}

	// Remove the container
	if err := d.RemoveContainer(ctx, containerName); err != nil {
		return fmt.Errorf("failed to remove existing container: %w", err)
	}

	return nil
}

// BuildImageOptions holds options for building an image
type BuildImageOptions struct {
	ImageName    string            // Primary image tag
	Context      string            // Build context path
	Dockerfile   string            // Dockerfile path (relative to context)
	BuildArgs    map[string]string // Build arguments
	Target       string            // Multi-stage build target
	Tags         []string          // Additional tags
	WorkflowDir  string            // Host path that serves as /workflow reference
	VolumeMounts []VolumeMount     // Additional volume mounts during build
}

// BuildImage builds an image using docker build
func (d *Docker) BuildImage(ctx context.Context, opts BuildImageOptions) (string, error) {
	if d.verbose {
		d.Output("  %s Starting docker build...\n", d.styles.Dim("[verbose]"))
	}

	// Resolve the build context path
	// The context is always relative to the workflow directory (conceptually /workflow)
	// Examples:
	//   - "" or "/workflow" → workflow root
	//   - "/workflow/Dockerfiles" → workflow_root/Dockerfiles
	//   - "./Dockerfiles" → workflow_root/Dockerfiles
	//   - "Dockerfiles" → workflow_root/Dockerfiles
	contextPath := opts.Context
	if contextPath == "" {
		contextPath = "/workflow"
	}

	// Resolve context path relative to workflow directory
	var resolvedPath string
	if strings.HasPrefix(contextPath, "/workflow") {
		// Absolute /workflow path: replace /workflow with actual workflow directory
		resolvedPath = strings.Replace(contextPath, "/workflow", opts.WorkflowDir, 1)
	} else if strings.HasPrefix(contextPath, "./") || strings.HasPrefix(contextPath, "../") || !strings.HasPrefix(contextPath, "/") {
		// Relative path: join with workflow directory
		resolvedPath = filepath.Join(opts.WorkflowDir, contextPath)
	} else {
		// Absolute path outside /workflow: use as-is
		resolvedPath = contextPath
	}

	// Make sure the path is absolute
	absContextPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve context path: %w", err)
	}

	if d.verbose {
		d.Output("  %s Build context: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(absContextPath))
	}

	// Verify context path exists
	if _, err := os.Stat(absContextPath); os.IsNotExist(err) {
		return "", fmt.Errorf("build context path does not exist: %s", absContextPath)
	}

	// Build the docker build command
	args := []string{"build"}

	// Add image tag
	args = append(args, "-t", opts.ImageName)
	if d.verbose {
		d.Output("  %s Image tag: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.ImageName))
	}

	// Add additional tags
	for _, tag := range opts.Tags {
		args = append(args, "-t", tag)
		if d.verbose {
			d.Output("  %s Additional tag: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(tag))
		}
	}

	// Add dockerfile if specified
	if opts.Dockerfile != "" {
		dockerfilePath := opts.Dockerfile

		// Resolve dockerfile path
		// If it's just a filename (no directory separators), resolve relative to build context
		// Otherwise, resolve relative to workflow directory
		var resolvedDockerfilePath string

		// Check if it's just a filename (no path separators)
		if !strings.Contains(dockerfilePath, "/") && !strings.Contains(dockerfilePath, string(filepath.Separator)) {
			// Just a filename: resolve relative to build context
			resolvedDockerfilePath = filepath.Join(absContextPath, dockerfilePath)
		} else if strings.HasPrefix(dockerfilePath, "/workflow") {
			// Absolute /workflow path: replace /workflow with actual workflow directory
			resolvedDockerfilePath = strings.Replace(dockerfilePath, "/workflow", opts.WorkflowDir, 1)
		} else if strings.HasPrefix(dockerfilePath, "./") || strings.HasPrefix(dockerfilePath, "../") || !strings.HasPrefix(dockerfilePath, "/") {
			// Relative path with directory: join with workflow directory
			resolvedDockerfilePath = filepath.Join(opts.WorkflowDir, dockerfilePath)
		} else {
			// Absolute path outside /workflow: use as-is
			resolvedDockerfilePath = dockerfilePath
		}

		args = append(args, "-f", resolvedDockerfilePath)
		if d.verbose {
			d.Output("  %s Dockerfile: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(resolvedDockerfilePath))
		}
	}

	// Add target if specified
	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
		if d.verbose {
			d.Output("  %s Build target: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.Target))
		}
	}

	// Add build args
	if len(opts.BuildArgs) > 0 && d.verbose {
		d.Output("  %s Build args: %d\n", d.styles.Dim("[verbose]"), len(opts.BuildArgs))
	}
	for key, value := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Note: Docker build doesn't support -v flag for volume mounts during build
	// Volume mounts are only available at runtime with docker run

	// Add context path
	args = append(args, absContextPath)

	if d.verbose {
		d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
	}

	// Create prefixed writers for build output
	logPrefix := d.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, d.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, d.secrets)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if d.verbose {
		d.Output("  %s Starting docker build command...\n", d.styles.Dim("[verbose]"))
	}
	if err := cmd.Run(); err != nil {
		stdoutWriter.Flush()
		stderrWriter.Flush()
		return "", fmt.Errorf("docker build failed: %w", err)
	}
	if d.verbose {
		d.Output("  %s Docker build completed successfully\n", d.styles.Dim("[verbose]"))
	}

	stdoutWriter.Flush()
	stderrWriter.Flush()

	d.Output("  %s %s\n", d.styles.Success("Built:"), d.styles.Value(opts.ImageName))
	return opts.ImageName, nil
}

// GetImageID returns the image ID for a given image name
func (d *Docker) GetImageID(ctx context.Context, imageName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Id}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get image ID for %s: %w", imageName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ImageExists checks if an image exists locally
func (d *Docker) ImageExists(ctx context.Context, imageName string) bool {
	if d.verbose {
		d.Output("  %s Checking image existence: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(imageName))
	}
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", imageName)
	exists := cmd.Run() == nil
	if d.verbose {
		if exists {
			d.Output("  %s Image found: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(imageName))
		} else {
			d.Output("  %s Image not found: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(imageName))
		}
	}
	return exists
}

// maskSecrets replaces all secret values with [secret]
func (d *Docker) maskSecrets(text string) string {
	result := text
	for _, secret := range d.secrets {
		if secret != "" {
			result = strings.ReplaceAll(result, secret, "[secret]")
		}
	}
	return result
}

// DebugSidecarOptions holds options for creating a debug sidecar container
type DebugSidecarOptions struct {
	TargetContainer  string       // Name of the container to debug
	DebugContainer   string       // Name for the debug sidecar container
	DebugImage       string       // Image to use for the sidecar (e.g., nicolaka/netshoot)
	Network          string       // Network to connect to
	OnContainerStart func(string) // Callback when container starts
	OnVolumeCreate   func(string) // Callback when volume is created (for filesystem sidecar)
}

// RunDebugSidecar creates a debug sidecar container for the target container.
// For running containers: shares namespaces (PID, network, IPC)
// For stopped containers: mounts the container's filesystem for post-mortem inspection
func (d *Docker) RunDebugSidecar(ctx context.Context, opts DebugSidecarOptions) error {
	if d.verbose {
		d.Output("  %s Creating debug sidecar for container: %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(opts.TargetContainer))
	}

	// Check if target container is running
	if d.IsContainerRunning(ctx, opts.TargetContainer) {
		// Container is running - use namespace sharing
		if d.verbose {
			d.Output("  %s Target container is running, using namespace sharing\n", d.styles.Dim("[verbose]"))
		}
		return d.runNamespaceSidecar(ctx, opts)
	}

	// Container is not running - use filesystem mount
	if d.verbose {
		d.Output("  %s Target container is not running, mounting filesystem\n", d.styles.Dim("[verbose]"))
	}
	return d.runFilesystemSidecar(ctx, opts)
}

// runNamespaceSidecar creates a sidecar sharing namespaces with a running container
func (d *Docker) runNamespaceSidecar(ctx context.Context, opts DebugSidecarOptions) error {
	// Get environment variables from the original container
	envCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.Env}}", opts.TargetContainer)
	envOutput, err := envCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get container environment: %w", err)
	}

	// Parse environment variables
	var envVars []string
	if err := json.Unmarshal(envOutput, &envVars); err != nil {
		// If parsing fails, continue without env vars
		if d.verbose {
			d.Output("  %s Warning: could not parse environment variables: %v\n", d.styles.Dim("[verbose]"), err)
		}
	}

	// Get volume mounts from the target container
	mountsCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Mounts}}", opts.TargetContainer)
	mountsOutput, err := mountsCmd.Output()
	if err != nil {
		if d.verbose {
			d.Output("  %s Warning: could not get container mounts: %v\n", d.styles.Dim("[verbose]"), err)
		}
	}

	// Parse mounts
	type MountInfo struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Mode        string `json:"Mode"`
		RW          bool   `json:"RW"`
	}
	var mounts []MountInfo
	if err := json.Unmarshal(mountsOutput, &mounts); err != nil {
		if d.verbose {
			d.Output("  %s Warning: could not parse container mounts: %v\n", d.styles.Dim("[verbose]"), err)
		}
	}

	// Build docker run command with namespace sharing
	args := []string{
		"run",
		"-d",                          // Detached
		"--name", opts.DebugContainer, // Container name
		"--pid=container:" + opts.TargetContainer,     // Share PID namespace
		"--network=container:" + opts.TargetContainer, // Share network namespace
		"--ipc=container:" + opts.TargetContainer,     // Share IPC namespace
		// Note: We don't share the mount namespace because that would hide the debug tools.
		// Instead, we mount the same volumes directly in the sidecar.
		"--init",               // Use tini for proper signal handling
		"--cap-add=SYS_PTRACE", // Allow debugging processes
		"--cap-add=SYS_ADMIN",  // Allow various admin operations
	}

	// Add environment variables
	for _, env := range envVars {
		args = append(args, "-e", env)
	}

	// Add volume mounts from target container to /target/<destination>
	// This makes them accessible at predictable paths in the sidecar
	for _, mount := range mounts {
		if mount.Type == "bind" || mount.Type == "volume" {
			mode := "ro"
			if mount.RW {
				mode = "rw"
			}
			// Mount to /target/<destination> so users can find them easily
			targetPath := "/target" + mount.Destination
			args = append(args, "-v", fmt.Sprintf("%s:%s:%s", mount.Source, targetPath, mode))
			if d.verbose {
				d.Output("  %s Mounting volume: %s -> %s (%s)\n", d.styles.Dim("[verbose]"), mount.Source, targetPath, mode)
			}
		}
	}

	// Add working directory and image last
	// Set a safe working directory within the sidecar's filesystem
	// Users can access target filesystem via /proc/1/root/ after attaching
	args = append(args,
		"-w", "/", // Working directory within sidecar's own filesystem
		opts.DebugImage,     // The debug image
		"sleep", "infinity", // Keep the container running
	)

	if d.verbose {
		d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create namespace sidecar: %w\nOutput: %s", err, string(output))
	}

	if d.verbose {
		d.Output("  %s Namespace sidecar created successfully\n", d.styles.Dim("[verbose]"))
	}

	return nil
}

// runFilesystemSidecar creates a sidecar mounting a stopped container's filesystem
func (d *Docker) runFilesystemSidecar(ctx context.Context, opts DebugSidecarOptions) error {
	// Create a named volume for the exported filesystem
	volumeName := fmt.Sprintf("ocw-debug-%s-%d", opts.TargetContainer, time.Now().UnixNano())

	if d.verbose {
		d.Output("  %s Creating debug volume %s...\n", d.styles.Dim("[verbose]"), d.styles.Dim(volumeName))
	}

	// Create the volume
	createCmd := exec.CommandContext(ctx, "docker", "volume", "create", volumeName)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create debug volume: %w\nOutput: %s", err, string(output))
	}

	// Register volume for cleanup
	if opts.OnVolumeCreate != nil {
		opts.OnVolumeCreate(volumeName)
	}

	// Export container filesystem and pipe into a temporary container that copies it to the volume
	if d.verbose {
		d.Output("  %s Exporting container filesystem...\n", d.styles.Dim("[verbose]"))
	}

	// Create a temp container to populate the volume
	tempContainer := fmt.Sprintf("ocw-debug-temp-%d", time.Now().UnixNano())
	tempCmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", tempContainer,
		"-v", volumeName+":/debugfs",
		"alpine", "sleep", "infinity")
	if output, err := tempCmd.CombinedOutput(); err != nil {
		// Clean up volume
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to create temp container: %w\nOutput: %s", err, string(output))
	}

	// Export and extract
	exportCmd := exec.CommandContext(ctx, "docker", "export", opts.TargetContainer)
	extractCmd := exec.CommandContext(ctx, "docker", "exec", "-i", tempContainer, "tar", "-xf", "-", "-C", "/debugfs")

	exportPipe, _ := exportCmd.StdoutPipe()
	extractCmd.Stdin = exportPipe

	if err := extractCmd.Start(); err != nil {
		dockerRm(ctx, tempContainer)
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to start extract: %w", err)
	}

	if err := exportCmd.Run(); err != nil {
		dockerRm(ctx, tempContainer)
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to export container: %w", err)
	}

	if err := extractCmd.Wait(); err != nil {
		dockerRm(ctx, tempContainer)
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to extract container filesystem: %w", err)
	}

	// Remove temp container
	dockerRm(ctx, tempContainer)

	if d.verbose {
		d.Output("  %s Container filesystem exported to volume\n", d.styles.Dim("[verbose]"))
	}

	// Get all mounts from the original container
	mountsCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Mounts}}", opts.TargetContainer)
	mountsOutput, err := mountsCmd.Output()
	if err != nil {
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to get container mounts: %w", err)
	}

	// Get environment variables from the original container
	envCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.Env}}", opts.TargetContainer)
	envOutput, err := envCmd.Output()
	if err != nil {
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to get container environment: %w", err)
	}

	// Parse environment variables
	var envVars []string
	if err := json.Unmarshal(envOutput, &envVars); err != nil {
		// If parsing fails, continue without env vars
		if d.verbose {
			d.Output("  %s Warning: could not parse environment variables: %v\n", d.styles.Dim("[verbose]"), err)
		}
	}

	// Create a sidecar that mounts the debug volume
	args := []string{
		"run",
		"-d",                          // Detached
		"--name", opts.DebugContainer, // Container name
		"--init",                         // Use tini for proper signal handling
		"-v", volumeName + ":/target:ro", // Mount exported filesystem as read-only
	}

	// Add environment variables
	for _, env := range envVars {
		args = append(args, "-e", env)
	}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	// Re-mount bind mounts from the original container inside /target
	// This recreates the full container filesystem view for debugging
	var mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Mode        string `json:"Mode"`
	}
	if err := json.Unmarshal(mountsOutput, &mounts); err == nil {
		for _, mount := range mounts {
			if mount.Type == "bind" || mount.Type == "volume" {
				mode := "ro"
				if strings.Contains(mount.Mode, "rw") {
					mode = "rw"
				}

				// Mount inside /target to recreate full container filesystem view
				targetPath := filepath.Join("/target", mount.Destination)
				args = append(args, "-v", fmt.Sprintf("%s:%s:%s", mount.Source, targetPath, mode))

				if d.verbose {
					d.Output("  %s Re-mounting %s -> %s (%s)\n", d.styles.Dim("[verbose]"), d.styles.Dim(mount.Source), d.styles.Dim(targetPath), d.styles.Dim(mode))
				}
			}
		}
	}

	// Add image and command last
	args = append(args,
		"-w", "/target", // Set working directory to /target
		opts.DebugImage,     // The debug image
		"sleep", "infinity", // Keep the container running
	)

	if d.verbose {
		d.Output("  %s Executing: docker %s\n", d.styles.Dim("[verbose]"), d.styles.Dim(strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up volume on failure
		exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).Run()
		return fmt.Errorf("failed to create filesystem sidecar: %w\nOutput: %s", err, string(cmdOutput))
	}

	if d.verbose {
		d.Output("  %s Filesystem sidecar created successfully\n", d.styles.Dim("[verbose]"))
	}

	return nil
}

// dockerRm removes a container, ignoring errors
func dockerRm(ctx context.Context, name string) {
	exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

// StreamLogs streams container logs (for long-running containers)
func (d *Docker) StreamLogs(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(d.maskSecrets(scanner.Text()))
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, d.maskSecrets(scanner.Text()))
		}
	}()

	return cmd.Wait()
}
