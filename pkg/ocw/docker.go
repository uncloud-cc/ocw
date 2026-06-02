package ocw

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

type DockerRuntime struct {
	volumes     schema.Volumes
	workflowDir string
}

func NewDockerRuntime(volumes schema.Volumes, workflowDir string) (*DockerRuntime, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker CLI not found in PATH")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		return nil, fmt.Errorf("docker daemon unreachable: %w", err)
	}
	return &DockerRuntime{volumes: volumes, workflowDir: workflowDir}, nil
}

func (d *DockerRuntime) Close() error { return nil }

func (d *DockerRuntime) Run(ctx context.Context, step *schema.RunStep) (map[string]string, error) {
	if step.Background {
		return nil, fmt.Errorf("background containers are not yet supported")
	}
	if err := d.execDocker(ctx, "run", d.buildRunArgs(step)...); err != nil {
		return nil, err
	}
	return map[string]string{}, nil
}

func (d *DockerRuntime) Build(ctx context.Context, step *schema.BuildStep) (map[string]string, error) {
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

	if err := d.execDocker(ctx, "build", args...); err != nil {
		return nil, err
	}

	// Read the image ID (digest) written by docker.
	imageIDBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read iidfile: %w", err)
	}

	return map[string]string{
		"image":     strings.TrimSpace(string(imageIDBytes)),
		"imageName": step.Build.Image,
	}, nil
}

// buildRunArgs translates a RunStep into the docker CLI args.
// Pure function: easy to unit-test without touching docker.
func (d *DockerRuntime) buildRunArgs(step *schema.RunStep) []string {
	args := []string{"run", "--rm"}
	args = append(args, "-v", d.workflowDir+":/workflow")
	if step.Workdir != "" {
		args = append(args, "-w", step.Workdir)
	}
	// Env from StepBase (already interpolated by interpolateStepbase)
	for k, v := range step.Env {
		args = append(args, "-e", k+"="+v)
	}
	// future: volumes, entrypoint, expose, ...
	args = append(args, step.Image)
	if step.Cmd != "" {
		args = append(args, "sh", "-c", step.Cmd)
	}
	return args
}

func (d *DockerRuntime) buildBuildArgs(step *schema.BuildStep) []string {
	contextPath := step.Build.Context
	if contextPath == "" {
		contextPath = d.workflowDir
	}
	args := []string{"build", "-t", step.Build.Image}
	if step.Build.Dockerfile != "" {
		args = append(args, "-f", step.Build.Dockerfile)
	}
	args = append(args, contextPath)
	return args
}

func (d *DockerRuntime) execDocker(ctx context.Context, op string, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr, cmd.Dir = os.Stdout, os.Stderr, d.workflowDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s failed: %w", op, err)
	}
	return nil
}
