package schema

// StepBase contains common fields for all step types
type StepBase struct {
	// Name is a human readable name for the step
	Name Name `yaml:"name" json:"name" jsonschema:"required,minLength=1"`
	// ID is an optional identifier to reference this step
	ID ID `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	// Description is an optional human readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`
	// Config is optional step-level configuration
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Env are optional environment variables
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Needs lists service IDs that must be healthy before this step runs.
	// All services are implicitly available to all steps on the internal network.
	// Use Needs only when this step must wait for specific services to be ready.
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
}

// OptionalStepBase is like StepBase but with optional name (for parallel/sequence)
type OptionalStepBase struct {
	// Name is an optional human readable name for the step
	Name Name `yaml:"name,omitempty" json:"name,omitempty" jsonschema:"minLength=1"`
	// ID is an optional identifier to reference this step
	ID ID `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	// Description is an optional human readable description
	Description Description `yaml:"description,omitempty" json:"description,omitempty"`
	// Config is optional step-level configuration
	Config Config `yaml:"config,omitempty" json:"config,omitempty"`
	// Env are optional environment variables
	Env Env `yaml:"env,omitempty" json:"env,omitempty"`
	// Secrets are optional secrets
	Secrets Secrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	// Needs lists service IDs that must be healthy before this step runs.
	Needs []string `yaml:"needs,omitempty" json:"needs,omitempty"`
}

// StringOrStringSlice can be either a single string or a slice of strings
type StringOrStringSlice struct {
	Single   *string
	Multiple []string
}

// UnmarshalYAML implements custom unmarshaling for StringOrStringSlice
func (s *StringOrStringSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		s.Single = &single
		return nil
	}

	var multiple []string
	if err := unmarshal(&multiple); err == nil {
		s.Multiple = multiple
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StringOrStringSlice
func (s StringOrStringSlice) MarshalYAML() (interface{}, error) {
	if s.Single != nil {
		return *s.Single, nil
	}
	return s.Multiple, nil
}

// StringMapOrSlice can be either a map[string]string or []string
type StringMapOrSlice struct {
	Map   map[string]string
	Slice []string
}

// UnmarshalYAML implements custom unmarshaling for StringMapOrSlice
func (s *StringMapOrSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]string
	if err := unmarshal(&m); err == nil {
		s.Map = m
		return nil
	}

	var sl []string
	if err := unmarshal(&sl); err == nil {
		s.Slice = sl
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StringMapOrSlice
func (s StringMapOrSlice) MarshalYAML() (interface{}, error) {
	if s.Map != nil {
		return s.Map, nil
	}
	return s.Slice, nil
}

// NumberOrString can be either a number or a string
type NumberOrString struct {
	Number *float64
	String *string
}

// UnmarshalYAML implements custom unmarshaling for NumberOrString
func (n *NumberOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var num float64
	if err := unmarshal(&num); err == nil {
		n.Number = &num
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		n.String = &s
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for NumberOrString
func (n NumberOrString) MarshalYAML() (interface{}, error) {
	if n.Number != nil {
		return *n.Number, nil
	}
	return n.String, nil
}

// UlimitValue can be either a number or a "soft:hard" string
type UlimitValue = NumberOrString

// PullPolicy represents an image pull policy
type PullPolicy string

const (
	PullPolicyAlways  PullPolicy = "always"
	PullPolicyMissing PullPolicy = "missing"
	PullPolicyNever   PullPolicy = "never"
)

// HealthCheck defines how to determine when a container is ready.
// Used with background containers to wait for readiness before continuing.
type HealthCheck struct {
	// Cmd is the command to run for health check
	Cmd string `yaml:"cmd" json:"cmd" jsonschema:"required"`
	// Interval is the time between health checks (e.g., "10s", "1m")
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	// Timeout is the timeout for each health check attempt (e.g., "5s")
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// Retries is the number of retries before considering unhealthy
	Retries int `yaml:"retries,omitempty" json:"retries,omitempty"`
	// StartPeriod is the grace period before health checks start (e.g., "30s")
	StartPeriod string `yaml:"startPeriod,omitempty" json:"startPeriod,omitempty"`
}

// ExposePort represents a single port exposure configuration
type ExposePort struct {
	// ContainerPort is the port inside the container
	ContainerPort int `yaml:"containerPort" json:"containerPort" jsonschema:"required,minimum=1,maximum=65535"`
	// HostPort is the port on the host (defaults to ContainerPort, may be reassigned if conflict)
	HostPort int `yaml:"hostPort,omitempty" json:"hostPort,omitempty" jsonschema:"minimum=1,maximum=65535"`
	// Protocol is the protocol type (defaults to "http")
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty" jsonschema:"enum=http,enum=https,enum=tcp,enum=udp"`
}

