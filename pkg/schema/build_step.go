package schema

// OutputType represents build output types
type OutputType string

const (
	OutputTypeDocker   OutputType = "docker"
	OutputTypeImage    OutputType = "image"
	OutputTypeLocal    OutputType = "local"
	OutputTypeTar      OutputType = "tar"
	OutputTypeOCI      OutputType = "oci"
	OutputTypeRegistry OutputType = "registry"
)

// CompressionType represents compression types for build output
type CompressionType string

const (
	CompressionGzip         CompressionType = "gzip"
	CompressionEstargz      CompressionType = "estargz"
	CompressionZstd         CompressionType = "zstd"
	CompressionUncompressed CompressionType = "uncompressed"
)

// OutputConfig represents build output configuration
type OutputConfig struct {
	Type             OutputType        `yaml:"type" json:"type" jsonschema:"required,enum=docker,enum=image,enum=local,enum=tar,enum=oci,enum=registry"`
	Dest             string            `yaml:"dest,omitempty" json:"dest,omitempty"`
	Push             bool              `yaml:"push,omitempty" json:"push,omitempty"`
	Compression      CompressionType   `yaml:"compression,omitempty" json:"compression,omitempty" jsonschema:"enum=gzip,enum=estargz,enum=zstd,enum=uncompressed"`
	CompressionLevel int               `yaml:"compressionLevel,omitempty" json:"compressionLevel,omitempty" jsonschema:"minimum=0,maximum=9"`
	ForceCompression bool              `yaml:"forceCompression,omitempty" json:"forceCompression,omitempty"`
	OCIMediaTypes    bool              `yaml:"ociMediatypes,omitempty" json:"ociMediatypes,omitempty"`
	Annotation       map[string]string `yaml:"annotation,omitempty" json:"annotation,omitempty"`
}

// BuildOutput can be a string, array of strings, OutputConfig, or array of OutputConfig
type BuildOutput struct {
	String  *string
	Strings []string
	Config  *OutputConfig
	Configs []OutputConfig
}

