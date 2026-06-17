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
	AssertEventMatches(t, events, eventType, substr, nil)
}

// AssertEventMatches asserts that at least one event of the given type matches
// all provided criteria. If contains is non-empty, the event must contain that
// substring in any of its string fields. If fields is non-empty, every key
// must exactly match a struct field on the event (by name) with a value-equal
// comparison (types are coerced for numeric fields).
// EventAssertion describes a single assertion against a collected event list.
type EventAssertion struct {
	EventType string
	Contains  string
	Fields    map[string]any
}

// AssertEvents runs a slice of EventAssertions against a list of events.
func AssertEvents(t testing.TB, events []Event, assertions []EventAssertion) {
	t.Helper()
	for _, a := range assertions {
		AssertEventMatches(t, events, a.EventType, a.Contains, a.Fields)
	}
}

type fatalHelper interface {
	Helper()
	Fatalf(format string, args ...any)
}

func AssertEventMatches(t fatalHelper, events []Event, eventType string, contains string, fields map[string]any) {
	t.Helper()
	filteredEvents := []Event{}
	for _, ev := range events {
		if ev.EventType() != eventType {
			continue
		}
		filteredEvents = append(filteredEvents, ev)

		if contains != "" && !EventContains(ev, contains) {
			continue
		}
		if fieldsMatch(ev, fields) {
			return
		}
	}

	var parts []string
	if contains != "" {
		parts = append(parts, fmt.Sprintf("contains %q", contains))
	}
	if len(fields) > 0 {
		parts = append(parts, fmt.Sprintf("fields %v", fields))
	}
	criteria := "any"
	if len(parts) > 0 {
		criteria = strings.Join(parts, " and ")
	}

	t.Fatalf("expected at least one %q event matching %s, instead got:\n%s", eventType, criteria, formatEvents(filteredEvents))
}

// fieldsMatch checks whether all entries in fields match the corresponding
// exported struct fields on ev by name. It supports bool, string, and numeric
// types with cross-kind coercion (e.g. int == int64).
func fieldsMatch(ev Event, fields map[string]any) bool {
	if len(fields) == 0 {
		return true
	}
	v := reflect.ValueOf(ev)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	for fieldName, expected := range fields {
		field := v.FieldByName(fieldName)
		if !field.IsValid() || !field.CanInterface() {
			return false
		}
		if !valuesEqual(field, expected) {
			return false
		}
	}
	return true
}

// valuesEqual compares a reflect.Value with an expected value, coercing
// numeric types so that int, int64, float64, etc. can match each other.
func valuesEqual(v reflect.Value, expected any) bool {
	if !v.IsValid() {
		return expected == nil
	}
	v = reflect.Indirect(v)

	ev := reflect.ValueOf(expected)
	if !ev.IsValid() {
		return false
	}
	ev = reflect.Indirect(ev)

	// Exact type match
	if v.Type() == ev.Type() {
		return reflect.DeepEqual(v.Interface(), ev.Interface())
	}

	// Numeric coercion
	if isNumeric(v) && isNumeric(ev) {
		return toFloat64(v) == toFloat64(ev)
	}

	// String comparison
	if v.Kind() == reflect.String && ev.Kind() == reflect.String {
		return v.String() == ev.String()
	}

	// Bool comparison
	if v.Kind() == reflect.Bool && ev.Kind() == reflect.Bool {
		return v.Bool() == ev.Bool()
	}

	return false
}

func isNumeric(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func toFloat64(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	}
	return 0
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
