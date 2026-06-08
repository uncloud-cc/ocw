package ocw

// Event is implemented by every concrete event type (see below)
type Event interface {
	EventType() string
}

// Event types
const (
	EventWorkflowStart    = "workflow.start"
	EventWorkflowComplete = "workflow.complete"
	EventGroupHeader      = "group.header"
	EventStepStart        = "step.start"
	EventStepComplete     = "step.complete"
	EventContainerOutput  = "container.output"
	EventWorkflowOutputs  = "workflow.outputs"
	EventLogDebug         = "log.debug"
	EventLogInfo          = "log.info"
	EventLogWarn          = "log.warn"
	EventLogError         = "log.error"
	EventServicesOverview      = "services.overview"
	EventWaiting               = "waiting"
	EventHealthCheckStart      = "healthcheck.start"
	EventHealthCheckProgress   = "healthcheck.progress"
	EventHealthCheckComplete   = "healthcheck.complete"
)

type LogDebug struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogDebug) EventType() string { return EventLogDebug }

type LogInfo struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogInfo) EventType() string { return EventLogInfo }

type LogWarn struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogWarn) EventType() string { return EventLogWarn }

type LogError struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogError) EventType() string { return EventLogError }

type WorkflowStart struct {
	Name        string   `json:"name"`
	Directory   string   `json:"directory,omitempty"`
	LoadedFiles []string `json:"loaded_files,omitempty"`
}

func (WorkflowStart) EventType() string { return EventWorkflowStart }

type WorkflowComplete struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
}

func (WorkflowComplete) EventType() string { return EventWorkflowComplete }

type GroupHeader struct {
	Text string `json:"text"`
}

func (GroupHeader) EventType() string { return EventGroupHeader }

type StepStart struct {
	Name     string            `json:"name"`
	StepType string            `json:"type"`
	Extra    map[string]string `json:"extra,omitempty"`
}

func (StepStart) EventType() string { return EventStepStart }

type StepComplete struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

func (StepComplete) EventType() string { return EventStepComplete }

type ContainerOutput struct {
	Step   string `json:"step"`
	Stream string `json:"stream"`
	Line   string `json:"line"`
}

func (ContainerOutput) EventType() string { return EventContainerOutput }

type WorkflowOutputs struct {
	Title   string            `json:"title"`
	Outputs map[string]string `json:"outputs"`
}

func (WorkflowOutputs) EventType() string { return EventWorkflowOutputs }

type HealthCheckStart struct {
	Name string `json:"name"`
}

func (HealthCheckStart) EventType() string { return EventHealthCheckStart }

type HealthCheckProgress struct {
	Name    string `json:"name"`
	Attempt int    `json:"attempt"`
	Status  string `json:"status,omitempty"`
}

func (HealthCheckProgress) EventType() string { return EventHealthCheckProgress }

type HealthCheckComplete struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
}

func (HealthCheckComplete) EventType() string { return EventHealthCheckComplete }
