package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnquoteValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain value",
			input:    "value",
			expected: "value",
		},
		{
			name:     "double quoted value",
			input:    `"value"`,
			expected: "value",
		},
		{
			name:     "single quoted value",
			input:    `'value'`,
			expected: "value",
		},
		{
			name:     "double quoted with escaped quotes",
			input:    `"value with \"quotes\""`,
			expected: `value with "quotes"`,
		},
		{
			name:     "double quoted with backslash",
			input:    `"value\\path"`,
			expected: `value\path`,
		},
		{
			name:     "double quoted with newline",
			input:    `"line1\nline2"`,
			expected: "line1\nline2",
		},
		{
			name:     "double quoted with tab",
			input:    `"value\ttab"`,
			expected: "value\ttab",
		},
		{
			name:     "single quoted no escape processing",
			input:    `'value\n'`,
			expected: `value\n`,
		},
		{
			name:     "value with leading/trailing spaces",
			input:    "  value  ",
			expected: "value",
		},
		{
			name:     "empty quoted string",
			input:    `""`,
			expected: "",
		},
		{
			name:     "empty single quoted string",
			input:    `''`,
			expected: "",
		},
		{
			name:     "unmatched quote",
			input:    `"value`,
			expected: `"value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unquoteValue(tt.input)
			if result != tt.expected {
				t.Errorf("unquoteValue(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadDotEnvFile(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expected    map[string]string
		wantErr     bool
	}{
		{
			name:        "simple key-value pairs",
			fileContent: "KEY1=value1\nKEY2=value2\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "with comments",
			fileContent: "# Comment\nKEY1=value1\n# Another comment\nKEY2=value2\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "with empty lines",
			fileContent: "KEY1=value1\n\nKEY2=value2\n\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "with export prefix",
			fileContent: "export KEY1=value1\nKEY2=value2\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "with quoted values",
			fileContent: "KEY1=\"value with spaces\"\nKEY2='single quoted'\n",
			expected: map[string]string{
				"KEY1": "value with spaces",
				"KEY2": "single quoted",
			},
			wantErr: false,
		},
		{
			name:        "with equals in value",
			fileContent: "KEY1=value=with=equals\n",
			expected: map[string]string{
				"KEY1": "value=with=equals",
			},
			wantErr: false,
		},
		{
			name:        "with spaces around equals",
			fileContent: "KEY1 = value1\nKEY2= value2\nKEY3 =value3\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
			wantErr: false,
		},
		{
			name:        "empty value",
			fileContent: "KEY1=\nKEY2=value2\n",
			expected: map[string]string{
				"KEY1": "",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "line without equals",
			fileContent: "KEY1=value1\nINVALID_LINE\nKEY2=value2\n",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:        "empty file",
			fileContent: "",
			expected:    map[string]string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp file
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")
			err := os.WriteFile(envFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create temp .env file: %v", err)
			}

			dotenv, err := LoadDotEnvFile(envFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadDotEnvFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(dotenv.Vars) != len(tt.expected) {
					t.Errorf("LoadDotEnvFile() got %d vars, want %d", len(dotenv.Vars), len(tt.expected))
				}
				for key, expectedValue := range tt.expected {
					if gotValue, ok := dotenv.Vars[key]; !ok {
						t.Errorf("LoadDotEnvFile() missing key %q", key)
					} else if gotValue != expectedValue {
						t.Errorf("LoadDotEnvFile() key %q = %q; want %q", key, gotValue, expectedValue)
					}
				}
			}
		})
	}
}

func TestLoadDotEnvFileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.env")

	dotenv, err := LoadDotEnvFile(nonExistentFile)
	if err != nil {
		t.Errorf("LoadDotEnvFile() should not error on non-existent file, got: %v", err)
	}
	if len(dotenv.Vars) != 0 {
		t.Errorf("LoadDotEnvFile() should return empty vars for non-existent file, got: %v", dotenv.Vars)
	}
}

func TestDotEnvGet(t *testing.T) {
	dotenv := &DotEnv{
		Vars: map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		},
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "existing key",
			key:      "KEY1",
			expected: "value1",
		},
		{
			name:     "another existing key",
			key:      "KEY2",
			expected: "value2",
		},
		{
			name:     "non-existent key",
			key:      "KEY3",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotenv.Get(tt.key)
			if result != tt.expected {
				t.Errorf("Get(%q) = %q; want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestDotEnvHas(t *testing.T) {
	dotenv := &DotEnv{
		Vars: map[string]string{
			"KEY1": "value1",
			"KEY2": "",
		},
	}

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "existing key with value",
			key:      "KEY1",
			expected: true,
		},
		{
			name:     "existing key with empty value",
			key:      "KEY2",
			expected: true,
		},
		{
			name:     "non-existent key",
			key:      "KEY3",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotenv.Has(tt.key)
			if result != tt.expected {
				t.Errorf("Has(%q) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}
