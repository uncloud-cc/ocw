package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestEnvVar_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *EnvVar)
		wantErr bool
	}{
		{
			name: "plain string value",
			yaml: "var: hello",
			check: func(t *testing.T, e *EnvVar) {
				if e.Value != "hello" {
					t.Errorf("expected Value to be 'hello', got %q", e.Value)
				}
				if e.IsSecret {
					t.Error("expected IsSecret to be false")
				}
			},
			wantErr: false,
		},
		{
			name: "secret with default",
			yaml: "var:\n  secret: true\n  default: default_value",
			check: func(t *testing.T, e *EnvVar) {
				if e.Value != "default_value" {
					t.Errorf("expected Value to be 'default_value', got %q", e.Value)
				}
				if !e.IsSecret {
					t.Error("expected IsSecret to be true")
				}
			},
			wantErr: false,
		},
		{
			name: "secret without default",
			yaml: "var:\n  secret: true",
			check: func(t *testing.T, e *EnvVar) {
				if e.Value != "" {
					t.Errorf("expected Value to be empty, got %q", e.Value)
				}
				if !e.IsSecret {
					t.Error("expected IsSecret to be true")
				}
			},
			wantErr: false,
		},
		{
			name: "secret false",
			yaml: "var:\n  secret: false\n  default: value",
			check: func(t *testing.T, e *EnvVar) {
				if e.Value != "value" {
					t.Errorf("expected Value to be 'value', got %q", e.Value)
				}
				if e.IsSecret {
					t.Error("expected IsSecret to be false")
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Var EnvVar `yaml:"var"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				tt.check(t, &obj.Var)
			}
		})
	}
}

func TestEnvVar_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		envVar   EnvVar
		expected string
	}{
		{
			name: "plain value",
			envVar: EnvVar{
				Value:    "hello",
				IsSecret: false,
			},
			expected: "var: hello\n",
		},
		{
			name: "secret with default",
			envVar: EnvVar{
				Value:    "default_value",
				IsSecret: true,
			},
			expected: "var:\n  secret: true\n  default: default_value\n",
		},
		{
			name: "secret without default",
			envVar: EnvVar{
				Value:    "",
				IsSecret: true,
			},
			expected: "var:\n  secret: true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Var EnvVar `yaml:"var"`
			}{
				Var: tt.envVar,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}

func TestSecretValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *SecretValue)
		wantErr bool
	}{
		{
			name: "plain string",
			yaml: "secret: plaintext",
			check: func(t *testing.T, s *SecretValue) {
				if s.Plain != "plaintext" {
					t.Errorf("expected Plain to be 'plaintext', got %q", s.Plain)
				}
				if s.Secure != nil {
					t.Error("expected Secure to be nil")
				}
			},
			wantErr: false,
		},
		{
			name: "secure string",
			yaml: "secret:\n  secure: encrypted_value",
			check: func(t *testing.T, s *SecretValue) {
				if s.Plain != "" {
					t.Errorf("expected Plain to be empty, got %q", s.Plain)
				}
				if s.Secure == nil {
					t.Fatal("expected Secure to be set")
				}
				if s.Secure.Secure != "encrypted_value" {
					t.Errorf("expected Secure.Secure to be 'encrypted_value', got %q", s.Secure.Secure)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Secret SecretValue `yaml:"secret"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				tt.check(t, &obj.Secret)
			}
		})
	}
}

func TestSecretValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		secret   SecretValue
		expected string
	}{
		{
			name: "plain value",
			secret: SecretValue{
				Plain: "plaintext",
			},
			expected: "secret: plaintext\n",
		},
		{
			name: "secure value",
			secret: SecretValue{
				Secure: &SecureString{
					Secure: "encrypted_value",
				},
			},
			expected: "secret:\n  secure: encrypted_value\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Secret SecretValue `yaml:"secret"`
			}{
				Secret: tt.secret,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}
