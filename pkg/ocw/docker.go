package ocw

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gonanoid "github.com/matoous/go-nanoid"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// linePrefixWriter buffers writes and emits each complete line through the printer.
// In pretty mode it formats with prefix + colored separator.
// In JSON mode it emits container.output events.
type linePrefixWriter struct {
	printer *Printer
	prefix  string // step name/ID
	stream  string // "stdout" or "stderr"
	buf     []byte
}

func (w *linePrefixWriter) emit(line string) {
	w.printer.PrintContainerOutput(w.prefix, w.stream, line)
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := 0
		for i, b := range w.buf {
			if b == '\n' {
				idx = i + 1
				break
			}
		}
		if idx == 0 {
			break
		}
		line := string(w.buf[:idx-1]) // exclude the newline
		// Strip trailing \r from CRLF line endings (common in Docker container output).
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		w.emit(line)
		w.buf = w.buf[idx:]
	}
	return len(p), nil
}

func (w *linePrefixWriter) Flush() {
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}

type DockerRuntime struct {
	volumes              schema.Volumes
	workflowDir          string
	printer              *Printer
	backgroundContainers []string
	services             []ServiceInfo
	networkName          string
	logCtx               context.Context
	logCancel            context.CancelFunc
	logWg                sync.WaitGroup
	mu                   sync.Mutex
}

func NewDockerRuntime(volumes schema.Volumes, workflowDir string, printer *Printer, runID string) (*DockerRuntime, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker CLI not found in PATH")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		return nil, fmt.Errorf("docker daemon unreachable: %w", err)
	}

	if runID == "" {
		var err error
		runID, err = gonanoid.Generate(NanoidAlphabet, 12)
		if err != nil {
			return nil, fmt.Errorf("cannot create runID: %w", err)
		}
	}

	networkName := "ocw-" + runID
	if err := exec.Command("docker", "network", "create", networkName).Run(); err != nil {
		return nil, fmt.Errorf("failed to create docker network %s: %w", networkName, err)
	}

	logCtx, logCancel := context.WithCancel(context.Background())
	return &DockerRuntime{
		volumes:     volumes,
		workflowDir: workflowDir,
		printer:     printer,
		networkName: networkName,
		logCtx:      logCtx,
		logCancel:   logCancel,
	}, nil
}

func (d *DockerRuntime) Close() error {
	d.mu.Lock()
	containers := make([]string, len(d.backgroundContainers))
	copy(containers, d.backgroundContainers)
	networkName := d.networkName
	d.backgroundContainers = d.backgroundContainers[:0]
	d.services = d.services[:0]
	d.mu.Unlock()

	// 1. Stop log streaming goroutines.
	if d.logCancel != nil {
		d.logCancel()
	}
	d.logWg.Wait()

	// 2. Remove containers.
	var firstErr error
	for _, id := range containers {
		cmd := exec.Command("docker", "rm", "-f", id)
		if err := cmd.Run(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to remove container %s: %w", id, err)
		}
	}

	// 3. Remove network.
	if networkName != "" {
		if err := exec.Command("docker", "network", "rm", networkName).Run(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to remove network %s: %w", networkName, err)
		}
	}
	return firstErr
}

func (d *DockerRuntime) HasActiveServices() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.services) > 0
}

func (d *DockerRuntime) ListServices() []ServiceInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]ServiceInfo, len(d.services))
	copy(out, d.services)
	return out
}

func (d *DockerRuntime) Run(ctx context.Context, step *schema.RunStep, prefix string) (map[string]string, error) {
	d.printer.Debug("docker_run_start", map[string]any{
		"image":  step.Image,
		"cmd":    step.Cmd,
		"prefix": prefix,
	})

	outputsFile, err := createTempOutputsFile()
	if err != nil {
		return nil, err
	}
	defer os.Remove(outputsFile)

	if err := d.execDocker(ctx, "run", prefix, d.buildRunArgs(step, outputsFile)...); err != nil {
		d.printer.Error("docker_run_failed", map[string]any{
			"image": step.Image,
			"error": err.Error(),
		})
		return nil, err
	}
	outputs, err := parseOutputsFile(outputsFile)
	if err != nil {
		return nil, err
	}
	d.printer.Debug("docker_run_complete", map[string]any{
		"image":   step.Image,
		"outputs": len(outputs),
	})
	return outputs, nil
}

