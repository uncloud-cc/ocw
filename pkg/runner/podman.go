package runner

import (
	"bufio"
	"bytes"
	"context"
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

// Podman wraps podman CLI commands
type Podman struct {
	// Output function for logging
	Output func(format string, args ...any)
	// styles provides styled output formatting
	styles *Styles
	// secrets contains sensitive values to mask in output
	secrets []string
}

// NetworkCreateOptions holds options for creating a network
type NetworkCreateOptions struct {
	Name   string // Network name
	Driver string // Network driver (default: bridge)
}

// CreateNetwork creates a Podman network
func (p *Podman) CreateNetwork(ctx context.Context, opts NetworkCreateOptions) error {
	// Check if network already exists
	checkCmd := exec.CommandContext(ctx, "podman", "network", "exists", opts.Name)
	if err := checkCmd.Run(); err == nil {
		// Network exists, silently continue
		return nil
	}

	driver := opts.Driver
	if driver == "" {
		driver = "bridge"
	}

	args := []string{"network", "create", "--driver", driver, opts.Name}

	cmd := exec.CommandContext(ctx, "podman", args...)
	// Suppress network creation output - not interesting for users
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create network %s: %w", opts.Name, err)
	}

	return nil
}

// RemoveNetwork removes a Podman network
func (p *Podman) RemoveNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "podman", "network", "rm", "-f", name)
	return cmd.Run()
}

// NetworkExists checks if a network exists
func (p *Podman) NetworkExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "podman", "network", "exists", name)
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

// NewPodman creates a new Podman wrapper
func NewPodman(output func(format string, args ...any), styles *Styles, secrets []string) *Podman {
	return &Podman{Output: output, styles: styles, secrets: secrets}
}

// SetSecrets updates the secrets to mask in output
func (p *Podman) SetSecrets(secrets []string) {
	p.secrets = secrets
}

// PullImage pulls an image if not present locally
func (p *Podman) PullImage(ctx context.Context, imageName string) error {
	// Check if image exists locally
	checkCmd := exec.CommandContext(ctx, "podman", "image", "exists", imageName)
	if err := checkCmd.Run(); err == nil {
		p.Output("  %s %s\n", p.styles.Dim("Image exists:"), p.styles.Value(imageName))
		return nil
	}

	p.Output("  %s %s\n", p.styles.Info("Pulling:"), p.styles.Value(imageName))

	// Create prefixed writers for pull output
	logPrefix := p.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, p.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, p.secrets)

	cmd := exec.CommandContext(ctx, "podman", "pull", imageName)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Run(); err != nil {
		stdoutWriter.Flush()
		stderrWriter.Flush()
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
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
	Name         string             // Container name (optional, auto-generated if empty)
	Hostname     string             // Hostname for the container (for DNS resolution in network)
	Network      string             // Network to connect to (empty = default podman network)
	Image        string             // Image to run
	Cmd          string             // Command string (will be passed to shell)
	Args         []string           // Command arguments (if Cmd is empty)
	Entrypoint   string             // Override entrypoint
	Env          map[string]string  // Environment variables
	WorkDir      string             // Working directory inside container
	WorkflowDir  string             // Host path to mount as /workflow
	TTY          bool               // Allocate TTY
	Remove       bool               // Remove container after exit (default true for non-background)
	Background   bool               // Run in background (detached)
	HealthCheck  *HealthCheckConfig // Health check for background containers
	PortMappings []PortMapping      // Ports to expose from container to host
}

// RunContainer runs a container and waits for it to complete
func (p *Podman) RunContainer(ctx context.Context, opts RunContainerOptions) error {
	args := []string{"run"}

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
	if containerName != "" {
		args = append(args, "--name", containerName)
	}

	// Network - connect to specified network
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
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
	}

	// Port mappings
	for _, pm := range opts.PortMappings {
		args = append(args, "-p", fmt.Sprintf("%d:%d", pm.HostPort, pm.ContainerPort))
	}

	// TTY
	if opts.TTY {
		args = append(args, "-t")
	}

	// Environment variables
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
	}

	// Entrypoint override
	if opts.Entrypoint != "" {
		args = append(args, "--entrypoint", opts.Entrypoint)
	}

	// Image
	args = append(args, opts.Image)

	// Command - if Cmd is set, run it through shell
	if opts.Cmd != "" {
		args = append(args, "/bin/sh", "-c", opts.Cmd)
	} else if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
	}

	// Create prefixed writers for container output
	logPrefix := p.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, p.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, p.secrets)

	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if !opts.Background {
		cmd.Stdin = os.Stdin
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

	// Flush any remaining output
	stdoutWriter.Flush()
	stderrWriter.Flush()

	// For background containers, wait for health check if configured
	if opts.Background && opts.HealthCheck != nil {
		p.Output("  %s\n", p.styles.Dim("Waiting for health check..."))
		if err := p.waitForHealthy(ctx, containerName, opts.HealthCheck); err != nil {
			// Clean up the container if health check fails
			p.StopContainer(context.Background(), containerName)
			p.RemoveContainer(context.Background(), containerName)
			return fmt.Errorf("health check failed: %w", err)
		}
		p.Output("  %s\n", p.styles.Success("Container healthy"))
	} else if opts.Background {
		// No health check, just wait a moment for container to start
		time.Sleep(500 * time.Millisecond)

		// Verify container is still running
		if !p.IsContainerRunning(ctx, containerName) {
			// Get logs to help debug
			logs, _ := p.GetContainerLogs(ctx, containerName, 20)
			p.RemoveContainer(context.Background(), containerName)
			return fmt.Errorf("background container exited immediately. Logs:\n%s", logs)
		}
		p.Output("  %s\n", p.styles.Success("Container started"))
	}

	return nil
}

