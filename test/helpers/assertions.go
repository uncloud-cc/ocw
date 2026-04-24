// Package testhelpers provides utilities for testing OCW workflows.
// These helpers are used by both unit and integration tests to verify
// workflow execution behavior.
package testhelpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// AssertSequentialOrder verifies that steps ran in strict sequential order:
// each step ended before the next one started.
func AssertSequentialOrder(t *testing.T, rec *RecordingRuntime, names []string) {
	t.Helper()
	for i := 0; i < len(names)-1; i++ {
		end := rec.EndTime(names[i])
		start := rec.StartTime(names[i+1])
		assert.False(t, end.IsZero(), "step %q has no end event", names[i])
		assert.False(t, start.IsZero(), "step %q has no start event", names[i+1])
		assert.True(t, end.Before(start) || end.Equal(start),
			"step %q ended at %v but %q started at %v -- expected sequential order",
			names[i], end, names[i+1], start)
	}
}

// AssertParallelOverlap verifies that all named steps had overlapping execution
// windows: every step started before any of them ended.
func AssertParallelOverlap(t *testing.T, rec *RecordingRuntime, names []string) {
	t.Helper()
	// Find the latest start and earliest end across all parallel steps.
	var latestStart time.Time
	var earliestEnd time.Time
	for i, name := range names {
		s := rec.StartTime(name)
		e := rec.EndTime(name)
		assert.False(t, s.IsZero(), "step %q has no start event", name)
		assert.False(t, e.IsZero(), "step %q has no end event", name)
		if i == 0 || s.After(latestStart) {
			latestStart = s
		}
		if i == 0 || e.Before(earliestEnd) {
			earliestEnd = e
		}
	}
	// All steps must have started before the earliest one ended.
	assert.True(t, latestStart.Before(earliestEnd) || latestStart.Equal(earliestEnd),
		"expected parallel overlap: latest start=%v, earliest end=%v", latestStart, earliestEnd)
}

// AssertEventCount verifies that the expected number of events were recorded.
func AssertEventCount(t *testing.T, rec *RecordingRuntime, expected int) {
	t.Helper()
	assert.Len(t, rec.Events, expected, "expected %d events, got %d", expected, len(rec.Events))
}

// AssertHasEvent verifies that at least one event matches the given criteria.
func AssertHasEvent(t *testing.T, rec *RecordingRuntime, name, typ, phase string) {
	t.Helper()
	for _, e := range rec.Events {
		if e.Name == name && e.Type == typ && e.Phase == phase {
			return
		}
	}
	t.Errorf("expected event with name=%q, type=%q, phase=%q not found", name, typ, phase)
}

// AssertEventOrder verifies that events occurred in the expected order.
func AssertEventOrder(t *testing.T, rec *RecordingRuntime, events []struct{ Name, Phase string }) {
	t.Helper()
	if len(events) < 2 {
		return
	}

	for i := 0; i < len(events)-1; i++ {
		current := rec.EndTime(events[i].Name)
		if events[i].Phase == "start" {
			current = rec.StartTime(events[i].Name)
		}

		next := rec.StartTime(events[i+1].Name)
		if events[i+1].Phase == "end" {
			next = rec.EndTime(events[i+1].Name)
		}

		assert.False(t, current.IsZero(), "event %d (name=%q, phase=%q) not found", i, events[i].Name, events[i].Phase)
		assert.False(t, next.IsZero(), "event %d (name=%q, phase=%q) not found", i+1, events[i+1].Name, events[i+1].Phase)
		assert.True(t, current.Before(next) || current.Equal(next),
			"event %d (name=%q, phase=%q at %v) should be before event %d (name=%q, phase=%q at %v)",
			i, events[i].Name, events[i].Phase, current,
			i+1, events[i+1].Name, events[i+1].Phase, next)
	}
}
