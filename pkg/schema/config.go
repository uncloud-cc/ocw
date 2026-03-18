package schema

// ConfigNamespace represents a namespace of config values for tool extensions.
// Config fields are natural extension points for tools implementing OCW support.
type ConfigNamespace = map[string]any

// Config is a map of namespaced config values.
// Config field providers need to be installed in an OCW using the "extensions" field.
type Config = map[string]ConfigNamespace

// EnvVar represents an environment variable that can be either a plain string
// or marked as a secret with an optional default value
type EnvVar struct {
	// Value is the default value (for plain strings or secrets with defaults)
	Value string
	// IsSecret marks whether this is a secret
	IsSecret bool
}

// UnmarshalYAML implements custom unmarshaling for EnvVar
// It accepts either:
//   - A plain string: "value"
//   - A secret object: { secret: true, default: "value" }
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a simple string first
	var plain string
	if err := unmarshal(&plain); err == nil {
		e.Value = plain
		e.IsSecret = false
		return nil
	}

	// Try to unmarshal as a secret marker object
	var secretObj struct {
		Secret  bool   `yaml:"secret"`
		Default string `yaml:"default"`
	}
	if err := unmarshal(&secretObj); err == nil {
		e.Value = secretObj.Default
		e.IsSecret = secretObj.Secret
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for EnvVar
func (e EnvVar) MarshalYAML() (interface{}, error) {
	if e.IsSecret {
		return struct {
			Secret  bool   `yaml:"secret"`
			Default string `yaml:"default,omitempty"`
		}{
			Secret:  true,
			Default: e.Value,
		}, nil
	}
	return e.Value, nil
}

// Env is a map of environment variables
type Env = map[string]EnvVar

// SecureString represents an encrypted secret value
type SecureString struct {
	Secure string `yaml:"secure" json:"secure" jsonschema:"required"`
}

// SecretValue can be either a plain string or a secure (encrypted) string.
// The "secure" attribute stores the encrypted value, while a plain string
// stores the unencrypted version. Implementing platforms are expected to detect
// any plaintext key-value pairs and automatically encrypt the values.
type SecretValue struct {
	// Plain is the unencrypted secret value (use with caution)
	Plain string
	// Secure is the encrypted secret value
	Secure *SecureString
}

// UnmarshalYAML implements custom unmarshaling for SecretValue
func (s *SecretValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a simple string first
	var plain string
	if err := unmarshal(&plain); err == nil {
		s.Plain = plain
		return nil
	}

	// Try to unmarshal as a secure object
	var secure SecureString
	if err := unmarshal(&secure); err == nil {
		s.Secure = &secure
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for SecretValue
func (s SecretValue) MarshalYAML() (interface{}, error) {
	if s.Secure != nil {
		return s.Secure, nil
	}
	return s.Plain, nil
}

// Secrets is a map of secret values
type Secrets = map[string]SecretValue
