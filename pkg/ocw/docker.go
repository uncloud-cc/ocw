package ocw

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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
	args := []string{"run", "--rm", "-v", fmt.Sprintf("%s:/workflow", d.workflowDir), "-w", "/workflow"}
	if step.Workdir != "" {
		args = append(args, "-w", step.Workdir)
	}
	args = append(args, step.Image)
	if step.Cmd != "" {
		args = append(args, "sh", "-c", step.Cmd)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr, cmd.Dir = os.Stdout, os.Stderr, d.workflowDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker run failed: %w", err)
	}
	return map[string]string{}, nil
}

func (d *DockerRuntime) Build(ctx context.Context, step *schema.BuildStep) (map[string]string, error) {
	contextPath := step.Build.Context
	if contextPath == "" {
		contextPath = d.workflowDir
	}
	args := []string{"build", "-t", step.Build.Image}
	if step.Build.Dockerfile != "" {
		args = append(args, "-f", step.Build.Dockerfile)
	}
	args = append(args, contextPath)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr, cmd.Dir = os.Stdout, os.Stderr, d.workflowDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}
	return map[string]string{}, nil
}
