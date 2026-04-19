package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPullOptions(t *testing.T) {
	opts := PullOptions{
		Platform: "linux/amd64",
		Quiet:    true,
	}

	assert.Equal(t, "linux/amd64", opts.Platform)
	assert.True(t, opts.Quiet)
}

func TestCreateOptions(t *testing.T) {
	opts := CreateOptions{
		Image:      "alpine:latest",
		Name:       "test-container",
		Cmd:        []string{"echo", "hello"},
		Entrypoint: []string{"/bin/sh"},
		Env:        map[string]string{"KEY": "value"},
		WorkingDir: "/app",
		Mounts: []Mount{
			{Type: string(MountTypeBind), Source: "/host", Target: "/container", ReadOnly: true},
		},
		Ports: []PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: string(ProtocolTCP)},
		},
		Network:     NetworkID("mynetwork"),
		NetworkMode: "bridge",
		CPUs:        2.0,
		Memory:      1024 * 1024 * 512, // 512MB
		GPUs:        "all",
		HealthCheck: &HealthCheckConfig{
			Test:        []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
			Interval:    30 * time.Second,
			Timeout:     10 * time.Second,
			Retries:     3,
			StartPeriod: 5 * time.Second,
		},
		TTY:       true,
		OpenStdin: true,
		Labels:    map[string]string{"app": "test"},
	}

	assert.Equal(t, "alpine:latest", opts.Image)
	assert.Equal(t, "test-container", opts.Name)
	assert.Equal(t, []string{"echo", "hello"}, opts.Cmd)
	assert.Equal(t, []string{"/bin/sh"}, opts.Entrypoint)
	assert.Equal(t, "/app", opts.WorkingDir)
	assert.Equal(t, 2.0, opts.CPUs)
	assert.Equal(t, int64(1024*1024*512), opts.Memory)
	assert.Equal(t, "all", opts.GPUs)
	assert.True(t, opts.TTY)
	assert.True(t, opts.OpenStdin)
}

func TestMount(t *testing.T) {
	mount := Mount{
		Type:     string(MountTypeVolume),
		Source:   "myvolume",
		Target:   "/data",
		ReadOnly: false,
	}

	assert.Equal(t, string(MountTypeVolume), mount.Type)
	assert.Equal(t, "myvolume", mount.Source)
	assert.Equal(t, "/data", mount.Target)
	assert.False(t, mount.ReadOnly)
}

func TestPortMapping(t *testing.T) {
	tests := []struct {
		name          string
		mapping       PortMapping
		containerPort int
		hostPort      int
		protocol      string
	}{
		{
			name: "tcp explicit",
			mapping: PortMapping{
				ContainerPort: 80,
				HostPort:      8080,
				Protocol:      string(ProtocolTCP),
			},
			containerPort: 80,
			hostPort:      8080,
			protocol:      string(ProtocolTCP),
		},
		{
			name: "udp",
			mapping: PortMapping{
				ContainerPort: 53,
				HostPort:      53,
				Protocol:      string(ProtocolUDP),
			},
			containerPort: 53,
			hostPort:      53,
			protocol:      string(ProtocolUDP),
		},
		{
			name: "auto assign host port",
			mapping: PortMapping{
				ContainerPort: 3000,
				HostPort:      0,
				Protocol:      string(ProtocolTCP),
			},
			containerPort: 3000,
			hostPort:      0,
			protocol:      string(ProtocolTCP),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.containerPort, tt.mapping.ContainerPort)
			assert.Equal(t, tt.hostPort, tt.mapping.HostPort)
			assert.Equal(t, tt.protocol, tt.mapping.Protocol)
		})
	}
}

func TestHealthCheckConfig(t *testing.T) {
	hc := HealthCheckConfig{
		Test:        []string{"CMD", "curl", "-f", "http://localhost"},
		Interval:    30 * time.Second,
		Timeout:     5 * time.Second,
		Retries:     3,
		StartPeriod: 10 * time.Second,
	}

	assert.Equal(t, []string{"CMD", "curl", "-f", "http://localhost"}, hc.Test)
	assert.Equal(t, 30*time.Second, hc.Interval)
	assert.Equal(t, 5*time.Second, hc.Timeout)
	assert.Equal(t, 3, hc.Retries)
	assert.Equal(t, 10*time.Second, hc.StartPeriod)
}

