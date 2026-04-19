package container

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrImageNotFound", ErrImageNotFound, "image not found"},
		{"ErrContainerNotFound", ErrContainerNotFound, "container not found"},
		{"ErrContainerNotRunning", ErrContainerNotRunning, "container is not running"},
		{"ErrBuildFailed", ErrBuildFailed, "build failed"},
		{"ErrNetworkNotFound", ErrNetworkNotFound, "network not found"},
		{"ErrVolumeNotFound", ErrVolumeNotFound, "volume not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestContainerError(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &ContainerError{
		ContainerID: ContainerID("abc123"),
		Operation:   "start",
		Err:         underlying,
	}

	wantMsg := "start container abc123: connection refused"
	assert.Equal(t, wantMsg, err.Error())

	// Test unwrapping
	unwrapped := errors.Unwrap(err)
	assert.Equal(t, underlying, unwrapped)
}

func TestContainerErrorWithoutUnderlying(t *testing.T) {
	err := &ContainerError{
		ContainerID: ContainerID("xyz789"),
		Operation:   "create",
		Err:         nil,
	}

	wantMsg := "create container xyz789: <nil>"
	assert.Equal(t, wantMsg, err.Error())
}

func TestErrorIs(t *testing.T) {
	// Test that typed errors work with errors.Is
	assert.True(t, errors.Is(ErrImageNotFound, ErrImageNotFound))
	assert.True(t, errors.Is(ErrContainerNotFound, ErrContainerNotFound))

	// Test wrapped error
	wrapped := &ContainerError{
		ContainerID: ContainerID("test"),
		Operation:   "pull",
		Err:         ErrImageNotFound,
	}

	assert.True(t, errors.Is(wrapped, ErrImageNotFound))
}
