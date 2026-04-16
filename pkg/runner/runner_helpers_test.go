package runner

import (
	"testing"
	"time"
)

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected bool
	}{
		{
			name:     "valid simple hostname",
			hostname: "app",
			expected: true,
		},
		{
			name:     "valid hostname with numbers",
			hostname: "app123",
			expected: true,
		},
		{
			name:     "valid hostname with hyphen",
			hostname: "my-app",
			expected: true,
		},
		{
			name:     "valid multi-hyphen hostname",
			hostname: "my-test-app",
			expected: true,
		},
		{
			name:     "single character",
			hostname: "a",
			expected: true,
		},
		{
			name:     "maximum length (63 chars)",
			hostname: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxy",
			expected: true,
		},
		{
			name:     "invalid: starts with uppercase",
			hostname: "App",
			expected: false,
		},
		{
			name:     "invalid: starts with number",
			hostname: "1app",
			expected: false,
		},
		{
			name:     "invalid: starts with hyphen",
			hostname: "-app",
			expected: false,
		},
		{
			name:     "invalid: ends with hyphen",
			hostname: "app-",
			expected: false,
		},
		{
			name:     "invalid: contains uppercase",
			hostname: "myApp",
			expected: false,
		},
		{
			name:     "invalid: contains underscore",
			hostname: "my_app",
			expected: false,
		},
		{
			name:     "invalid: contains dot",
			hostname: "my.app",
			expected: false,
		},
		{
			name:     "invalid: contains special character",
			hostname: "my@app",
			expected: false,
		},
		{
			name:     "invalid: empty string",
			hostname: "",
			expected: false,
		},
		{
			name:     "invalid: too long (64 chars)",
			hostname: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz12",
			expected: false,
		},
		{
			name:     "invalid: contains space",
			hostname: "my app",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("isValidHostname(%q) = %v; want %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal time.Duration
		expected   time.Duration
	}{
		{
			name:       "valid duration - seconds",
			input:      "5s",
			defaultVal: 1 * time.Second,
			expected:   5 * time.Second,
		},
		{
			name:       "valid duration - minutes",
			input:      "2m",
			defaultVal: 10 * time.Second,
			expected:   2 * time.Minute,
		},
		{
			name:       "valid duration - hours",
			input:      "1h",
			defaultVal: 5 * time.Minute,
			expected:   1 * time.Hour,
		},
		{
			name:       "valid duration - milliseconds",
			input:      "500ms",
			defaultVal: 1 * time.Second,
			expected:   500 * time.Millisecond,
		},
		{
			name:       "valid duration - complex",
			input:      "1h30m",
			defaultVal: 1 * time.Second,
			expected:   90 * time.Minute,
		},
		{
			name:       "empty string - uses default",
			input:      "",
			defaultVal: 10 * time.Second,
			expected:   10 * time.Second,
		},
		{
			name:       "invalid duration - uses default",
			input:      "invalid",
			defaultVal: 5 * time.Second,
			expected:   5 * time.Second,
		},
		{
			name:       "invalid format - uses default",
			input:      "5",
			defaultVal: 2 * time.Second,
			expected:   2 * time.Second,
		},
		{
			name:       "zero duration",
			input:      "0s",
			defaultVal: 10 * time.Second,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDuration(tt.input, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("parseDuration(%q, %v) = %v; want %v", tt.input, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestSplitEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple key=value",
			input:    "KEY=value",
			expected: []string{"KEY", "value"},
		},
		{
			name:     "key with empty value",
			input:    "KEY=",
			expected: []string{"KEY", ""},
		},
		{
			name:     "value with equals",
			input:    "KEY=value=with=equals",
			expected: []string{"KEY", "value=with=equals"},
		},
		{
			name:     "no equals sign",
			input:    "JUST_KEY",
			expected: []string{"JUST_KEY"},
		},
		{
			name:     "complex value",
			input:    "DATABASE_URL=postgres://user:pass@localhost:5432/db",
			expected: []string{"DATABASE_URL", "postgres://user:pass@localhost:5432/db"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitEnvVar(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitEnvVar(%q) returned %d parts; want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitEnvVar(%q)[%d] = %q; want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		c        byte
		expected int
	}{
		{
			name:     "character found at beginning",
			s:        "=hello",
			c:        '=',
			expected: 0,
		},
		{
			name:     "character found in middle",
			s:        "key=value",
			c:        '=',
			expected: 3,
		},
		{
			name:     "character found at end",
			s:        "hello=",
			c:        '=',
			expected: 5,
		},
		{
			name:     "character not found",
			s:        "hello",
			c:        '=',
			expected: -1,
		},
		{
			name:     "empty string",
			s:        "",
			c:        '=',
			expected: -1,
		},
		{
			name:     "multiple occurrences - returns first",
			s:        "a=b=c",
			c:        '=',
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.s, tt.c)
			if result != tt.expected {
				t.Errorf("indexOf(%q, %q) = %d; want %d", tt.s, tt.c, result, tt.expected)
			}
		})
	}
}
