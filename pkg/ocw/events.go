package ocw

// Package ocw/events.go defines the NDJSON event protocol used as the source
// of truth for all workflow output.
//
// It provides:
//   - Typed Event structs (WorkflowStart, StepComplete, LogInfo, etc.)
//   - A two-phase ParseEvent() that unmarshals NDJSON into concrete types
//   - EventWriter, an NDJSON Logger implementation
//   - BaseEvent embedding for automatic event type and timestamp injection
//
// Event types are registered in a central eventFactories map — adding a new
// event requires only: define the struct, add one line to the map.
//
// Secret masking is handled externally by mask.go before serialization.
// Pretty rendering is handled by pretty.go as a pure consumer.
// The Logger interface lives in logger.go.
//
// For human-readable output, use PrettyPrinter (implements Logger).
// For machine-readable NDJSON, use EventWriter (implements Logger).
//
// Usage:
//
//	w := NewEventWriter(os.Stdout, false, secrets)
//	w.Event(&WorkflowStart{Name: "deploy"})
//	w.Info("step complete", map[string]any{"name": "build"})

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Event is implemented by every concrete event type.
type Event interface {
	EventType() string
}

// BaseEvent holds the envelope fields (event type + timestamp).
// It is embedded in every concrete event struct; json flattens it.
type BaseEvent struct {
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
}

// eventBase is an internal contract that EventWriter uses to auto-populate
// BaseEvent fields. Any concrete event embedding BaseEvent satisfies this
// automatically when passed as a pointer.
type eventBase interface {
	Event
	setBase(event, timestamp string)
}

func (b *BaseEvent) setBase(event, timestamp string) {
	b.Event = event
	b.Timestamp = timestamp
}

// ── Event type constants ─────────────────────────────────────

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
)

// ── Central factory map: single place to add new events ─────

var eventFactories = map[string]func() Event{
	EventWorkflowStart:    func() Event { return &WorkflowStart{} },
	EventWorkflowComplete: func() Event { return &WorkflowComplete{} },
	EventGroupHeader:      func() Event { return &GroupHeader{} },
	EventStepStart:        func() Event { return &StepStart{} },
	EventStepComplete:     func() Event { return &StepComplete{} },
	EventContainerOutput:  func() Event { return &ContainerOutput{} },
	EventWorkflowOutputs:  func() Event { return &WorkflowOutputs{} },
	EventLogDebug:         func() Event { return &LogDebug{} },
	EventLogInfo:          func() Event { return &LogInfo{} },
	EventLogWarn:          func() Event { return &LogWarn{} },
	EventLogError:         func() Event { return &LogError{} },
}

// ── Concrete event structs (payload only, BaseEvent embedded) ─

type WorkflowStart struct {
	BaseEvent
	Name        string   `json:"name"`
	Directory   string   `json:"directory,omitempty"`
	LoadedFiles []string `json:"loaded_files,omitempty"`
}

func (WorkflowStart) EventType() string { return EventWorkflowStart }

type WorkflowComplete struct {
	BaseEvent
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
}

func (WorkflowComplete) EventType() string { return EventWorkflowComplete }

type GroupHeader struct {
	BaseEvent
	Text string `json:"text"`
}

func (GroupHeader) EventType() string { return EventGroupHeader }

type StepStart struct {
	BaseEvent
	Name     string            `json:"name"`
	StepType string            `json:"type"`
	Extra    map[string]string `json:"extra,omitempty"`
}

func (StepStart) EventType() string { return EventStepStart }

type StepComplete struct {
	BaseEvent
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

func (StepComplete) EventType() string { return EventStepComplete }

type ContainerOutput struct {
	BaseEvent
	Step   string `json:"step"`
	Stream string `json:"stream"`
	Line   string `json:"line"`
}

func (ContainerOutput) EventType() string { return EventContainerOutput }

type WorkflowOutputs struct {
	BaseEvent
	Title   string            `json:"title"`
	Outputs map[string]string `json:"outputs"`
}

func (WorkflowOutputs) EventType() string { return EventWorkflowOutputs }

type LogDebug struct {
	BaseEvent
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogDebug) EventType() string { return EventLogDebug }

type LogInfo struct {
	BaseEvent
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogInfo) EventType() string { return EventLogInfo }

type LogWarn struct {
	BaseEvent
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogWarn) EventType() string { return EventLogWarn }

type LogError struct {
	BaseEvent
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (LogError) EventType() string { return EventLogError }

// ── Parser: looks up the factory map (no giant switch) ──────

// ParseEvent reads a single NDJSON line and returns the concrete Event.
func ParseEvent(data []byte) (Event, error) {
	var envelope BaseEvent
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	factory, ok := eventFactories[envelope.Event]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %q", envelope.Event)
	}

	ev := factory()
	return ev, json.Unmarshal(data, ev)
}

// ── EventWriter: NDJSON Logger ─────────────────────────────

type EventWriter struct {
	out         io.Writer
	mu          sync.Mutex
	secrets     []string
	showSecrets bool
}

func NewEventWriter(out io.Writer, showSecrets bool, secrets []string) *EventWriter {
	return &EventWriter{
		out:         out,
		secrets:     secrets,
		showSecrets: showSecrets,
	}
}

// Event writes any Event as a single NDJSON line. It automatically injects
// the event type and timestamp, masks secrets, and writes to the output.
// Callers should pass a pointer to a struct embedding BaseEvent.
func (w *EventWriter) Event(ev Event) {
	if e, ok := ev.(eventBase); ok {
		e.setBase(e.EventType(), time.Now().UTC().Format(time.RFC3339Nano))
	}

	ev = MaskEvent(ev, w.secrets, w.showSecrets)

	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintln(w.out, string(data))
}

// ── Standard logging helpers (Logger interface) ─────────────

func (w *EventWriter) Debug(msg string, fields map[string]any) {
	w.Event(&LogDebug{Message: msg, Fields: fields})
}

func (w *EventWriter) Info(msg string, fields map[string]any) {
	w.Event(&LogInfo{Message: msg, Fields: fields})
}

func (w *EventWriter) Warn(msg string, fields map[string]any) {
	w.Event(&LogWarn{Message: msg, Fields: fields})
}

func (w *EventWriter) Error(msg string, fields map[string]any) {
	w.Event(&LogError{Message: msg, Fields: fields})
}
