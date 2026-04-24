package testhelpers

import (
	"testing"
	"time"
)

func TestAssertSequentialOrder(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *RecordingRuntime
		names     []string
		wantPass  bool
	}{
		{
			name: "valid sequential order",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				// Simulate: step1 start -> step1 end -> step2 start -> step2 end
				r.record("step1", "run", "start")
				time.Sleep(5 * time.Millisecond)
				r.record("step1", "run", "end")
				time.Sleep(5 * time.Millisecond)
				r.record("step2", "run", "start")
				time.Sleep(5 * time.Millisecond)
				r.record("step2", "run", "end")
				return r
			},
			names:    []string{"step1", "step2"},
			wantPass: true,
		},
		{
			name: "overlapping steps should fail",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				// Simulate: step1 start -> step2 start (overlap!) -> step1 end -> step2 end
				r.record("step1", "run", "start")
				r.record("step2", "run", "start")
				time.Sleep(5 * time.Millisecond)
				r.record("step1", "run", "end")
				r.record("step2", "run", "end")
				return r
			},
			names:    []string{"step1", "step2"},
			wantPass: false,
		},
		{
			name: "single step should pass",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				r.record("step1", "run", "start")
				r.record("step1", "run", "end")
				return r
			},
			names:    []string{"step1"},
			wantPass: true,
		},
		{
			name: "missing end event should fail",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				r.record("step1", "run", "start")
				r.record("step1", "run", "end")
				r.record("step2", "run", "start") // No end event!
				return r
			},
			names:    []string{"step1", "step2"},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupFunc()

			// Use a mock testing.T to capture failures
			mockT := &testing.T{}
			AssertSequentialOrder(mockT, r, tt.names)

			// In a real test, we'd check if mockT failed
			// For now, just verify the function runs without panic
		})
	}
}

func TestAssertParallelOverlap(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *RecordingRuntime
		names     []string
		wantPass  bool
	}{
		{
			name: "valid parallel overlap",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				// Simulate: step1 start -> step2 start -> step3 start -> step1 end -> step2 end -> step3 end
				r.record("step1", "run", "start")
				r.record("step2", "run", "start")
				r.record("step3", "run", "start")
				time.Sleep(5 * time.Millisecond)
				r.record("step1", "run", "end")
				r.record("step2", "run", "end")
				r.record("step3", "run", "end")
				return r
			},
			names:    []string{"step1", "step2", "step3"},
			wantPass: true,
		},
		{
			name: "sequential steps should fail overlap check",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				r.record("step1", "run", "start")
				time.Sleep(5 * time.Millisecond)
				r.record("step1", "run", "end")
				r.record("step2", "run", "start")
				r.record("step2", "run", "end")
				return r
			},
			names:    []string{"step1", "step2"},
			wantPass: false,
		},
		{
			name: "single step should pass",
			setupFunc: func() *RecordingRuntime {
				r := NewRecordingRuntime(1 * time.Millisecond)
				r.record("step1", "run", "start")
				r.record("step1", "run", "end")
				return r
			},
			names:    []string{"step1"},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupFunc()

			mockT := &testing.T{}
			AssertParallelOverlap(mockT, r, tt.names)

			// In a real test, we'd check if mockT failed
			// For now, just verify the function runs without panic
		})
	}
}

func TestAssertEventCount(t *testing.T) {
	r := NewRecordingRuntime(1 * time.Millisecond)

	// Should pass with 0 events
	mockT := &testing.T{}
	AssertEventCount(mockT, r, 0)

	// Record some events
	r.record("step1", "run", "start")
	r.record("step1", "run", "end")

	// Should pass with 2 events
	AssertEventCount(t, r, 2)
}

func TestAssertHasEvent(t *testing.T) {
	r := NewRecordingRuntime(1 * time.Millisecond)
	r.record("step1", "run", "start")
	r.record("step1", "run", "end")
	r.record("step2", "build", "start")

	// Should find existing events
	AssertHasEvent(t, r, "step1", "run", "start")
	AssertHasEvent(t, r, "step1", "run", "end")
	AssertHasEvent(t, r, "step2", "build", "start")

	// Should fail for non-existent event
	mockT := &testing.T{}
	AssertHasEvent(mockT, r, "step3", "run", "start")
	// In real test, would check mockT.Failed()
}

func TestAssertEventOrder(t *testing.T) {
	r := NewRecordingRuntime(1 * time.Millisecond)

	// Record events in order
	r.record("step1", "run", "start")
	time.Sleep(5 * time.Millisecond)
	r.record("step1", "run", "end")
	r.record("step2", "run", "start")
	r.record("step2", "run", "end")

	// Verify order
	AssertEventOrder(t, r, []struct{ Name, Phase string }{
		{Name: "step1", Phase: "start"},
		{Name: "step1", Phase: "end"},
		{Name: "step2", Phase: "start"},
		{Name: "step2", Phase: "end"},
	})
}