// Expose handles the expose syntax:
//   - expose: 8080                    (single port)
//   - expose: [8080, 9229]            (array of ports)
//   - expose: [{containerPort: 3000, hostPort: 80, protocol: http}] (array of ExposePort objects)
type Expose struct {
	Ports []ExposePort
}

// UnmarshalYAML implements custom unmarshaling for Expose
func (e *Expose) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try single int
	var singleInt int
	if err := unmarshal(&singleInt); err == nil {
		e.Ports = []ExposePort{{ContainerPort: singleInt, HostPort: singleInt, Protocol: "http"}}
		return nil
	}

	// Try array of ints
	var intArray []int
	if err := unmarshal(&intArray); err == nil {
		e.Ports = make([]ExposePort, len(intArray))
		for i, port := range intArray {
			e.Ports[i] = ExposePort{ContainerPort: port, HostPort: port, Protocol: "http"}
		}
		return nil
	}

	// Try array of ExposePort objects
	var portArray []ExposePort
	if err := unmarshal(&portArray); err == nil {
		e.Ports = portArray
		// Apply defaults
		for i := range e.Ports {
			if e.Ports[i].HostPort == 0 {
				e.Ports[i].HostPort = e.Ports[i].ContainerPort
			}
			if e.Ports[i].Protocol == "" {
				e.Ports[i].Protocol = "http"
			}
		}
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for Expose
func (e Expose) MarshalYAML() (interface{}, error) {
	if len(e.Ports) == 0 {
		return nil, nil
	}
	// Check if all ports are simple (containerPort == hostPort, protocol == http)
	allSimple := true
	for _, p := range e.Ports {
		if p.HostPort != p.ContainerPort || p.Protocol != "http" {
			allSimple = false
			break
		}
	}
	if allSimple {
		if len(e.Ports) == 1 {
			return e.Ports[0].ContainerPort, nil
		}
		// Return array of ints
		ports := make([]int, len(e.Ports))
		for i, p := range e.Ports {
			ports[i] = p.ContainerPort
		}
		return ports, nil
	}
	// Return full array of objects
	return e.Ports, nil
}

// RunStep represents a step that runs a container.
//
// OCW automatically provides:
//   - /workflow mount (the directory containing the workflow file)
//   - Rootless execution by default
//   - Network isolation with firewall controls
type RunStep struct {
	StepBase `yaml:",inline" json:",inline"`

	// === Core ===
	// Image is the container image to run
	Image string `yaml:"image" json:"image" jsonschema:"required"`
	// Cmd is the command to run (overrides image CMD)
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	// Args are command arguments
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Entrypoint overrides container entrypoint
	Entrypoint string `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	// Workdir is the working directory
	Workdir string `yaml:"workdir,omitempty" json:"workdir,omitempty"`

	// === Background Execution ===
	// Background runs the container in the background (detached).
	// The step completes immediately after the container starts (or after healthcheck passes).
	// Background containers are automatically cleaned up when the job/workflow completes.
	Background bool `yaml:"background,omitempty" json:"background,omitempty"`
	// HealthCheck determines when a background container is ready.
	// If set, the step waits for the health check to pass before continuing.
	// If not set, the step continues immediately after the container starts.
	HealthCheck *HealthCheck `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
	// Expose makes the container's ports accessible from the host.
	// Requires background: true. Exposed services are listed after startup.
	// Can be a single port, array of ports, or array of ExposePort objects.
	Expose *Expose `yaml:"expose,omitempty" json:"expose,omitempty"`

	// === Environment ===
	// RunEnv are environment variables (supports both map and array format)
	// Extends workflow-level env
	RunEnv *StringMapOrSlice `yaml:"env,omitempty" json:"env,omitempty"`
	// EnvFile is one or more environment files from workspace
	// Useful for config assembled by previous steps
	EnvFile *StringOrStringSlice `yaml:"envFile,omitempty" json:"envFile,omitempty"`

	// === Resource Limits ===
	// CPUs is the number of CPUs
	CPUs *NumberOrString `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	// Memory is the memory limit (e.g., "512m", "2g")
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty"`

	// === GPU Access ===
	// GPUs are GPU devices ("all" or number)
	GPUs *NumberOrString `yaml:"gpus,omitempty" json:"gpus,omitempty"`

	// === Platform ===
	// Platform is the platform (e.g., "linux/amd64")
	Platform string `yaml:"platform,omitempty" json:"platform,omitempty"`

	// === Image Pull ===
	// Pull is the image pull policy
	Pull PullPolicy `yaml:"pull,omitempty" json:"pull,omitempty" jsonschema:"enum=always,enum=missing,enum=never"`

	// === Output Control ===
	// Quiet suppresses pull output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`
	// TTY allocates pseudo-TTY (useful for colored output)
	TTY bool `yaml:"tty,omitempty" json:"tty,omitempty"`
}

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
}