func (d *DockerRuntime) StartService(ctx context.Context, step *schema.RunStep, prefix string) (map[string]string, error) {
	if !step.Background {
		return nil, fmt.Errorf("StartService called on a non-background step")
	}

	d.printer.Debug("docker_service_start", map[string]any{
		"image":  step.Image,
		"cmd":    step.Cmd,
		"prefix": prefix,
	})

	args := d.buildServiceArgs(step)

	d.printer.Debug("docker_exec", map[string]any{
		"op":   "run",
		"args": args,
	})

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			d.printer.Error("docker_service_failed", map[string]any{
				"image": step.Image,
				"error": string(ee.Stderr),
			})
			return nil, fmt.Errorf("docker run -d failed: %s", string(ee.Stderr))
		}
		return nil, fmt.Errorf("docker run -d failed: %w", err)
	}

	containerID := strings.TrimSpace(string(out))
	d.mu.Lock()
	d.backgroundContainers = append(d.backgroundContainers, containerID)
	d.mu.Unlock()

	d.printer.Debug("docker_service_started", map[string]any{
		"image":       step.Image,
		"containerID": containerID,
	})

	running, err := d.isContainerRunning(ctx, containerID)
	if err != nil {
		d.stopAndRemove(containerID)
		return nil, fmt.Errorf("failed to check container status: %w", err)
	}
	if !running {
		d.stopAndRemove(containerID)
		return nil, fmt.Errorf("container exited immediately after start")
	}

	name := step.Name
	if name == "" {
		name = step.Image
	}

	// Background services always get a log prefix so their output is identifiable.
	logPrefix := prefix
	if logPrefix == "" {
		if step.ID != "" {
			logPrefix = string(step.ID)
		} else if step.Name != "" {
			logPrefix = step.Name
		} else {
			logPrefix = step.Image
		}
	}

	// Start streaming container logs as soon as the container is running.
	d.logWg.Add(1)
	go func() {
		defer d.logWg.Done()
		d.streamLogs(d.logCtx, containerID, logPrefix)
	}()

	if step.HealthCheck != nil {
		if err := d.waitForHealthy(ctx, name, containerID, step.HealthCheck); err != nil {
			d.stopAndRemove(containerID)
			return nil, err
		}
	}

	outputs := map[string]string{
		"containerID": containerID,
	}

	var exposed []ExposedServicePort
	if step.Expose != nil {
		for _, port := range step.Expose.Ports {
			key := fmt.Sprintf("port_%d", port.ContainerPort)
			outputs[key] = fmt.Sprintf("%s://localhost:%d", port.Protocol, port.HostPort)
			exposed = append(exposed, ExposedServicePort{
				Protocol:      port.Protocol,
				HostPort:      port.HostPort,
				ContainerPort: port.ContainerPort,
			})
		}
	}

	d.mu.Lock()
	d.services = append(d.services, ServiceInfo{
		Name:        name,
		ContainerID: containerID,
		Exposed:     exposed,
	})
	d.mu.Unlock()

	d.printer.Debug("docker_service_ready", map[string]any{
		"image":       step.Image,
		"containerID": containerID,
	})

	return outputs, nil
}

func (d *DockerRuntime) isContainerRunning(ctx context.Context, containerID string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Running}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func (d *DockerRuntime) getHealthStatus(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{end}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *DockerRuntime) waitForHealthy(ctx context.Context, name, containerID string, hc *schema.HealthCheck) error {
	// Snappy defaults — must stay in sync with buildServiceArgs.
	interval := 500 * time.Millisecond
	if hc.Interval != "" {
		if dur, err := time.ParseDuration(hc.Interval); err == nil {
			interval = dur
		}
	}

	timeout := 1 * time.Second
	if hc.Timeout != "" {
		if dur, err := time.ParseDuration(hc.Timeout); err == nil {
			timeout = dur
		}
	}

	startPeriod := 0 * time.Second
	if hc.StartPeriod != "" {
		if dur, err := time.ParseDuration(hc.StartPeriod); err == nil {
			startPeriod = dur
		}
	}

	retries := 10
	if hc.Retries > 0 {
		retries = hc.Retries
	}

	maxCheckTime := interval
	if timeout > maxCheckTime {
		maxCheckTime = timeout
	}
	overallTimeout := startPeriod + time.Duration(retries)*interval + 2*maxCheckTime

	ctx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	d.printer.PrintHealthCheckStart(name)
	start := time.Now()
	attempt := 0

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.printer.PrintHealthCheckEnd(name, false, time.Since(start))
			return fmt.Errorf("health check timed out after %v", overallTimeout)
		case <-ticker.C:
			attempt++
			status, err := d.getHealthStatus(ctx, containerID)
			if err != nil {
				d.printer.PrintHealthCheckEnd(name, false, time.Since(start))
				return fmt.Errorf("health check inspect failed: %w", err)
			}
			d.printer.PrintHealthCheckTick(name, attempt, status)
			switch status {
			case "healthy":
				d.printer.PrintHealthCheckEnd(name, true, time.Since(start))
				return nil
			case "unhealthy":
				d.printer.PrintHealthCheckEnd(name, false, time.Since(start))
				return fmt.Errorf("container became unhealthy")
			}
		}
	}
}

