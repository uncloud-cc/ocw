package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerBasicOperations tests basic Docker operations that were previously untested
func TestDockerBasicOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	docker := NewDocker(
		func(format string, args ...any) {
			t.Logf(format, args...)
		},
		NewStyles(),
		nil,
	)

	t.Run("PullImage", func(t *testing.T) {
		err := docker.PullImage(ctx, "alpine:latest")
		require.NoError(t, err, "Should pull alpine:latest successfully")
	})

	t.Run("NetworkOperations", func(t *testing.T) {
		networkName := fmt.Sprintf("ocw-test-net-%d", time.Now().UnixNano())

		// Create network
		err := docker.CreateNetwork(ctx, NetworkCreateOptions{
			Name:   networkName,
			Driver: "bridge",
		})
		require.NoError(t, err, "Should create network successfully")

		// Verify network exists
		exists := docker.NetworkExists(ctx, networkName)
		assert.True(t, exists, "Network should exist after creation")

		// Cleanup
		err = docker.RemoveNetwork(ctx, networkName)
		require.NoError(t, err, "Should remove network successfully")

		// Verify network is gone
		exists = docker.NetworkExists(ctx, networkName)
		assert.False(t, exists, "Network should not exist after removal")
	})

	t.Run("RunSimpleContainer", func(t *testing.T) {
		containerName := fmt.Sprintf("ocw-test-%d", time.Now().UnixNano())

		err := docker.RunContainer(ctx, RunContainerOptions{
			Image: "alpine:latest",
			Name:  containerName,
			Cmd:   "echo 'hello world'",
		})
		require.NoError(t, err, "Should run container successfully")

		// Note: Container is auto-removed with --rm flag for non-background containers
		// So we don't check existence here
	})

	t.Run("RunBackgroundContainer", func(t *testing.T) {
		containerName := fmt.Sprintf("ocw-test-bg-%d", time.Now().UnixNano())

		err := docker.RunContainer(ctx, RunContainerOptions{
			Image:      "alpine:latest",
			Name:       containerName,
			Args:       []string{"sleep", "10"},
			Background: true,
		})
		require.NoError(t, err, "Should start background container")

		// Give it time to start
		time.Sleep(500 * time.Millisecond)

		// Verify it's running
		running := docker.IsContainerRunning(ctx, containerName)
		assert.True(t, running, "Container should be running")

		// Stop it
		err = docker.StopContainer(ctx, containerName)
		require.NoError(t, err, "Should stop container")

		// Verify it stopped
		running = docker.IsContainerRunning(ctx, containerName)
		assert.False(t, running, "Container should be stopped")

		// Cleanup
		_ = docker.RemoveContainer(ctx, containerName)
	})

	t.Run("BuildSimpleImage", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a simple Dockerfile
		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		dockerfile := `FROM alpine:latest
RUN echo "test build" > /test.txt
CMD ["cat", "/test.txt"]
`
		err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644)
		require.NoError(t, err)

		imageName := fmt.Sprintf("ocw-test-img:%d", time.Now().UnixNano())

		imageID, err := docker.BuildImage(ctx, BuildImageOptions{
			ImageName:  imageName,
			Context:    tmpDir,
			Dockerfile: dockerfilePath,
		})
		require.NoError(t, err, "Should build image successfully")
		assert.NotEmpty(t, imageID, "Image ID should not be empty")

		// Verify image exists
		exists := docker.ImageExists(ctx, imageName)
		assert.True(t, exists, "Image should exist after build")
	})
}

// TestDockerContainerLogs tests container log retrieval
func TestDockerContainerLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	docker := NewDocker(
		func(format string, args ...any) {
			t.Logf(format, args...)
		},
		NewStyles(),
		nil,
	)

	containerName := fmt.Sprintf("ocw-test-logs-%d", time.Now().UnixNano())
	testMessage := "unique test message 12345"

	// Run as background to prevent auto-removal
	err := docker.RunContainer(ctx, RunContainerOptions{
		Image:      "alpine:latest",
		Name:       containerName,
		Cmd:        fmt.Sprintf("sh -c 'echo \"%s\" && sleep 1'", testMessage),
		Background: true,
	})
	require.NoError(t, err)

	// Give it a moment to output
	time.Sleep(500 * time.Millisecond)

	// Get logs
	logs, err := docker.GetContainerLogs(ctx, containerName, 100)
	require.NoError(t, err, "Should get container logs")
	assert.Contains(t, logs, testMessage, "Logs should contain test message")

	// Cleanup
	_ = docker.StopContainer(ctx, containerName)
	_ = docker.RemoveContainer(ctx, containerName)
}

// TestDockerNetworkCommunication tests container-to-container communication
func TestDockerNetworkCommunication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	docker := NewDocker(
		func(format string, args ...any) {
			t.Logf(format, args...)
		},
		NewStyles(),
		nil,
	)

	networkName := fmt.Sprintf("ocw-test-comm-%d", time.Now().UnixNano())

	// Create network
	err := docker.CreateNetwork(ctx, NetworkCreateOptions{
		Name:   networkName,
		Driver: "bridge",
	})
	require.NoError(t, err)
	defer docker.RemoveNetwork(ctx, networkName)

	// Start first container
	container1 := fmt.Sprintf("server-%d", time.Now().UnixNano())
	err = docker.RunContainer(ctx, RunContainerOptions{
		Image:      "alpine:latest",
		Name:       container1,
		Args:       []string{"sleep", "20"},
		Network:    networkName,
		Background: true,
	})
	require.NoError(t, err)
	defer func() {
		_ = docker.StopContainer(ctx, container1)
		_ = docker.RemoveContainer(ctx, container1)
	}()

	// Give it time to start
	time.Sleep(500 * time.Millisecond)

	// Start second container that pings first
	container2 := fmt.Sprintf("client-%d", time.Now().UnixNano())
	err = docker.RunContainer(ctx, RunContainerOptions{
		Image:   "alpine:latest",
		Name:    container2,
		Cmd:     fmt.Sprintf("ping -c 1 %s", container1),
		Network: networkName,
	})

	// This should succeed if networking works
	assert.NoError(t, err, "Containers on same network should be able to communicate")

	// Cleanup
	_ = docker.RemoveContainer(ctx, container2)
}