// UnmarshalYAML implements custom unmarshaling for BuildOutput
func (b *BuildOutput) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		b.String = &s
		return nil
	}

	var ss []string
	if err := unmarshal(&ss); err == nil {
		b.Strings = ss
		return nil
	}

	var c OutputConfig
	if err := unmarshal(&c); err == nil {
		b.Config = &c
		return nil
	}

	var cs []OutputConfig
	if err := unmarshal(&cs); err == nil {
		b.Configs = cs
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for BuildOutput
func (b BuildOutput) MarshalYAML() (interface{}, error) {
	if b.String != nil {
		return *b.String, nil
	}
	if b.Strings != nil {
		return b.Strings, nil
	}
	if b.Config != nil {
		return b.Config, nil
	}
	return b.Configs, nil
}

// BuildSecretConfig represents a build secret configuration
type BuildSecretConfig struct {
	ID  string `yaml:"id" json:"id" jsonschema:"required"`
	Src string `yaml:"src,omitempty" json:"src,omitempty"`
	Env string `yaml:"env,omitempty" json:"env,omitempty"`
}

// BuildSecrets can be a map or array of BuildSecretConfig
type BuildSecrets struct {
	Map   map[string]string
	Array []BuildSecretConfig
}

// UnmarshalYAML implements custom unmarshaling for BuildSecrets
func (b *BuildSecrets) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]string
	if err := unmarshal(&m); err == nil {
		b.Map = m
		return nil
	}

	var a []BuildSecretConfig
	if err := unmarshal(&a); err == nil {
		b.Array = a
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for BuildSecrets
func (b BuildSecrets) MarshalYAML() (interface{}, error) {
	if b.Map != nil {
		return b.Map, nil
	}
	return b.Array, nil
}

// BuildProgressMode represents build progress output modes
type BuildProgressMode string

const (
	BuildProgressAuto    BuildProgressMode = "auto"
	BuildProgressQuiet   BuildProgressMode = "quiet"
	BuildProgressPlain   BuildProgressMode = "plain"
	BuildProgressTTY     BuildProgressMode = "tty"
	BuildProgressRawJSON BuildProgressMode = "rawjson"
)

// BoolOrString can be either a boolean or a string
type BoolOrString struct {
	Bool   *bool
	String *string
}

// UnmarshalYAML implements custom unmarshaling for BoolOrString
func (b *BoolOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var bl bool
	if err := unmarshal(&bl); err == nil {
		b.Bool = &bl
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		b.String = &s
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for BoolOrString
func (b BoolOrString) MarshalYAML() (interface{}, error) {
	if b.Bool != nil {
		return *b.Bool, nil
	}
	return b.String, nil
}

// BuildConfig represents the build configuration.
//
// OCW automatically provides:
//   - /workspace as the default build context (copy-on-write overlay)
//   - Secrets from workflow-level secrets via --secret
//   - Network isolation during build
type BuildConfig struct {
	// === Core Options ===
	// Image is the primary image name (equivalent to first -t tag)
	Image string `yaml:"image" json:"image" jsonschema:"required"`
	// Context is the build context path (defaults to /workspace)
	Context string `yaml:"context,omitempty" json:"context,omitempty" jsonschema:"default=/workspace"`

	// === Dockerfile ===
	// Dockerfile is the path to Dockerfile
	Dockerfile string `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`

	// === Multi-stage Builds ===
	// Target is the build target
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// === Build Arguments ===
	// BuildArgs are build arguments
	BuildArgs map[string]string `yaml:"buildArgs,omitempty" json:"buildArgs,omitempty"`

	// === Platform/Architecture ===
	// Platform is the target platform(s)
	Platform *StringOrStringSlice `yaml:"platform,omitempty" json:"platform,omitempty"`

	// === Caching ===
	// CacheFrom are cache sources
	CacheFrom []string `yaml:"cacheFrom,omitempty" json:"cacheFrom,omitempty"`
	// CacheTo are cache export destinations
	CacheTo []string `yaml:"cacheTo,omitempty" json:"cacheTo,omitempty"`
	// NoCache disables cache
	NoCache bool `yaml:"noCache,omitempty" json:"noCache,omitempty"`
	// NoCacheFilter disables cache for specific stages
	NoCacheFilter []string `yaml:"noCacheFilter,omitempty" json:"noCacheFilter,omitempty"`

	// === Tags and Output ===
	// Tags are additional tags
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	// Output is the output destination
	Output *BuildOutput `yaml:"output,omitempty" json:"output,omitempty"`

	// === Push/Load ===
	// Push pushes to registry
	Push bool `yaml:"push,omitempty" json:"push,omitempty"`
	// Load loads into docker images
	Load bool `yaml:"load,omitempty" json:"load,omitempty"`

	// === Base Image Handling ===
	// Pull always pulls base images
	Pull bool `yaml:"pull,omitempty" json:"pull,omitempty"`

	// === Secrets (fed through OCW's secret handling) ===
	// BuildSecrets are build secrets - uses OCW's secret management
	BuildSecrets *BuildSecrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`

	// === Labels & Annotations ===
	// Labels are metadata labels
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	// Annotation are OCI annotations
	Annotation *StringMapOrSlice `yaml:"annotation,omitempty" json:"annotation,omitempty"`

	// === Resource Limits ===
	// ShmSize is the shared memory size (e.g., "128m")
	ShmSize string `yaml:"shmSize,omitempty" json:"shmSize,omitempty"`
	// Ulimit are ulimits
	Ulimit map[string]UlimitValue `yaml:"ulimit,omitempty" json:"ulimit,omitempty"`

	// === Progress & Logging ===
	// Progress is the progress output mode
	Progress BuildProgressMode `yaml:"progress,omitempty" json:"progress,omitempty" jsonschema:"enum=auto,enum=quiet,enum=plain,enum=tty,enum=rawjson"`
	// Quiet suppresses build output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`

	// === Attestations (BuildKit 0.11+) ===
	// Provenance is the provenance attestation setting
	Provenance *BoolOrString `yaml:"provenance,omitempty" json:"provenance,omitempty"`
	// SBOM is the SBOM attestation setting
	SBOM *BoolOrString `yaml:"sbom,omitempty" json:"sbom,omitempty"`
	// Attest are custom attestations
	Attest []string `yaml:"attest,omitempty" json:"attest,omitempty"`

	// === Metadata Output ===
	// MetadataFile writes build metadata JSON
	MetadataFile string `yaml:"metadataFile,omitempty" json:"metadataFile,omitempty"`
	// IIDFile writes image ID to file
	IIDFile string `yaml:"iidfile,omitempty" json:"iidfile,omitempty"`

	// === Additional Build Contexts ===
	// BuildContext are additional build contexts
	BuildContext map[string]string `yaml:"buildContext,omitempty" json:"buildContext,omitempty"`
}

// BuildStep represents a step that builds an image
type BuildStep struct {
	StepBase `yaml:",inline" json:",inline"`
	// Build is the build configuration
	Build BuildConfig `yaml:"build" json:"build" jsonschema:"required"`

	// === Volume Access ===
	// Volumes grant access to named volumes during build
	Volumes VolumeRefs `yaml:"volumes,omitempty" json:"volumes,omitempty"`
}