// waitForHealthy waits for a container to become healthy
func (p *Podman) waitForHealthy(ctx context.Context, containerName string, hc *HealthCheckConfig) error {
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
		p.Output("Waiting %s before starting health checks...\n", startPeriod)
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
		if !p.IsContainerRunning(ctx, containerName) {
			logs, _ := p.GetContainerLogs(ctx, containerName, 20)
			return fmt.Errorf("container exited before becoming healthy. Logs:\n%s", logs)
		}

		// Run health check command inside the container
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(checkCtx, "podman", "exec", containerName, "/bin/sh", "-c", hc.Cmd)
		err := cmd.Run()
		cancel()

		if err == nil {
			return nil // Health check passed
		}

		p.Output("  %s\n", p.styles.Dim(fmt.Sprintf("Health check %d/%d failed, retrying...", i+1, retries)))

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("health check failed after %d attempts", retries)
}

// IsContainerRunning checks if a container is running
func (p *Podman) IsContainerRunning(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "podman", "inspect", "--format", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// GetContainerLogs returns the last n lines of container logs
func (p *Podman) GetContainerLogs(ctx context.Context, containerName string, lines int) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", "logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StopContainer stops a running container
func (p *Podman) StopContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "podman", "stop", "-t", "5", containerName)
	return cmd.Run()
}

// RemoveContainer removes a container
func (p *Podman) RemoveContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", containerName)
	return cmd.Run()
}

// BuildImageOptions holds options for building an image
type BuildImageOptions struct {
	ImageName   string            // Primary image tag
	Context     string            // Build context path
	Dockerfile  string            // Dockerfile path (relative to context)
	BuildArgs   map[string]string // Build arguments
	Target      string            // Multi-stage build target
	Tags        []string          // Additional tags
	WorkflowDir string            // Host path that serves as /workflow reference
}

// BuildImage builds an image using podman build
func (p *Podman) BuildImage(ctx context.Context, opts BuildImageOptions) (string, error) {
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

	// Verify context path exists
	if _, err := os.Stat(absContextPath); os.IsNotExist(err) {
		return "", fmt.Errorf("build context path does not exist: %s", absContextPath)
	}

	// Build the podman build command
	args := []string{"build"}

	// Add image tag
	args = append(args, "-t", opts.ImageName)

	// Add additional tags
	for _, tag := range opts.Tags {
		args = append(args, "-t", tag)
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
	}

	// Add target if specified
	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
	}

	// Add build args
	for key, value := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Mount workflow directory at /workflow during build
	// This makes the workflow directory available at /workflow inside the Dockerfile
	// consistent with how run steps work
	if opts.WorkflowDir != "" {
		absWorkflowDir, err := filepath.Abs(opts.WorkflowDir)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for workflow dir: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/workflow:ro", absWorkflowDir))
	}

	// Add context path
	args = append(args, absContextPath)

	// Create prefixed writers for build output
	logPrefix := p.styles.LogPrefix()
	stdoutWriter := newPrefixWriter(os.Stdout, logPrefix, p.secrets)
	stderrWriter := newPrefixWriter(os.Stderr, logPrefix, p.secrets)

	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Run(); err != nil {
		stdoutWriter.Flush()
		stderrWriter.Flush()
		return "", fmt.Errorf("podman build failed: %w", err)
	}

	stdoutWriter.Flush()
	stderrWriter.Flush()

	p.Output("  %s %s\n", p.styles.Success("Built:"), p.styles.Value(opts.ImageName))
	return opts.ImageName, nil
}

// GetImageID returns the image ID for a given image name
func (p *Podman) GetImageID(ctx context.Context, imageName string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", "image", "inspect", "--format", "{{.Id}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get image ID for %s: %w", imageName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ImageExists checks if an image exists locally
func (p *Podman) ImageExists(ctx context.Context, imageName string) bool {
	cmd := exec.CommandContext(ctx, "podman", "image", "exists", imageName)
	return cmd.Run() == nil
}

// maskSecrets replaces all secret values with [secret]
func (p *Podman) maskSecrets(text string) string {
	result := text
	for _, secret := range p.secrets {
		if secret != "" {
			result = strings.ReplaceAll(result, secret, "[secret]")
		}
	}
	return result
}

// StreamLogs streams container logs (for long-running containers)
func (p *Podman) StreamLogs(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "podman", "logs", "-f", containerName)

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
			fmt.Println(p.maskSecrets(scanner.Text()))
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, p.maskSecrets(scanner.Text()))
		}
	}()

	return cmd.Wait()
}
