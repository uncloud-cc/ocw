package schema

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

	// === Watch Mode ===
	// Watch enables automatic reloading when source files change.
	// Requires background: true. Can be bool, glob pattern(s), or full config.
	Watch *Watch `yaml:"watch,omitempty" json:"watch,omitempty"`

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

	// === Volume Access ===
	// Volumes grant access to named volumes for this step
	Volumes VolumeRefs `yaml:"volumes,omitempty" json:"volumes,omitempty"`
}
