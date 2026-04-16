package e2e_test

import (
	"testing"
)

func TestE2E_Env_Variables(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Environment Variables",
		Fixture:      "env",
		Args:         []string{"-f", "env.yaml"},
		WantExitCode: 0,
		WantStdout:   []string{"Port: 5432", "User: testuser"},
		SkipNoDocker: true,
	})
}

func TestE2E_Env_SecretsMasked(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:          "Secrets Masked",
		Fixture:       "env",
		Args:          []string{"-f", "secrets.yaml"},
		WantExitCode:  0,
		WantStdout:    []string{"User: testuser"},
		WantStdoutNot: []string{"secretpass123", "api-key-456"},
		SkipNoDocker:  true,
	})
}

func TestE2E_Env_SecretsShown(t *testing.T) {
	t.Parallel()
	runTest(t, TestCase{
		Name:         "Secrets Shown",
		Fixture:      "env",
		Args:         []string{"-f", "secrets.yaml", "--show-secrets"},
		WantExitCode: 0,
		WantStdout:   []string{"User: testuser", "Password: secretpass123", "API Key: api-key-456"},
		SkipNoDocker: true,
	})
}
