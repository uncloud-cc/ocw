// Package security provides jailbreak prevention and security hardening for OCW containers
package security

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed seccomp-ocw.json
var seccompProfile []byte

var seccompPath string

// SetupSeccompProfile writes the seccomp profile to a temp location and returns the path
func SetupSeccompProfile() (string, error) {
	if seccompPath != "" {
		return seccompPath, nil
	}

	// Write seccomp profile to temp directory
	tmpDir := os.TempDir()
	seccompPath = filepath.Join(tmpDir, "ocw-seccomp.json")

	if err := os.WriteFile(seccompPath, seccompProfile, 0644); err != nil {
		return "", fmt.Errorf("failed to write seccomp profile: %w", err)
	}

	return seccompPath, nil
}

// CleanupSeccompProfile removes the temporary seccomp profile
func CleanupSeccompProfile() {
	if seccompPath != "" {
		os.Remove(seccompPath)
		seccompPath = ""
	}
}

// GetSeccompProfilePath returns the path to the seccomp profile
// Call SetupSeccompProfile first to ensure it exists
func GetSeccompProfilePath() string {
	if seccompPath != "" {
		return seccompPath
	}
	return filepath.Join(os.TempDir(), "ocw-seccomp.json")
}