func TestBuildOptions(t *testing.T) {
	opts := BuildOptions{
		ContextPath: "/tmp/build",
		Dockerfile:  "Dockerfile.prod",
		Tags:        []string{"myapp:latest", "myapp:v1.0"},
		BuildArgs:   map[string]string{"VERSION": "1.0"},
		Target:      "production",
		Platform:    []string{"linux/amd64", "linux/arm64"},
		CacheFrom:   []string{"type=local,src=/cache"},
		CacheTo:     []string{"type=local,dest=/cache"},
		NoCache:     true,
		Pull:        true,
		Push:        true,
		Load:        true,
		Labels:      map[string]string{"org.opencontainers.image.source": "github.com/org/repo"},
		Secrets: []BuildSecret{
			{ID: "npmrc", Source: "/root/.npmrc", IsEnv: false},
			{ID: "token", Source: "GITHUB_TOKEN", IsEnv: true},
		},
	}

	assert.Equal(t, "/tmp/build", opts.ContextPath)
	assert.Equal(t, "Dockerfile.prod", opts.Dockerfile)
	assert.Equal(t, []string{"myapp:latest", "myapp:v1.0"}, opts.Tags)
	assert.Equal(t, "production", opts.Target)
	assert.True(t, opts.NoCache)
	assert.True(t, opts.Pull)
	assert.True(t, opts.Push)
	assert.True(t, opts.Load)
	assert.Len(t, opts.Secrets, 2)
	assert.False(t, opts.Secrets[0].IsEnv)
	assert.True(t, opts.Secrets[1].IsEnv)
}

func TestBuildSecret(t *testing.T) {
	fileSecret := BuildSecret{
		ID:     "ssh-key",
		Source: "/root/.ssh/id_rsa",
		IsEnv:  false,
	}

	envSecret := BuildSecret{
		ID:     "api-key",
		Source: "API_KEY",
		IsEnv:  true,
	}

	assert.Equal(t, "ssh-key", fileSecret.ID)
	assert.Equal(t, "/root/.ssh/id_rsa", fileSecret.Source)
	assert.False(t, fileSecret.IsEnv)

	assert.Equal(t, "api-key", envSecret.ID)
	assert.Equal(t, "API_KEY", envSecret.Source)
	assert.True(t, envSecret.IsEnv)
}

func TestNetworkOptions(t *testing.T) {
	opts := NetworkOptions{
		Driver:   string(NetworkDriverBridge),
		Labels:   map[string]string{"project": "test"},
		Internal: true,
	}

	assert.Equal(t, string(NetworkDriverBridge), opts.Driver)
	assert.True(t, opts.Internal)
	assert.Equal(t, "test", opts.Labels["project"])
}

func TestLogOptions(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	opts := LogOptions{
		Follow:     true,
		Timestamps: true,
		Since:      since,
		Until:      until,
		Tail:       "100",
	}

	assert.True(t, opts.Follow)
	assert.True(t, opts.Timestamps)
	assert.Equal(t, since, opts.Since)
	assert.Equal(t, until, opts.Until)
	assert.Equal(t, "100", opts.Tail)
}

func TestAttachOptions(t *testing.T) {
	opts := AttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: false,
		Stream: true,
	}

	assert.True(t, opts.Stdin)
	assert.True(t, opts.Stdout)
	assert.False(t, opts.Stderr)
	assert.True(t, opts.Stream)
}

func TestMountTypeConstants(t *testing.T) {
	assert.Equal(t, MountType("bind"), MountTypeBind)
	assert.Equal(t, MountType("volume"), MountTypeVolume)
	assert.Equal(t, MountType("tmpfs"), MountTypeTmpfs)
}

func TestNetworkDriverConstants(t *testing.T) {
	assert.Equal(t, NetworkDriver("bridge"), NetworkDriverBridge)
	assert.Equal(t, NetworkDriver("host"), NetworkDriverHost)
	assert.Equal(t, NetworkDriver("none"), NetworkDriverNone)
	assert.Equal(t, NetworkDriver("overlay"), NetworkDriverOverlay)
}
