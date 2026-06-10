package ocw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestBuildRunArgs(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		StepBase: schema.StepBase{Env: map[string]string{"FOO": "bar"}},
		Image:    "alpine",
		Cmd:      "echo hello",
		Workdir:  "/app",
	}

	args := d.buildRunArgs(step, "/host/outputs")

	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "ocw-testnet")
	assert.Contains(t, args, "-v")
	assert.Contains(t, args, "/tmp/wf:/workflow")
	assert.Contains(t, args, "-w")
	assert.Contains(t, args, "/app")
	assert.Contains(t, args, "-e")
	assert.Contains(t, args, "FOO=bar")
	assert.Contains(t, args, "alpine")
	assert.Contains(t, args, "echo hello")
}

func TestBuildServiceArgs_Basic(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		Image:      "node:25-alpine",
		Cmd:        "npx serve",
		Background: true,
	}

	args := d.buildServiceArgs(step)

	assert.Contains(t, args, "run")
	assert.Contains(t, args, "-d")
	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "ocw-testnet")
	assert.Contains(t, args, "node:25-alpine")
	assert.Contains(t, args, "npx serve")
	assert.NotContains(t, args, "--rm")
}

func TestBuildServiceArgs_WithExpose(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		Image:      "node:25-alpine",
		Background: true,
		Expose: &schema.Expose{
			Ports: []schema.ExposePort{
				{ContainerPort: 8080, HostPort: 8080, Protocol: "http"},
				{ContainerPort: 3000, HostPort: 3000, Protocol: "tcp"},
			},
		},
	}

	args := d.buildServiceArgs(step)

	assert.Contains(t, args, "-p")
	idx := indexOf(args, "-p")
	assert.Equal(t, "8080:8080/tcp", args[idx+1])
	// second -p should be after the first one
	second := indexOf(args[idx+2:], "-p")
	assert.GreaterOrEqual(t, second, 0)
	assert.Equal(t, "3000:3000/tcp", args[idx+2+second+1])
}

func TestBuildServiceArgs_WithHealthCheck(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		Image:      "node:25-alpine",
		Background: true,
		HealthCheck: &schema.HealthCheck{
			Cmd:         "curl -f http://localhost:8080/health",
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     3,
			StartPeriod: "30s",
		},
	}

	args := d.buildServiceArgs(step)

	assert.Contains(t, args, "--health-cmd")
	assert.Contains(t, args, "curl -f http://localhost:8080/health")
	assert.Contains(t, args, "--health-interval")
	assert.Contains(t, args, "10s")
	assert.Contains(t, args, "--health-timeout")
	assert.Contains(t, args, "5s")
	assert.Contains(t, args, "--health-retries")
	assert.Contains(t, args, "3")
	assert.Contains(t, args, "--health-start-period")
	assert.Contains(t, args, "30s")
}

func TestBuildServiceArgs_Full(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		StepBase:   schema.StepBase{ID: "web", Env: map[string]string{"NODE_ENV": "production"}},
		Image:      "node:25-alpine",
		Background: true,
		Workdir:    "/app",
		Expose: &schema.Expose{
			Ports: []schema.ExposePort{
				{ContainerPort: 8080, HostPort: 8080, Protocol: "https"},
			},
		},
		HealthCheck: &schema.HealthCheck{
			Cmd:      "wget -qO- http://localhost:8080",
			Interval: "5s",
		},
	}

	args := d.buildServiceArgs(step)

	assert.Contains(t, args, "run")
	assert.Contains(t, args, "-d")
	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "ocw-testnet")
	assert.Contains(t, args, "--network-alias")
	assert.Contains(t, args, "web")
	assert.Contains(t, args, "-w")
	assert.Contains(t, args, "/app")
	assert.Contains(t, args, "-e")
	assert.Contains(t, args, "NODE_ENV=production")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "8080:8080/tcp") // https maps to tcp
	assert.Contains(t, args, "--health-cmd")
	assert.Contains(t, args, "wget -qO- http://localhost:8080")
	assert.Contains(t, args, "--health-interval")
	assert.Contains(t, args, "5s")
}

func TestBuildServiceArgs_HealthCheckDefaults(t *testing.T) {
	d := &DockerRuntime{workflowDir: "/tmp/wf", networkName: "ocw-testnet", printer: NewPrinter(nil, false, false, nil)}
	step := &schema.RunStep{
		Image:      "node:25-alpine",
		Background: true,
		HealthCheck: &schema.HealthCheck{
			Cmd: "redis-cli ping",
		},
	}

	args := d.buildServiceArgs(step)

	assert.Contains(t, args, "--health-cmd")
	assert.Contains(t, args, "redis-cli ping")
	assert.Contains(t, args, "--health-interval")
	assert.Contains(t, args, "500ms")
	assert.Contains(t, args, "--health-timeout")
	assert.Contains(t, args, "1s")
	assert.Contains(t, args, "--health-retries")
	assert.Contains(t, args, "10")
	assert.Contains(t, args, "--health-start-period")
	assert.Contains(t, args, "0s")
}

func TestBuildVolumeMount(t *testing.T) {
	d := &DockerRuntime{
		workflowDir: "/tmp/wf",
		volumes: schema.Volumes{
			"site":    {Path: "./website/site", Mode: schema.VolumeModeReadWrite, MountPath: "/src/site"},
			"secrets": {Path: "/home/user/.aws", Mode: schema.VolumeModeReadOnly},
		},
		printer: NewPrinter(nil, false, false, nil),
	}

	cases := []struct {
		name     string
		ref      schema.VolumeRef
		expected string
	}{
		{
			name:     "simple volume with defaults",
			ref:      schema.VolumeRef{Name: "site"},
			expected: "/tmp/wf/website/site:/src/site",
		},
		{
			name:     "absolute path volume with default mountPath",
			ref:      schema.VolumeRef{Name: "secrets"},
			expected: "/home/user/.aws:/volumes/secrets:ro",
		},
		{
			name:     "override mount path",
			ref:      schema.VolumeRef{Name: "site", MountPath: "/custom/path"},
			expected: "/tmp/wf/website/site:/custom/path",
		},
		{
			name:     "force read-only on readwrite volume",
			ref:      schema.VolumeRef{Name: "site", ReadOnly: ptrBool(true)},
			expected: "/tmp/wf/website/site:/src/site:ro",
		},
		{
			name:     "force readwrite on readonly volume (should error in validation, but runtime allows)",
			ref:      schema.VolumeRef{Name: "secrets", ReadOnly: ptrBool(false)},
			expected: "/home/user/.aws:/volumes/secrets",
		},
		{
			name:     "unknown volume",
			ref:      schema.VolumeRef{Name: "missing"},
			expected: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := d.buildVolumeMount(tt.ref)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func ptrBool(b bool) *bool {
	return &b
}

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}
