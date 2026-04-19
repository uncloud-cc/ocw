package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerID(t *testing.T) {
	id := ContainerID("abc123")
	assert.Equal(t, ContainerID("abc123"), id)
	assert.Equal(t, "abc123", string(id))
}

func TestImageID(t *testing.T) {
	id := ImageID("myimage:latest")
	assert.Equal(t, ImageID("myimage:latest"), id)
	assert.Equal(t, "myimage:latest", string(id))
}

func TestVolumeID(t *testing.T) {
	id := VolumeID("myvolume")
	assert.Equal(t, VolumeID("myvolume"), id)
	assert.Equal(t, "myvolume", string(id))
}

func TestNetworkID(t *testing.T) {
	id := NetworkID("mynetwork")
	assert.Equal(t, NetworkID("mynetwork"), id)
	assert.Equal(t, "mynetwork", string(id))
}

func TestExitResult(t *testing.T) {
	tests := []struct {
		name      string
		result    ExitResult
		wantCode  int
		wantOOM   bool
		wantError string
	}{
		{
			name:      "success",
			result:    ExitResult{StatusCode: 0, OOMKilled: false, Error: ""},
			wantCode:  0,
			wantOOM:   false,
			wantError: "",
		},
		{
			name:      "failure",
			result:    ExitResult{StatusCode: 1, OOMKilled: false, Error: "exit error"},
			wantCode:  1,
			wantOOM:   false,
			wantError: "exit error",
		},
		{
			name:      "oom killed",
			result:    ExitResult{StatusCode: 137, OOMKilled: true, Error: ""},
			wantCode:  137,
			wantOOM:   true,
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.result.StatusCode)
			assert.Equal(t, tt.wantOOM, tt.result.OOMKilled)
			assert.Equal(t, tt.wantError, tt.result.Error)
		})
	}
}

func TestContainerInfo(t *testing.T) {
	info := ContainerInfo{
		ID:     ContainerID("abc123"),
		Name:   "test-container",
		Image:  "alpine:latest",
		Status: StatusRunning,
		Health: HealthHealthy,
		Ports: []PortBinding{
			{ContainerPort: 80, HostPort: 8080, Protocol: ProtocolTCP},
			{ContainerPort: 443, HostPort: 8443, Protocol: ProtocolTCP},
		},
	}

	assert.Equal(t, ContainerID("abc123"), info.ID)
	assert.Equal(t, "test-container", info.Name)
	assert.Equal(t, "alpine:latest", info.Image)
	assert.Equal(t, StatusRunning, info.Status)
	assert.Equal(t, HealthHealthy, info.Health)
	assert.Len(t, info.Ports, 2)
	assert.Equal(t, 80, info.Ports[0].ContainerPort)
	assert.Equal(t, 8080, info.Ports[0].HostPort)
	assert.Equal(t, ProtocolTCP, info.Ports[0].Protocol)
}

func TestPortBinding(t *testing.T) {
	binding := PortBinding{
		ContainerPort: 3000,
		HostPort:      3000,
		Protocol:      ProtocolTCP,
	}

	assert.Equal(t, 3000, binding.ContainerPort)
	assert.Equal(t, 3000, binding.HostPort)
	assert.Equal(t, ProtocolTCP, binding.Protocol)
}
