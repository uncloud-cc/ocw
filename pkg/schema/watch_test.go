package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestWatch_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *Watch)
		wantErr bool
	}{
		{
			name: "bool true",
			yaml: "watch: true",
			check: func(t *testing.T, w *Watch) {
				if w.Bool == nil || !*w.Bool {
					t.Error("expected Bool to be true")
				}
			},
			wantErr: false,
		},
		{
			name: "bool false",
			yaml: "watch: false",
			check: func(t *testing.T, w *Watch) {
				if w.Bool == nil || *w.Bool {
					t.Error("expected Bool to be false")
				}
			},
			wantErr: false,
		},
		{
			name: "single string glob",
			yaml: `watch: "src/**/*.go"`,
			check: func(t *testing.T, w *Watch) {
				if w.String == nil || *w.String != "src/**/*.go" {
					t.Error("expected String to be 'src/**/*.go'")
				}
			},
			wantErr: false,
		},
		{
			name: "array of globs",
			yaml: "watch:\n  - src/**/*.go\n  - pkg/**/*.go",
			check: func(t *testing.T, w *Watch) {
				if len(w.Strings) != 2 {
					t.Errorf("expected 2 strings, got %d", len(w.Strings))
				}
				if w.Strings[0] != "src/**/*.go" {
					t.Errorf("expected first string to be 'src/**/*.go', got %q", w.Strings[0])
				}
				if w.Strings[1] != "pkg/**/*.go" {
					t.Errorf("expected second string to be 'pkg/**/*.go', got %q", w.Strings[1])
				}
			},
			wantErr: false,
		},
		{
			name: "full config object",
			yaml: `watch:
  files:
    - "src/**/*.go"
  ignore:
    - "**/*_test.go"
  useGitIgnore: false
  mode: reload`,
			check: func(t *testing.T, w *Watch) {
				if w.Config == nil {
					t.Fatal("expected Config to be set")
				}
				if len(w.Config.Files) != 1 || w.Config.Files[0] != "src/**/*.go" {
					t.Error("expected Files to contain 'src/**/*.go'")
				}
				if len(w.Config.Ignore) != 1 || w.Config.Ignore[0] != "**/*_test.go" {
					t.Error("expected Ignore to contain '**/*_test.go'")
				}
				if w.Config.UseGitIgnore == nil || *w.Config.UseGitIgnore {
					t.Error("expected UseGitIgnore to be false")
				}
				if w.Config.Mode != WatchModeReload {
					t.Errorf("expected Mode to be 'reload', got %q", w.Config.Mode)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Watch *Watch `yaml:"watch"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Watch != nil {
				tt.check(t, obj.Watch)
			}
		})
	}
}

func TestWatch_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		watch    *Watch
		expected string
	}{
		{
			name: "bool true",
			watch: &Watch{
				Bool: boolPtr(true),
			},
			expected: "watch: true\n",
		},
		{
			name: "bool false",
			watch: &Watch{
				Bool: boolPtr(false),
			},
			expected: "watch: false\n",
		},
		{
			name: "single string",
			watch: &Watch{
				String: stringPtr("src/**/*.go"),
			},
			expected: "watch: src/**/*.go\n",
		},
		{
			name: "array of strings",
			watch: &Watch{
				Strings: []string{"src/**/*.go", "pkg/**/*.go"},
			},
			expected: "watch:\n- src/**/*.go\n- pkg/**/*.go\n",
		},
		{
			name:     "nil watch",
			watch:    nil,
			expected: "watch: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Watch *Watch `yaml:"watch"`
			}{
				Watch: tt.watch,
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
