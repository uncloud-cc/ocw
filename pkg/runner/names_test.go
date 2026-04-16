package runner

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid name with letters and numbers",
			input:    "test123",
			expected: "test123",
		},
		{
			name:     "name with spaces",
			input:    "my test container",
			expected: "my-test-container",
		},
		{
			name:     "name with special characters",
			input:    "test@container#name!",
			expected: "test-container-name",
		},
		{
			name:     "name with leading special characters",
			input:    "---test",
			expected: "test",
		},
		{
			name:     "name with trailing hyphens",
			input:    "test---",
			expected: "test",
		},
		{
			name:     "name with multiple consecutive hyphens",
			input:    "test---container",
			expected: "test-container",
		},
		{
			name:     "uppercase letters",
			input:    "TestContainer",
			expected: "testcontainer",
		},
		{
			name:     "mixed case with symbols",
			input:    "My-Test_Container.123",
			expected: "my-test_container.123",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "container",
		},
		{
			name:     "only special characters",
			input:    "@#$%",
			expected: "container",
		},
		{
			name:     "already valid container name",
			input:    "nginx",
			expected: "nginx",
		},
		{
			name:     "hyphenated name",
			input:    "my-app",
			expected: "my-app",
		},
		{
			name:     "underscored name",
			input:    "my_app",
			expected: "my_app",
		},
		{
			name:     "dotted name",
			input:    "my.app",
			expected: "my.app",
		},
		{
			name:     "name starting with number",
			input:    "123app",
			expected: "123app",
		},
		{
			name:     "name with unicode characters",
			input:    "test café",
			expected: "test-caf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeName(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