func (d *DockerRuntime) streamLogs(ctx context.Context, containerID, prefix string) {
	args := []string{"logs", "-f", "--since", "0s", containerID}
	cmd := exec.CommandContext(ctx, "docker", args...)
	outWriter := &linePrefixWriter{printer: d.printer, prefix: prefix, stream: "stdout"}
	errWriter := &linePrefixWriter{printer: d.printer, prefix: prefix, stream: "stderr"}
	cmd.Stdout, cmd.Stderr = outWriter, errWriter
	if err := cmd.Run(); err != nil {
		// Container exited or was stopped — expected on shutdown.
	}
	outWriter.Flush()
	errWriter.Flush()
}

func (d *DockerRuntime) stopAndRemove(containerID string) {
	d.mu.Lock()
	for i, id := range d.backgroundContainers {
		if id == containerID {
			d.backgroundContainers = append(d.backgroundContainers[:i], d.backgroundContainers[i+1:]...)
			break
		}
	}
	for i, svc := range d.services {
		if svc.ContainerID == containerID {
			d.services = append(d.services[:i], d.services[i+1:]...)
			break
		}
	}
	d.mu.Unlock()
	cmd := exec.Command("docker", "rm", "-f", containerID)
	_ = cmd.Run()
}

func (d *DockerRuntime) Build(ctx context.Context, step *schema.BuildStep, prefix string) (map[string]string, error) {
	d.printer.Debug("docker_build_start", map[string]any{
		"image":   step.Build.Image,
		"context": step.Build.Context,
		"prefix":  prefix,
	})

	// Create a temp file for docker to write the built image ID into.
	tmpFile, err := os.CreateTemp("", "ocw-iid-*")
	if err != nil {
		return nil, fmt.Errorf("create temp iidfile: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Build the arg list and inject --iidfile before the context path (last arg).
	args := d.buildBuildArgs(step)
	if len(args) > 0 {
		last := args[len(args)-1]
		args = append(args[:len(args)-1], "--iidfile", tmpPath, last)
	}

	if err := d.execDocker(ctx, "build", prefix, args...); err != nil {
		d.printer.Error("docker_build_failed", map[string]any{
			"image": step.Build.Image,
			"error": err.Error(),
		})
		return nil, err
	}

	// Read the image ID (digest) written by docker.
	imageIDBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read iidfile: %w", err)
	}

	imageID := strings.TrimSpace(string(imageIDBytes))
	d.printer.Debug("docker_build_complete", map[string]any{
		"image":   step.Build.Image,
		"imageID": imageID,
	})

	return map[string]string{
		"image":     imageID,
		"imageName": step.Build.Image,
	}, nil
}

// buildCommonRunArgs returns the args shared between foreground and background runs.
func (d *DockerRuntime) buildCommonRunArgs(step *schema.RunStep) []string {
	args := []string{"--network", d.networkName}
	args = append(args, "-v", d.workflowDir+":/workflow")

	for _, ref := range step.Volumes {
		mount := d.buildVolumeMount(ref)
		if mount != "" {
			args = append(args, "-v", mount)
		}
	}

	if step.Workdir != "" {
		args = append(args, "-w", step.Workdir)
	} else {
		args = append(args, "-w", "/workflow")
	}
	for k, v := range step.Env {
		args = append(args, "-e", k+"="+v)
	}
	return args
}

// buildVolumeMount translates a VolumeRef into a docker -v string.
// Returns "" if the volume is unknown.
func (d *DockerRuntime) buildVolumeMount(ref schema.VolumeRef) string {
	vol, ok := d.volumes[ref.Name]
	if !ok {
		d.printer.Warn("unknown_volume", map[string]any{"name": ref.Name})
		return ""
	}

	hostPath := vol.Path
	if !filepath.IsAbs(hostPath) {
		hostPath = filepath.Join(d.workflowDir, hostPath)
	}

	mountPath := ref.MountPath
	if mountPath == "" {
		mountPath = vol.MountPath
	}
	if mountPath == "" {
		mountPath = "/volumes/" + ref.Name
	}

	readOnly := true // default
	if vol.Mode == schema.VolumeModeReadWrite {
		readOnly = false
	}
	if ref.ReadOnly != nil {
		readOnly = *ref.ReadOnly
	}

	if readOnly {
		mountPath += ":ro"
	}

	return hostPath + ":" + mountPath
}

// buildRunArgs translates a RunStep into the docker CLI args for a foreground container.
func (d *DockerRuntime) buildRunArgs(step *schema.RunStep, outputsFile string) []string {
	args := []string{"run", "--rm"}
	args = append(args, d.buildCommonRunArgs(step)...)

	if outputsFile != "" {
		const containerPath = "/tmp/ocw-outputs"
		args = append(args,
			"-v", outputsFile+":"+containerPath,
			"-e", "OUTPUTS="+containerPath,
		)
	}

	args = append(args, step.Image)
	if step.Cmd != "" {
		args = append(args, "sh", "-c", step.Cmd)
	}
	return args
}

// buildServiceArgs translates a RunStep into the docker CLI args for a background container.
func (d *DockerRuntime) buildServiceArgs(step *schema.RunStep) []string {
	args := []string{"run", "-d"}
	args = append(args, d.buildCommonRunArgs(step)...)

	if step.ID != "" {
		args = append(args, "--network-alias", string(step.ID))
	}

	if step.Expose != nil {
		for _, port := range step.Expose.Ports {
			proto := port.Protocol
			if proto == "http" || proto == "https" {
				proto = "tcp"
			}
			args = append(args, "-p", fmt.Sprintf("%d:%d/%s", port.HostPort, port.ContainerPort, proto))
		}
	}

	if step.HealthCheck != nil {
		// Snappy defaults — override Docker's sluggish 30s defaults.
		interval := "500ms"
		if step.HealthCheck.Interval != "" {
			interval = step.HealthCheck.Interval
		}
		timeout := "1s"
		if step.HealthCheck.Timeout != "" {
			timeout = step.HealthCheck.Timeout
		}
		retries := 10
		if step.HealthCheck.Retries > 0 {
			retries = step.HealthCheck.Retries
		}
		startPeriod := "0s"
		if step.HealthCheck.StartPeriod != "" {
			startPeriod = step.HealthCheck.StartPeriod
		}

		if step.HealthCheck.Cmd != "" {
			args = append(args, "--health-cmd", step.HealthCheck.Cmd)
		}
		args = append(args, "--health-interval", interval)
		args = append(args, "--health-timeout", timeout)
		args = append(args, "--health-retries", fmt.Sprintf("%d", retries))
		args = append(args, "--health-start-period", startPeriod)
	}

	args = append(args, step.Image)
	if step.Cmd != "" {
		args = append(args, "sh", "-c", step.Cmd)
	}
	return args
}

// createTempOutputsFile creates an empty host temp file for a container to write
// key=value lines into. The caller is responsible for removing it.
func createTempOutputsFile() (string, error) {
	f, err := os.CreateTemp("", "ocw-outputs-*")
	if err != nil {
		return "", fmt.Errorf("create outputs temp file: %w", err)
	}
	name := f.Name()
	f.Close()
	return name, nil
}

// parseOutputsFile reads a file of "key=value" lines into a map.
// Lines without "=" and empty lines are ignored.
func parseOutputsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// If the step never wrote to $OUTPUTS, the file is empty.
		return map[string]string{}, nil
	}

	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out, nil
}

