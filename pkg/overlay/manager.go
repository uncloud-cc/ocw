// Package overlay provides immutable filesystem support for workflow containers
// using Podman's overlay volume feature. This eliminates the need for FUSE on
// the host machine by leveraging kernel overlayfs support inside the Podman VM.
package overlay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles overlay volume lifecycle for a workflow run
type Manager struct {
	RunID          string
	BaseDir        string   // Base directory in VM: /tmp/ocw/<run-id>
	SourceDir      string   // Host workflow dir mounted in VM
	CompletedSteps []string // List of completed step overlay dirs (for lowerdir)
	CurrentStep    string   // Current step ID
	inVM           bool     // Whether we're using Podman Machine (vs native Linux)
}

// NewManager creates a new overlay manager for a workflow run
func NewManager(runID, workflowDir string) (*Manager, error) {
	m := &Manager{
		RunID:          runID,
		BaseDir:        fmt.Sprintf("/tmp/ocw/%s", runID),
		CompletedSteps: []string{},
	}

	// Detect if we're using Podman Machine
	m.inVM = isPodmanMachine()

	// Initialize directory structure in VM/host
	if err := m.initDirectories(workflowDir); err != nil {
		return nil, err
	}

	return m, nil
}

// isPodmanMachine checks if Podman is running via a VM
func isPodmanMachine() bool {
	cmd := exec.Command("podman", "machine", "list", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return false // Assume native Linux if command fails
	}
	return strings.TrimSpace(string(output)) != ""
}

// initDirectories creates the overlay directory structure
func (m *Manager) initDirectories(workflowDir string) error {
	if m.inVM {
		// Podman Machine: directories are created inside the VM
		// Host paths are auto-mapped by Podman Machine for paths under $HOME
		m.SourceDir = workflowDir
		return m.execInVM(fmt.Sprintf("mkdir -p %s", m.BaseDir))
	}

	// Native Linux: directories are created directly on host
	m.SourceDir = workflowDir
	return os.MkdirAll(m.BaseDir, 0755)
}

// execInVM executes commands inside the Podman VM (or directly on Linux)
func (m *Manager) execInVM(cmds ...string) error {
	for _, c := range cmds {
		var cmd *exec.Cmd
		if m.inVM {
			cmd = exec.Command("podman", "machine", "ssh", "--", c)
		} else {
			cmd = exec.Command("sh", "-c", c)
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("exec failed: %s: %w\nOutput: %s", c, err, output)
		}
	}
	return nil
}

// PrepareStepOverlay creates overlay volume for the next step
// Returns the volume name to use with podman run -v
func (m *Manager) PrepareStepOverlay(stepID string) (string, error) {
	// Clean up previous step's volume (if any) - the upperdir persists
	if m.CurrentStep != "" {
		oldVolume := fmt.Sprintf("ocw-%s-%s", m.RunID, m.CurrentStep)
		exec.Command("podman", "volume", "rm", "-f", oldVolume).Run()
	}

	m.CurrentStep = stepID

	upperDir := filepath.Join(m.BaseDir, fmt.Sprintf("step-%s-upper", stepID))
	workDir := filepath.Join(m.BaseDir, fmt.Sprintf("step-%s-work", stepID))

	// Create directories in VM
	if err := m.execInVM(
		fmt.Sprintf("mkdir -p %s %s", upperDir, workDir),
		fmt.Sprintf("chmod 777 %s %s", upperDir, workDir),
		// Create .ocw-outputs directory for step outputs
		fmt.Sprintf("mkdir -p %s", filepath.Join(upperDir, ".ocw-outputs")),
	); err != nil {
		return "", fmt.Errorf("failed to create step overlay dirs: %w", err)
	}

	// Build lowerdir string: newest completed steps first, source last
	// Format: step3:step2:step1:source
	lowerdirs := make([]string, 0, len(m.CompletedSteps)+1)
	for i := len(m.CompletedSteps) - 1; i >= 0; i-- {
		lowerdirs = append(lowerdirs, m.CompletedSteps[i])
	}
	lowerdirs = append(lowerdirs, m.SourceDir)
	lowerdir := strings.Join(lowerdirs, ":")

	// Create podman volume with overlay options
	volumeName := fmt.Sprintf("ocw-%s-%s", m.RunID, stepID)

	createCmd := exec.Command("podman", "volume", "create",
		"--driver", "local",
		"--opt", "type=overlay",
		"--opt", "device=overlay",
		"--opt", fmt.Sprintf("o=lowerdir=%s,upperdir=%s,workdir=%s", lowerdir, upperDir, workDir),
		volumeName,
	)

	if output, err := createCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create overlay volume: %w\nOutput: %s", err, output)
	}

	return volumeName, nil
}

// CompleteStep marks current step as complete, adding its overlay to the stack
func (m *Manager) CompleteStep() error {
	if m.CurrentStep == "" {
		return nil
	}

	upperDir := filepath.Join(m.BaseDir, fmt.Sprintf("step-%s-upper", m.CurrentStep))
	m.CompletedSteps = append(m.CompletedSteps, upperDir)
	m.CurrentStep = ""

	return nil
}

// ReadStepOutput reads the contents of a step output file from the overlay
// The file path should be relative to /workflow (e.g., ".ocw-outputs/step-id")
func (m *Manager) ReadStepOutput(stepID string, filePath string) ([]byte, error) {
	// The output file is in the step's upperdir under .ocw-outputs/<step-id>
	// Since the step that wrote it is now in CompletedSteps, we need to find
	// which step's upperdir contains this file

	// Try the most recent completed steps first (reverse order)
	for i := len(m.CompletedSteps) - 1; i >= 0; i-- {
		upperDir := m.CompletedSteps[i]
		fullPath := filepath.Join(upperDir, filePath)

		// Try to read the file
		if m.inVM {
			// Use podman machine ssh to cat the file
			cmd := exec.Command("podman", "machine", "ssh", "--", fmt.Sprintf("cat %s 2>/dev/null || echo '__FILE_NOT_FOUND__'", fullPath))
			output, err := cmd.Output()
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(output)) != "__FILE_NOT_FOUND__" {
				return output, nil
			}
		} else {
			// Direct file access on native Linux
			content, err := os.ReadFile(fullPath)
			if err == nil {
				return content, nil
			}
		}
	}

	return nil, fmt.Errorf("output file not found: %s", filePath)
}

// Cleanup removes all overlay volumes and directories for this run
func (m *Manager) Cleanup() error {
	var errs []error

	// Remove all volumes for this run
	listCmd := exec.Command("podman", "volume", "ls", "--format", "{{.Name}}",
		"--filter", fmt.Sprintf("name=ocw-%s", m.RunID))
	output, _ := listCmd.Output()

	volumes := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, vol := range volumes {
		if vol != "" {
			if err := exec.Command("podman", "volume", "rm", "-f", vol).Run(); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove volume %s: %w", vol, err))
			}
		}
	}

	// Remove directories in VM
	if err := m.execInVM(fmt.Sprintf("rm -rf %s", m.BaseDir)); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}
