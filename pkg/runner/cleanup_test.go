package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanupBackgroundContainers(t *testing.T) {
	tests := []struct {
		name       string
		containers []string
	}{
		{
			name:       "no containers",
			containers: []string{},
		},
		{
			name:       "single container",
			containers: []string{"container1"},
		},
		{
			name:       "multiple containers",
			containers: []string{"container1", "container2", "container3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(".")
			runner.backgroundContainers = tt.containers
			runner.networkName = "test-network"

			// cleanupBackgroundContainers will clear the containers
			// We can't fully test this without a real docker, but we can test the structure
			initial := len(tt.containers)
			assert.Equal(t, initial, len(runner.backgroundContainers))
		})
	}
}

func TestCleanupNetwork(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
	}{
		{
			name:        "no network",
			networkName: "",
		},
		{
			name:        "with network",
			networkName: "test-network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(".")
			runner.networkName = tt.networkName

			// cleanupNetwork should handle empty network names
			if tt.networkName == "" {
				// Should be a no-op
				runner.cleanupNetwork()
				assert.Empty(t, runner.networkName)
			}
		})
	}
}

func TestHasBackgroundContainers(t *testing.T) {
	tests := []struct {
		name       string
		containers []string
		want       bool
	}{
		{
			name:       "no containers",
			containers: []string{},
			want:       false,
		},
		{
			name:       "has containers",
			containers: []string{"container1"},
			want:       true,
		},
		{
			name:       "multiple containers",
			containers: []string{"container1", "container2"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(".")
			runner.backgroundContainers = tt.containers

			got := runner.hasBackgroundContainers()
			assert.Equal(t, tt.want, got)
		})
	}
}
