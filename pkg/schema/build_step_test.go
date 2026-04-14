package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestBuildOutput_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *BuildOutput)
		wantErr bool
	}{
		{
			name: "string value",
			yaml: "output: type=docker",
			check: func(t *testing.T, b *BuildOutput) {
				if b.String == nil || *b.String != "type=docker" {
					t.Error("expected String to be 'type=docker'")
				}
			},
			wantErr: false,
		},
		{
			name: "array of strings",
			yaml: "output:\n- type=docker\n- type=local,dest=/tmp/build",
			check: func(t *testing.T, b *BuildOutput) {
				if b.Strings == nil || len(b.Strings) != 2 {
					t.Fatalf("expected 2 strings, got %d", len(b.Strings))
				}
				if b.Strings[0] != "type=docker" {
					t.Errorf("expected first string 'type=docker', got %q", b.Strings[0])
				}
				if b.Strings[1] != "type=local,dest=/tmp/build" {
					t.Errorf("expected second string 'type=local,dest=/tmp/build', got %q", b.Strings[1])
				}
			},
			wantErr: false,
		},
		{
			name: "OutputConfig object",
			yaml: "output:\n  type: docker\n  push: true",
			check: func(t *testing.T, b *BuildOutput) {
				if b.Config == nil {
					t.Fatal("expected Config to be set")
				}
				if b.Config.Type != OutputTypeDocker {
					t.Errorf("expected Type 'docker', got %q", b.Config.Type)
				}
				if !b.Config.Push {
					t.Error("expected Push to be true")
				}
			},
			wantErr: false,
		},
		{
			name: "array of OutputConfig objects",
			yaml: "output:\n- type: docker\n  push: true\n- type: local\n  dest: /tmp/out",
			check: func(t *testing.T, b *BuildOutput) {
				if b.Configs == nil || len(b.Configs) != 2 {
					t.Fatalf("expected 2 configs, got %d", len(b.Configs))
				}
				if b.Configs[0].Type != OutputTypeDocker {
					t.Errorf("expected first Type 'docker', got %q", b.Configs[0].Type)
				}
				if !b.Configs[0].Push {
					t.Error("expected first Push to be true")
				}
				if b.Configs[1].Type != OutputTypeLocal {
					t.Errorf("expected second Type 'local', got %q", b.Configs[1].Type)
				}
				if b.Configs[1].Dest != "/tmp/out" {
					t.Errorf("expected Dest '/tmp/out', got %q", b.Configs[1].Dest)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Output *BuildOutput `yaml:"output"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Output != nil {
				tt.check(t, obj.Output)
			}
		})
	}
}

func TestBuildOutput_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		output   *BuildOutput
		expected string
	}{
		{
			name: "string value",
			output: &BuildOutput{
				String: strPtr("type=docker"),
			},
			expected: "output: type=docker\n",
		},
		{
			name: "array of strings",
			output: &BuildOutput{
				Strings: []string{"type=docker", "type=local,dest=/tmp"},
			},
			expected: "output:\n- type=docker\n- type=local,dest=/tmp\n",
		},
		{
			name: "OutputConfig object",
			output: &BuildOutput{
				Config: &OutputConfig{
					Type: OutputTypeDocker,
					Push: true,
				},
			},
			expected: "output:\n  type: docker\n  push: true\n",
		},
		{
			name: "array of OutputConfig objects",
			output: &BuildOutput{
				Configs: []OutputConfig{
					{Type: OutputTypeDocker, Push: true},
					{Type: OutputTypeLocal, Dest: "/tmp/out"},
				},
			},
			expected: "output:\n- type: docker\n  push: true\n- type: local\n  dest: /tmp/out\n",
		},
		{
			name:     "nil output",
			output:   nil,
			expected: "output: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Output *BuildOutput `yaml:"output"`
			}{
				Output: tt.output,
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

func TestBuildSecrets_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *BuildSecrets)
		wantErr bool
	}{
		{
			name: "map of secrets",
			yaml: "secrets:\n  aws: $AWS_SECRET_KEY\n  token: $GITHUB_TOKEN",
			check: func(t *testing.T, b *BuildSecrets) {
				if b.Map == nil || len(b.Map) != 2 {
					t.Fatalf("expected 2 secrets in map, got %d", len(b.Map))
				}
				if b.Map["aws"] != "$AWS_SECRET_KEY" {
					t.Errorf("expected aws secret '$AWS_SECRET_KEY', got %q", b.Map["aws"])
				}
				if b.Map["token"] != "$GITHUB_TOKEN" {
					t.Errorf("expected token secret '$GITHUB_TOKEN', got %q", b.Map["token"])
				}
			},
			wantErr: false,
		},
		{
			name: "array of BuildSecretConfig",
			yaml: "secrets:\n- id: aws\n  src: /run/secrets/aws\n- id: token\n  env: GITHUB_TOKEN",
			check: func(t *testing.T, b *BuildSecrets) {
				if b.Array == nil || len(b.Array) != 2 {
					t.Fatalf("expected 2 secrets in array, got %d", len(b.Array))
				}
				if b.Array[0].ID != "aws" {
					t.Errorf("expected first ID 'aws', got %q", b.Array[0].ID)
				}
				if b.Array[0].Src != "/run/secrets/aws" {
					t.Errorf("expected first Src '/run/secrets/aws', got %q", b.Array[0].Src)
				}
				if b.Array[1].ID != "token" {
					t.Errorf("expected second ID 'token', got %q", b.Array[1].ID)
				}
				if b.Array[1].Env != "GITHUB_TOKEN" {
					t.Errorf("expected second Env 'GITHUB_TOKEN', got %q", b.Array[1].Env)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Secrets *BuildSecrets `yaml:"secrets"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Secrets != nil {
				tt.check(t, obj.Secrets)
			}
		})
	}
}