func (d *DockerRuntime) buildBuildArgs(step *schema.BuildStep) []string {
	contextPath := step.Build.Context
	if contextPath == "" {
		contextPath = d.workflowDir
	}

	// Resolve context path to absolute for path calculations
	absContext := contextPath
	if !filepath.IsAbs(absContext) {
		absContext = filepath.Join(d.workflowDir, absContext)
	}

	args := []string{"build", "-t", step.Build.Image}
	if step.Build.Dockerfile != "" {
		dockerfile := step.Build.Dockerfile
		if !filepath.IsAbs(dockerfile) {
			dockerfile = filepath.Join(absContext, dockerfile)
		}
		args = append(args, "-f", dockerfile)
	}
	args = append(args, contextPath)
	return args
}

func (d *DockerRuntime) execDocker(ctx context.Context, op, prefix string, args ...string) error {
	d.printer.Debug("docker_exec", map[string]any{
		"op":   op,
		"args": args,
	})
	cmd := exec.CommandContext(ctx, "docker", args...)
	outWriter := &linePrefixWriter{printer: d.printer, prefix: prefix, stream: "stdout"}
	errWriter := &linePrefixWriter{printer: d.printer, prefix: prefix, stream: "stderr"}
	cmd.Stdout, cmd.Stderr, cmd.Dir = outWriter, errWriter, d.workflowDir
	if err := cmd.Run(); err != nil {
		outWriter.Flush()
		errWriter.Flush()
		return fmt.Errorf("docker %s failed: %w", op, err)
	}
	outWriter.Flush()
	errWriter.Flush()
	return nil
}