// ParallelStep represents a step that runs steps in parallel
type ParallelStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Parallel are the steps to run in parallel
	Parallel []Step `yaml:"parallel" json:"parallel" jsonschema:"required"`
}

// SequenceStep represents a step that runs steps in sequence
type SequenceStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Sequence are the steps to run in sequence
	Sequence []Step `yaml:"sequence" json:"sequence" jsonschema:"required"`
}

// StepOrSteps can be a single step or an array of steps
type StepOrSteps struct {
	Single   *Step
	Multiple []Step
}

// UnmarshalYAML implements custom unmarshaling for StepOrSteps
func (s *StepOrSteps) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single Step
	if err := unmarshal(&single); err == nil {
		s.Single = &single
		return nil
	}

	var multiple []Step
	if err := unmarshal(&multiple); err == nil {
		s.Multiple = multiple
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for StepOrSteps
func (s StepOrSteps) MarshalYAML() (interface{}, error) {
	if s.Single != nil {
		return s.Single, nil
	}
	return s.Multiple, nil
}

// SwitchStep represents a step that switches on a value
type SwitchStep struct {
	OptionalStepBase `yaml:",inline" json:",inline"`
	// Switch is the expression to switch on
	Switch string `yaml:"switch" json:"switch" jsonschema:"required"`
	// Case are the case branches
	Case map[string]StepOrSteps `yaml:"case" json:"case" jsonschema:"required"`
	// Default is the default branch
	Default *StepOrSteps `yaml:"default,omitempty" json:"default,omitempty"`
}

// Step represents any step type (discriminated union)
type Step struct {
	// RunStep for container run steps
	RunStep *RunStep
	// BuildStep for image build steps
	BuildStep *BuildStep
	// ParallelStep for parallel execution
	ParallelStep *ParallelStep
	// SequenceStep for sequential execution
	SequenceStep *SequenceStep
	// WorkflowStep for running other workflows
	WorkflowStep *WorkflowStep
	// SwitchStep for conditional branching
	SwitchStep *SwitchStep
}

// UnmarshalYAML implements custom unmarshaling for Step
func (s *Step) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Probe for discriminating fields
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return err
	}

	// Check for each step type based on its discriminating field
	if _, ok := probe["parallel"]; ok {
		var step ParallelStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.ParallelStep = &step
		return nil
	}

	if _, ok := probe["sequence"]; ok {
		var step SequenceStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.SequenceStep = &step
		return nil
	}

	if _, ok := probe["workflow"]; ok {
		var step WorkflowStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.WorkflowStep = &step
		return nil
	}

	if _, ok := probe["switch"]; ok {
		var step SwitchStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.SwitchStep = &step
		return nil
	}

	if _, ok := probe["build"]; ok {
		var step BuildStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.BuildStep = &step
		return nil
	}

	// Default to RunStep (has "image" field)
	if _, ok := probe["image"]; ok {
		var step RunStep
		if err := unmarshal(&step); err != nil {
			return err
		}
		s.RunStep = &step
		return nil
	}

	return nil
}

// MarshalYAML implements custom marshaling for Step
func (s Step) MarshalYAML() (interface{}, error) {
	if s.RunStep != nil {
		return s.RunStep, nil
	}
	if s.BuildStep != nil {
		return s.BuildStep, nil
	}
	if s.ParallelStep != nil {
		return s.ParallelStep, nil
	}
	if s.SequenceStep != nil {
		return s.SequenceStep, nil
	}
	if s.WorkflowStep != nil {
		return s.WorkflowStep, nil
	}
	if s.SwitchStep != nil {
		return s.SwitchStep, nil
	}
	return nil, nil
}
