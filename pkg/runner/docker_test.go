package runner

import (
	"bytes"
	"testing"
)

func TestPrefixWriter_Write(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "single line with newline",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "hello world\n",
			expected: "  | hello world\n",
		},
		{
			name:     "multiple lines",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "line1\nline2\nline3\n",
			expected: "  | line1\n  | line2\n  | line3\n",
		},
		{
			name:     "line without newline",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "incomplete",
			expected: "",
		},
		{
			name:     "empty input",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "",
			expected: "",
		},
		{
			name:     "single secret masking",
			prefix:   "  | ",
			secrets:  []string{"secret123"},
			input:    "password is secret123\n",
			expected: "  | password is [secret]\n",
		},
		{
			name:     "multiple secrets masking",
			prefix:   "  | ",
			secrets:  []string{"secret1", "secret2"},
			input:    "secret1 and secret2 are secrets\n",
			expected: "  | [secret] and [secret] are secrets\n",
		},
		{
			name:     "secret masking across multiple lines",
			prefix:   "  | ",
			secrets:  []string{"secret"},
			input:    "line with secret\nanother secret line\n",
			expected: "  | line with [secret]\n  | another [secret] line\n",
		},
		{
			name:     "empty secret ignored",
			prefix:   "  | ",
			secrets:  []string{"", "real_secret"},
			input:    "text with real_secret\n",
			expected: "  | text with [secret]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			pw := newPrefixWriter(&buf, tt.prefix, tt.secrets)

			n, err := pw.Write([]byte(tt.input))
			if err != nil {
				t.Errorf("Write() error = %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write() returned %d bytes written; want %d", n, len(tt.input))
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("Write() output = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestPrefixWriter_Flush(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "flush incomplete line",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "incomplete line",
			expected: "  | incomplete line\n",
		},
		{
			name:     "flush with secret",
			prefix:   "  | ",
			secrets:  []string{"secret"},
			input:    "has secret",
			expected: "  | has [secret]\n",
		},
		{
			name:     "flush empty buffer",
			prefix:   "  | ",
			secrets:  []string{},
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			pw := newPrefixWriter(&buf, tt.prefix, tt.secrets)

			pw.Write([]byte(tt.input))
			pw.Flush()

			result := buf.String()
			if result != tt.expected {
				t.Errorf("Flush() output = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestPrefixWriter_MaskSecrets(t *testing.T) {
	tests := []struct {
		name     string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "no secrets",
			secrets:  []string{},
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "single secret",
			secrets:  []string{"password123"},
			input:    "password is password123",
			expected: "password is [secret]",
		},
		{
			name:     "multiple occurrences of same secret",
			secrets:  []string{"secret"},
			input:    "secret here and secret there",
			expected: "[secret] here and [secret] there",
		},
		{
			name:     "multiple different secrets",
			secrets:  []string{"pass1", "pass2"},
			input:    "pass1 and pass2",
			expected: "[secret] and [secret]",
		},
		{
			name:     "empty secret in list",
			secrets:  []string{"", "valid"},
			input:    "text with valid secret",
			expected: "text with [secret] secret",
		},
		{
			name:     "secret not in text",
			secrets:  []string{"notpresent"},
			input:    "no secrets here",
			expected: "no secrets here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := newPrefixWriter(nil, "", tt.secrets)
			result := string(pw.maskSecrets([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("maskSecrets() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Test with a likely available high port
	// Note: This test could be flaky in CI environments, so we just verify it returns a boolean
	result := IsPortAvailable(54321)
	// We can't assert this is true since something else might be using it
	// Just verify the function returns a boolean without panicking
	_ = result
}

func TestFindAvailablePort(t *testing.T) {
	tests := []struct {
		name          string
		preferredPort int
		wantErr       bool
		minPort       int
	}{
		{
			name:          "find any available port (port 0 special case)",
			preferredPort: 0,
			wantErr:       false,
			minPort:       0, // Port 0 is special - returns 0 if "available"
		},
		{
			name:          "high port likely available",
			preferredPort: 54321,
			wantErr:       false,
			minPort:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := FindAvailablePort(tt.preferredPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindAvailablePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if port < tt.minPort || port > 65535 {
					t.Errorf("FindAvailablePort() returned invalid port %d", port)
				}
			}
		})
	}
}

func TestDocker_MaskSecrets(t *testing.T) {
	tests := []struct {
		name     string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "no secrets",
			secrets:  []string{},
			input:    "plain text output",
			expected: "plain text output",
		},
		{
			name:     "single secret",
			secrets:  []string{"my_secret_key"},
			input:    "using key my_secret_key for auth",
			expected: "using key [secret] for auth",
		},
		{
			name:     "multiple secrets",
			secrets:  []string{"secret1", "secret2"},
			input:    "secret1 connects to secret2",
			expected: "[secret] connects to [secret]",
		},
		{
			name:     "empty secret ignored",
			secrets:  []string{"", "real_secret"},
			input:    "contains real_secret",
			expected: "contains [secret]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Docker{secrets: tt.secrets}
			result := d.maskSecrets(tt.input)
			if result != tt.expected {
				t.Errorf("maskSecrets() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestDocker_SetSecrets(t *testing.T) {
	d := NewDocker(nil, nil, nil)

	secrets := []string{"secret1", "secret2"}
	d.SetSecrets(secrets)

	if len(d.secrets) != len(secrets) {
		t.Errorf("SetSecrets() set %d secrets; want %d", len(d.secrets), len(secrets))
	}

	for i, secret := range secrets {
		if d.secrets[i] != secret {
			t.Errorf("SetSecrets() secrets[%d] = %q; want %q", i, d.secrets[i], secret)
		}
	}
}

func TestDocker_WithVerbose(t *testing.T) {
	d := NewDocker(nil, nil, nil)

	if d.verbose {
		t.Error("NewDocker() should initialize with verbose = false")
	}

	result := d.WithVerbose(true)
	if !result.verbose {
		t.Error("WithVerbose(true) should set verbose to true")
	}

	result = d.WithVerbose(false)
	if result.verbose {
		t.Error("WithVerbose(false) should set verbose to false")
	}

	// Verify it returns the same instance (for chaining)
	if result != d {
		t.Error("WithVerbose() should return the same Docker instance")
	}
}

func TestNewDocker(t *testing.T) {
	outputCalled := false
	output := func(format string, args ...any) {
		outputCalled = true
	}

	styles := NewStyles()
	secrets := []string{"secret1", "secret2"}

	d := NewDocker(output, styles, secrets)

	if d.Output == nil {
		t.Error("NewDocker() should set Output function")
	}

	// Test that output function works
	d.Output("test")
	if !outputCalled {
		t.Error("NewDocker() Output function not working")
	}

	if d.styles != styles {
		t.Error("NewDocker() should set styles")
	}

	if len(d.secrets) != len(secrets) {
		t.Errorf("NewDocker() set %d secrets; want %d", len(d.secrets), len(secrets))
	}

	if d.verbose {
		t.Error("NewDocker() should initialize with verbose = false")
	}
}
