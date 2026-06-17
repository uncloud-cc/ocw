package ocw

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        error
		b        error
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "first nil",
			a:        nil,
			b:        errors.New("some error"),
			expected: false,
		},
		{
			name:     "second nil",
			a:        errors.New("some error"),
			b:        nil,
			expected: false,
		},
		{
			name:     "same message",
			a:        errors.New("same error"),
			b:        errors.New("same error"),
			expected: true,
		},
		{
			name:     "different message",
			a:        errors.New("error a"),
			b:        errors.New("error b"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorsEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("errorsEqual(%v, %v) = %v; expected %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestAssertContainsEvent(t *testing.T) {
	// Only test the success path; the failure path is trivial (t.Fatalf).
	AssertContainsEvent(t, []Event{
		&StepStart{Name: "build", StepType: "run"},
		&StepComplete{Name: "build", Success: true},
	}, "step.start")
}

func TestAssertContainsEventWith(t *testing.T) {
	// Only test the success path; the failure path is trivial (t.Fatalf).
	AssertContainsEventWith(t, []Event{
		&ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"},
	}, "container.output", "World")
}

func TestEventContains(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		substr   string
		expected bool
	}{
		{
			name:     "ContainerOutput matches Line",
			event:    &ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"},
			substr:   "World",
			expected: true,
		},
		{
			name:     "ContainerOutput matches Step",
			event:    &ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"},
			substr:   "hello",
			expected: true,
		},
		{
			name:     "ContainerOutput no match",
			event:    &ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"},
			substr:   "Goodbye",
			expected: false,
		},
		{
			name:     "StepStart matches Name",
			event:    &StepStart{Name: "build", StepType: "run"},
			substr:   "build",
			expected: true,
		},
		{
			name:     "StepStart matches Extra",
			event:    &StepStart{Name: "build", StepType: "run", Extra: map[string]string{"key": "value"}},
			substr:   "value",
			expected: true,
		},
		{
			name:     "StepComplete matches Name",
			event:    &StepComplete{Name: "build", Success: true},
			substr:   "build",
			expected: true,
		},
		{
			name:     "LogDebug matches Message",
			event:    &LogDebug{Message: "debug msg", Fields: map[string]any{"key": "val"}},
			substr:   "debug",
			expected: true,
		},
		{
			name:     "LogDebug matches Field",
			event:    &LogDebug{Message: "debug msg", Fields: map[string]any{"key": "val"}},
			substr:   "val",
			expected: true,
		},
		{
			name:     "WorkflowStart matches Name",
			event:    &WorkflowStart{Name: "Hello World"},
			substr:   "Hello",
			expected: true,
		},
		{
			name:     "WorkflowOutputs matches Output",
			event:    &WorkflowOutputs{Title: "Outputs", Outputs: map[string]string{"url": "https://example.com"}},
			substr:   "example.com",
			expected: true,
		},
		{
			name:     "unknown event type",
			event:    &WorkflowStart{Name: "test"},
			substr:   "nope",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EventContains(tt.event, tt.substr)
			if got != tt.expected {
				t.Errorf("EventContains(%T, %q) = %v; expected %v", tt.event, tt.substr, got, tt.expected)
			}
		})
	}
}

func TestCollectEvents(t *testing.T) {
	bus := NewEventBus()
	collector := CollectEvents(bus)

	bus.Event(&StepStart{Name: "hello", StepType: "run"})
	bus.Event(&ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"})
	bus.Event(&StepComplete{Name: "hello", Success: true})

	bus.Close()
	collector.Wait()

	if len(collector.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(collector.Events))
	}

	AssertContainsEvent(t, collector.Events, "step.start")
	AssertContainsEvent(t, collector.Events, "container.output")
	AssertContainsEvent(t, collector.Events, "step.complete")
}

type mockTB struct {
	HelperCalled bool
	FatalfMsg    string
}

func (m *mockTB) Helper()                        { m.HelperCalled = true }
func (m *mockTB) Fatalf(msg string, args ...any) { m.FatalfMsg = fmt.Sprintf(msg, args...) }

func TestAssertEventMatches(t *testing.T) {
	events := []Event{
		&StepStart{Name: "build", StepType: "run"},
		&ContainerOutput{Step: "hello", Stream: "stdout", Line: "Hello World"},
		&StepComplete{Name: "build", Success: true},
		&WorkflowComplete{Name: "test", Success: false},
	}

	t.Run("fields only success", func(t *testing.T) {
		AssertEventMatches(t, events, "step.complete", "", map[string]any{"Success": true})
	})

	t.Run("contains only success", func(t *testing.T) {
		AssertEventMatches(t, events, "container.output", "World", nil)
	})

	t.Run("combined success", func(t *testing.T) {
		AssertEventMatches(t, events, "container.output", "Hello", map[string]any{"Stream": "stdout"})
	})

	t.Run("no criteria success", func(t *testing.T) {
		AssertEventMatches(t, events, "step.start", "", nil)
	})

	t.Run("fields only failure message", func(t *testing.T) {
		m := &mockTB{}
		AssertEventMatches(m, events, "workflow.complete", "", map[string]any{"Success": true})
		if m.FatalfMsg == "" {
			t.Fatal("expected Fatalf to be called")
		}
		want := `expected at least one "workflow.complete" event matching fields map[Success:true]`
		if !strings.Contains(m.FatalfMsg, want) {
			t.Fatalf("expected Fatalf message to contain %q, got:\n%s", want, m.FatalfMsg)
		}
	})

	t.Run("contains only failure message", func(t *testing.T) {
		m := &mockTB{}
		AssertEventMatches(m, events, "container.output", "Goodbye", nil)
		if m.FatalfMsg == "" {
			t.Fatal("expected Fatalf to be called")
		}
		want := `expected at least one "container.output" event matching contains "Goodbye"`
		if !strings.Contains(m.FatalfMsg, want) {
			t.Fatalf("expected Fatalf message to contain %q, got:\n%s", want, m.FatalfMsg)
		}
	})

	t.Run("combined failure message", func(t *testing.T) {
		m := &mockTB{}
		AssertEventMatches(m, events, "container.output", "Goodbye", map[string]any{"Success": true})
		if m.FatalfMsg == "" {
			t.Fatal("expected Fatalf to be called")
		}
		wantContains := `contains "Goodbye"`
		wantFields := `fields map[Success:true]`
		if !strings.Contains(m.FatalfMsg, wantContains) || !strings.Contains(m.FatalfMsg, wantFields) {
			t.Fatalf("expected Fatalf message to contain both %q and %q, got:\n%s", wantContains, wantFields, m.FatalfMsg)
		}
	})

	t.Run("no criteria failure message", func(t *testing.T) {
		m := &mockTB{}
		AssertEventMatches(m, events, "workflow.start", "", nil)
		if m.FatalfMsg == "" {
			t.Fatal("expected Fatalf to be called")
		}
		want := `expected at least one "workflow.start" event matching any`
		if !strings.Contains(m.FatalfMsg, want) {
			t.Fatalf("expected Fatalf message to contain %q, got:\n%s", want, m.FatalfMsg)
		}
	})
}
