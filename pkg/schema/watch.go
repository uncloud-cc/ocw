package schema

// WatchMode represents the reload behavior when files change
type WatchMode string

const (
	// WatchModeRebuildReload rebuilds the image then reloads the container
	WatchModeRebuildReload WatchMode = "rebuild-reload"
	// WatchModeReload just restarts the container without rebuilding
	WatchModeReload WatchMode = "reload"
)

// WatchConfig represents the full watch configuration
type WatchConfig struct {
	// Files are glob patterns to watch (e.g., "src/**/*.go")
	// If empty, watches entire context directory
	Files []string `yaml:"files,omitempty" json:"files,omitempty"`
	// Ignore are additional glob patterns to ignore
	Ignore []string `yaml:"ignore,omitempty" json:"ignore,omitempty"`
	// UseGitIgnore respects .gitignore files (default: true)
	UseGitIgnore *bool `yaml:"useGitIgnore,omitempty" json:"useGitIgnore,omitempty"`
	// UseDockerIgnore respects .dockerignore files (default: true)
	UseDockerIgnore *bool `yaml:"useDockerIgnore,omitempty" json:"useDockerIgnore,omitempty"`
	// Mode is the reload mode: "rebuild-reload" or "reload" (default: "rebuild-reload")
	Mode WatchMode `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"enum=rebuild-reload,enum=reload"`
}

// Watch can be a bool, string glob, array of globs, or full WatchConfig
type Watch struct {
	Bool    *bool
	String  *string
	Strings []string
	Config  *WatchConfig
}

// UnmarshalYAML implements custom unmarshaling for Watch
func (w *Watch) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try bool first
	var b bool
	if err := unmarshal(&b); err == nil {
		w.Bool = &b
		return nil
	}

	// Try single string (glob pattern)
	var s string
	if err := unmarshal(&s); err == nil {
		w.String = &s
		return nil
	}

	// Try array of strings (glob patterns)
	var ss []string
	if err := unmarshal(&ss); err == nil {
		w.Strings = ss
		return nil
	}

	// Try full config object
	var c WatchConfig
	if err := unmarshal(&c); err == nil {
		w.Config = &c
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for Watch
func (w Watch) MarshalYAML() (interface{}, error) {
	if w.Bool != nil {
		return *w.Bool, nil
	}
	if w.String != nil {
		return *w.String, nil
	}
	if w.Strings != nil {
		return w.Strings, nil
	}
	if w.Config != nil {
		return w.Config, nil
	}
	return nil, nil
}
