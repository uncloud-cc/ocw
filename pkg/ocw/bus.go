package ocw

import (
	"sync"
	"time"
)

// EventBus is the central event emitter. It broadcasts Event instances to all
// registered consumers over individual buffered channels.
type EventBus struct {
	consumers   []chan<- IngestedEvent
	secrets     []string
	showSecrets bool
	mu          sync.RWMutex
}

type IngestedEvent struct {
	EventMetadata
	Event
}

type EventMetadata struct {
	Type       string `json:"type"`
	Timestanmp string `json:"timestamp"`
}

// NewEventBus creates a new EventBus with secret masking configuration.
func NewEventBus() *EventBus {
	return &EventBus{}
}

// SetSecrets updates the secrets masking settings of the eventbus
func (b *EventBus) SetSecrets(showSecrets bool, secrets []string) {
	b.secrets = secrets
	b.showSecrets = showSecrets
}

// Subscribe registers a new consumer and returns the receive-only channel.
// The caller is responsible for running a goroutine that ranges over the
// returned channel. Buffer size controls how many events can be queued
// before the bus blocks.
func (b *EventBus) Subscribe(bufSize int) <-chan IngestedEvent {
	ch := make(chan IngestedEvent, bufSize)
	b.mu.Lock()
	b.consumers = append(b.consumers, ch)
	b.mu.Unlock()
	return ch
}

// Event emits an event to every subscribed consumer. It injects the base
// timestamp, masks secrets once, and blocks until the event is queued in
// all consumer channels.
func (b *EventBus) Event(ev Event) {
	maskedEvent := MaskEvent(ev, b.secrets, b.showSecrets)
	e := IngestedEvent{
		Event: maskedEvent,
		EventMetadata: EventMetadata{
			Type:       ev.EventType(),
			Timestanmp: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}

	b.mu.RLock()
	consumers := b.consumers
	b.mu.RUnlock()

	for _, ch := range consumers {
		ch <- e // blocks if channel full
	}
}

// Close closes all consumer channels, signaling consumers to shut down.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.consumers {
		close(ch)
	}
	b.consumers = nil
}

// Convenience helper to send logging events - use Event() for anything else
func (b *EventBus) Debug(msg string) {
	b.Event(&LogDebug{Message: msg, Fields: map[string]any{}})
}

func (b *EventBus) Info(msg string) {
	b.Event(&LogInfo{Message: msg, Fields: map[string]any{}})
}

func (b *EventBus) Warn(msg string) {
	b.Event(&LogWarn{Message: msg, Fields: map[string]any{}})
}

func (b *EventBus) Error(msg string) {
	b.Event(&LogError{Message: msg, Fields: map[string]any{}})
}

func (b *EventBus) DebugWithData(msg string, fields map[string]any) {
	b.Event(&LogDebug{Message: msg, Fields: fields})
}

func (b *EventBus) InfoWithData(msg string, fields map[string]any) {
	b.Event(&LogInfo{Message: msg, Fields: fields})
}

func (b *EventBus) WarnWithData(msg string, fields map[string]any) {
	b.Event(&LogWarn{Message: msg, Fields: fields})
}

func (b *EventBus) ErrorWithData(msg string, fields map[string]any) {
	b.Event(&LogError{Message: msg, Fields: fields})
}
