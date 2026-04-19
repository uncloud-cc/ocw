package mock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uncloud-cc/ocw/pkg/container"
)

func TestRuntimePull(t *testing.T) {
	r := &Runtime{}

	// Test default behavior (no custom function)
	err := r.Pull(context.Background(), "alpine:latest", container.PullOptions{})
	assert.NoError(t, err)
	assert.Len(t, r.PullCalls, 1)
	assert.Equal(t, "alpine:latest", r.PullCalls[0].Image)

	// Test with custom function
	customCalled := false
	r2 := &Runtime{
		PullFunc: func(ctx context.Context, image string, opts container.PullOptions) error {
			customCalled = true
			assert.Equal(t, "nginx", image)
			return nil
		},
	}

	err = r2.Pull(context.Background(), "nginx", container.PullOptions{Quiet: true})
	assert.NoError(t, err)
	assert.True(t, customCalled)
}

func TestRuntimeBuild(t *testing.T) {
	r := &Runtime{}

	// Test default behavior
	imageID, err := r.Build(context.Background(), container.BuildOptions{
		ContextPath: "/tmp/build",
		Dockerfile:  "Dockerfile",
	})
	assert.NoError(t, err)
	assert.Equal(t, container.ImageID("mock-image-id"), imageID)
	assert.Len(t, r.BuildCalls, 1)
	assert.Equal(t, "/tmp/build", r.BuildCalls[0].Opts.ContextPath)
}

func TestRuntimeImageExists(t *testing.T) {
	r := &Runtime{}

	exists, err := r.ImageExists(context.Background(), "alpine:latest")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Len(t, r.ImageExistsCalls, 1)
}

func TestRuntimeContainerLifecycle(t *testing.T) {
	r := &Runtime{}
	ctx := context.Background()

	// Create
	containerID, err := r.Create(ctx, container.CreateOptions{
		Image: "alpine:latest",
		Cmd:   []string{"echo", "hello"},
	})
	assert.NoError(t, err)
	assert.Equal(t, container.ContainerID("mock-container-id"), containerID)
	assert.Len(t, r.CreateCalls, 1)

	// Start
	err = r.Start(ctx, containerID)
	assert.NoError(t, err)
	assert.Len(t, r.StartCalls, 1)
	assert.Equal(t, containerID, r.StartCalls[0].ID)

	// Wait
	result, err := r.Wait(ctx, containerID)
	assert.NoError(t, err)
	assert.Equal(t, 0, result.StatusCode)
	assert.Len(t, r.WaitCalls, 1)

	// Stop
	err = r.Stop(ctx, containerID, 30*time.Second)
	assert.NoError(t, err)
	assert.Len(t, r.StopCalls, 1)
	assert.Equal(t, 30*time.Second, r.StopCalls[0].Timeout)

	// Remove
	err = r.Remove(ctx, containerID, false)
	assert.NoError(t, err)
	assert.Len(t, r.RemoveCalls, 1)
	assert.False(t, r.RemoveCalls[0].Force)
}

func TestRuntimeInspect(t *testing.T) {
	r := &Runtime{}

	info, err := r.Inspect(context.Background(), container.ContainerID("abc123"))
	assert.NoError(t, err)
	assert.Equal(t, container.ContainerID("abc123"), info.ID)
}

func TestRuntimeExec(t *testing.T) {
	r := &Runtime{}

	result, err := r.Exec(context.Background(), container.ContainerID("abc123"), []string{"ls", "-la"})
	assert.NoError(t, err)
	assert.Equal(t, 0, result.StatusCode)
	assert.Len(t, r.ExecCalls, 1)
	assert.Equal(t, []string{"ls", "-la"}, r.ExecCalls[0].Cmd)
}

func TestRuntimeVolumes(t *testing.T) {
	r := &Runtime{}

	volID, err := r.CreateVolume(context.Background(), "myvolume")
	assert.NoError(t, err)
	assert.Equal(t, container.VolumeID("myvolume"), volID)
	assert.Len(t, r.CreateVolumeCalls, 1)
	assert.Equal(t, "myvolume", r.CreateVolumeCalls[0].Name)

	err = r.RemoveVolume(context.Background(), volID)
	assert.NoError(t, err)
	assert.Len(t, r.RemoveVolumeCalls, 1)
}

func TestRuntimeNetworks(t *testing.T) {
	r := &Runtime{}

	netID, err := r.CreateNetwork(context.Background(), "mynetwork", container.NetworkOptions{
		Driver: "bridge",
	})
	assert.NoError(t, err)
	assert.Equal(t, container.NetworkID("mynetwork"), netID)
	assert.Len(t, r.CreateNetworkCalls, 1)
	assert.Equal(t, "bridge", r.CreateNetworkCalls[0].Opts.Driver)

	containerID := container.ContainerID("abc123")
	err = r.ConnectNetwork(context.Background(), netID, containerID)
	assert.NoError(t, err)
	assert.Len(t, r.ConnectNetworkCalls, 1)

	err = r.RemoveNetwork(context.Background(), netID)
	assert.NoError(t, err)
	assert.Len(t, r.RemoveNetworkCalls, 1)
}

func TestRuntimeKill(t *testing.T) {
	r := &Runtime{}

	err := r.Kill(context.Background(), container.ContainerID("abc123"), "SIGTERM")
	assert.NoError(t, err)
	assert.Len(t, r.KillCalls, 1)
	assert.Equal(t, "SIGTERM", r.KillCalls[0].Signal)
}

func TestRuntimeImplementsInterface(t *testing.T) {
	// This test verifies at compile time that Runtime implements container.Runtime
	var _ container.Runtime = (*Runtime)(nil)
}