func TestBuildSecrets_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		secrets  *BuildSecrets
		expected string
	}{
		{
			name: "map of secrets",
			secrets: &BuildSecrets{
				Map: map[string]string{
					"aws":   "$AWS_SECRET_KEY",
					"token": "$GITHUB_TOKEN",
				},
			},
			// Map order is non-deterministic, so we'll just check it doesn't error
			expected: "",
		},
		{
			name: "array of BuildSecretConfig",
			secrets: &BuildSecrets{
				Array: []BuildSecretConfig{
					{ID: "aws", Src: "/run/secrets/aws"},
					{ID: "token", Env: "GITHUB_TOKEN"},
				},
			},
			expected: "secrets:\n- id: aws\n  src: /run/secrets/aws\n- id: token\n  env: GITHUB_TOKEN\n",
		},
		{
			name:     "nil secrets",
			secrets:  nil,
			expected: "secrets: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Secrets *BuildSecrets `yaml:"secrets"`
			}{
				Secrets: tt.secrets,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			// For map tests, just verify it doesn't error (map order is non-deterministic)
			if tt.expected == "" {
				return
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}

func TestBoolOrString_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *BoolOrString)
		wantErr bool
	}{
		{
			name: "bool true",
			yaml: "value: true",
			check: func(t *testing.T, b *BoolOrString) {
				if b.Bool == nil || *b.Bool != true {
					t.Error("expected Bool to be true")
				}
			},
			wantErr: false,
		},
		{
			name: "bool false",
			yaml: "value: false",
			check: func(t *testing.T, b *BoolOrString) {
				if b.Bool == nil || *b.Bool != false {
					t.Error("expected Bool to be false")
				}
			},
			wantErr: false,
		},
		{
			name: "string value",
			yaml: "value: mode=max",
			check: func(t *testing.T, b *BoolOrString) {
				if b.String == nil || *b.String != "mode=max" {
					t.Error("expected String to be 'mode=max'")
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Value *BoolOrString `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Value != nil {
				tt.check(t, obj.Value)
			}
		})
	}
}

func TestBoolOrString_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		value    *BoolOrString
		expected string
	}{
		{
			name: "bool true",
			value: &BoolOrString{
				Bool: boolPtr(true),
			},
			expected: "value: true\n",
		},
		{
			name: "bool false",
			value: &BoolOrString{
				Bool: boolPtr(false),
			},
			expected: "value: false\n",
		},
		{
			name: "string value",
			value: &BoolOrString{
				String: strPtr("mode=max"),
			},
			expected: "value: mode=max\n",
		},
		{
			name:     "nil value",
			value:    nil,
			expected: "value: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Value *BoolOrString `yaml:"value"`
			}{
				Value: tt.value,
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

// Helper function
func strPtr(s string) *string {
	return &s
}
