package ocw

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// errorsEqual reports whether two errors are equal (both nil or same message).
func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}

// ---------------------------------------------------------------------------
// Event assertion helpers
// ---------------------------------------------------------------------------

// AssertContainsEvent asserts that at least one event with the given type exists.
func AssertContainsEvent(t testing.TB, events []Event, eventType string) {
	t.Helper()
	for _, ev := range events {
		if ev.EventType() == eventType {
			return
		}
	}
	t.Fatalf("expected at least one %q event, got none", eventType)
}

// AssertContainsEventWith asserts that at least one event of the given type
// contains substr in any of its relevant string fields.
func AssertContainsEventWith(t testing.TB, events []Event, eventType string, substr string) {
	t.Helper()
	filteredEvents := []Event{}
	for _, ev := range events {
		if ev.EventType() != eventType {
			continue
		}

		filteredEvents = append(filteredEvents, ev)

		if EventContains(ev, substr) {
			return
		}
	}
	t.Fatalf("expected at least one %q event containing %q, instead got:\n%s", eventType, substr, formatEvents(filteredEvents))
}

// formatEvents returns a human-readable string representation of a slice of events.
func formatEvents(events []Event) string {
	var parts []string
	for _, ev := range events {
		parts = append(parts, formatEvent(ev))
	}
	return strings.Join(parts, "\n")
}

// formatEvent returns a human-readable string representation of a single event.
func formatEvent(ev Event) string {
	v := reflect.ValueOf(ev)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	return fmt.Sprintf("- %s: %+v", v.Type().String(), v.Interface())
}

// EventContains checks whether any string field (recursively, including maps,
// slices, and nested structs) in the event contains substr.
// It uses reflection so it works automatically for any new event type.
func EventContains(ev Event, substr string) bool {
	return reflectContains(reflect.ValueOf(ev), substr)
}

// reflectContains recursively searches a reflect.Value for a string containing substr.
func reflectContains(v reflect.Value, substr string) bool {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		return strings.Contains(v.String(), substr)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			// Skip unexported fields to avoid noise from internal state
			if !field.CanInterface() {
				continue
			}
			if reflectContains(field, substr) {
				return true
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			if reflectContains(val, substr) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if reflectContains(v.Index(i), substr) {
				return true
			}
		}
	case reflect.Interface:
		if !v.IsNil() {
			if reflectContains(v.Elem(), substr) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Event collector
// ---------------------------------------------------------------------------

// EventCollector consumes events from a channel and stores them in a list.
type EventCollector struct {
	Events []Event
	done   chan struct{}
}

// CollectEvents starts a goroutine that collects events from the bus.
func CollectEvents(bus *EventBus) *EventCollector {
	c := &EventCollector{done: make(chan struct{})}
	ch := bus.Subscribe(64)
	go func() {
		defer close(c.done)
		for ev := range ch {
			c.Events = append(c.Events, ev.Event)
		}
	}()
	return c
}

// Wait blocks until the event channel is closed.
func (c *EventCollector) Wait() {
	<-c.done
}
