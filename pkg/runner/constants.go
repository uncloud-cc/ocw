package runner

import "time"

const (
	// DefaultDebounceInterval is the default debounce duration for file watcher events
	DefaultDebounceInterval = 100 * time.Millisecond

	// ContainerStopGracePeriod is the time to wait for a container to stop gracefully
	ContainerStopGracePeriod = 500 * time.Millisecond

	// HealthCheckInterval is the interval between container health checks
	HealthCheckInterval = 100 * time.Millisecond

	// MaxHealthCheckAttempts is the maximum number of health check attempts
	MaxHealthCheckAttempts = 300
)
