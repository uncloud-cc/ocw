package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ResolvedVolume contains the resolved paths for a workflow volume
type ResolvedVolume struct {
	Name      string
	HostPath  string            // Absolute path on host
	Mode      schema.VolumeMode // "ro" or "rw"
	MountPath string            // Default mount path (empty = /volumes/<name>)
}

// VolumeMount represents a resolved volume mount
type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// resolveVolumes resolves volume paths from the workflow schema
func (r *Runner) resolveVolumes(volumes schema.Volumes) error {
	r.resolvedVolumes = make(map[string]*ResolvedVolume)

	for name, vol := range volumes {
		hostPath := vol.Path

		// Expand ~ to home directory
		if strings.HasPrefix(hostPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("volume %q: failed to get home directory: %w", name, err)
			}
			hostPath = filepath.Join(homeDir, hostPath[2:])
		}

		if !filepath.IsAbs(hostPath) {
			hostPath = filepath.Join(r.WorkflowDir, vol.Path)
		}

		absPath, err := filepath.Abs(hostPath)
		if err != nil {
			return fmt.Errorf("volume %q: %w", name, err)
		}

		// Verify path exists
		if _, err := os.Stat(absPath); err != nil {
			return fmt.Errorf("volume %q path does not exist: %s", name, absPath)
		}

		mode := vol.Mode
		if mode == "" {
			mode = schema.VolumeModeReadOnly
		}

		r.resolvedVolumes[name] = &ResolvedVolume{
			Name:      name,
			HostPath:  absPath,
			Mode:      mode,
			MountPath: vol.MountPath,
		}
	}

	return nil
}

// resolveStepVolumes resolves volume references for a step
// Returns a list of mount specifications for docker
func (r *Runner) resolveStepVolumes(stepVolumes, jobVolumes schema.VolumeRefs) ([]VolumeMount, error) {
	var mounts []VolumeMount
	seen := make(map[string]bool)

	// Process step-level volumes (highest priority)
	for _, ref := range stepVolumes {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true

		mount, err := r.resolveVolumeRef(ref)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}

	// Process job-level volumes
	for _, ref := range jobVolumes {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true

		mount, err := r.resolveVolumeRef(ref)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}

	return mounts, nil
}

// resolveVolumeRef resolves a single volume reference
func (r *Runner) resolveVolumeRef(ref schema.VolumeRef) (VolumeMount, error) {
	vol, ok := r.resolvedVolumes[ref.Name]
	if !ok {
		return VolumeMount{}, fmt.Errorf("volume %q not defined", ref.Name)
	}

	// Determine mount path (ref override > volume default > /volumes/<name>)
	mountPath := ref.MountPath
	if mountPath == "" {
		mountPath = vol.MountPath
	}
	if mountPath == "" {
		mountPath = "/volumes/" + ref.Name
	}

	// Determine if mount should be read-only
	// Start with volume's mode
	readOnly := vol.Mode == schema.VolumeModeReadOnly || vol.Mode == ""

	// ref.ReadOnly can only make it MORE restrictive (rw -> ro)
	// It cannot make a ro volume writable
	if ref.ReadOnly != nil {
		if *ref.ReadOnly {
			// Always allowed: making mount read-only
			readOnly = true
		} else if readOnly {
			// ERROR: Cannot make a read-only volume writable
			return VolumeMount{}, fmt.Errorf(
				"volume %q is read-only and cannot be mounted as read-write; "+
					"steps can only make volumes more restrictive, not less",
				ref.Name,
			)
		}
		// If ref.ReadOnly is false and volume is rw, readOnly stays false
	}

	return VolumeMount{
		HostPath:      vol.HostPath,
		ContainerPath: mountPath,
		ReadOnly:      readOnly,
	}, nil
}
