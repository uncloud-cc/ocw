package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// cleanupBackgroundContainers stops and removes all background containers
func (r *Runner) cleanupBackgroundContainers() {
	// Stop reloader first (stops file watchers and pending reloads)
	if r.reloader != nil {
		r.reloader.Stop()
	}

	r.backgroundMu.Lock()
	containers := make([]string, len(r.backgroundContainers))
	copy(containers, r.backgroundContainers)
	r.backgroundContainers = r.backgroundContainers[:0]
	r.backgroundMu.Unlock()

	if len(containers) == 0 {
		// Still clean up network even if no containers
		r.cleanupNetwork()
		return
	}

	r.Output("\n%s\n", r.styles.Dim(fmt.Sprintf("Cleaning up %d background container(s)...", len(containers))))
	ctx := context.Background()
	for _, name := range containers {
		if err := r.docker.StopContainer(ctx, name); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to stop %s: %v", name, err)))
		}
		if err := r.docker.RemoveContainer(ctx, name); err != nil {
			r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to remove %s: %v", name, err)))
		}
	}

	// Clean up the network after all containers are removed
	r.cleanupNetwork()
}

// cleanupNetwork removes the job network
func (r *Runner) cleanupNetwork() {
	if r.networkName == "" {
		return
	}
	// Network cleanup is silent - only show errors
	if err := r.docker.RemoveNetwork(context.Background(), r.networkName); err != nil {
		r.Output("  %s\n", r.styles.Warning(fmt.Sprintf("Warning: failed to remove network: %v", err)))
	}
	r.networkName = ""
}

// waitForInterrupt waits for SIGINT or SIGTERM, keeping background containers running
func (r *Runner) waitForInterrupt() {
	r.Output("\n%s\n", r.styles.Info("Background services running. Press Ctrl+C to stop..."))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	r.Output("\n%s\n", r.styles.Dim("Shutting down..."))
}

// hasBackgroundContainers returns true if there are background containers running
func (r *Runner) hasBackgroundContainers() bool {
	r.backgroundMu.Lock()
	defer r.backgroundMu.Unlock()
	return len(r.backgroundContainers) > 0
}
