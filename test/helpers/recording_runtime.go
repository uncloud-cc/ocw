// Package testhelpers provides utilities for testing OCW workflows.
// These helpers are used by both unit and integration tests to verify
// workflow execution behavior.
package testhelpers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// StepEvent records when a step started or ended.
type StepEvent struct {
	Name      string
	Type      string // "run", "build", "service-start", "service-stop"
	Phase     string // "start" or "end"
	Timestamp time.Time
}

// RecordingRuntime wraps DummyRuntime-like behavior with configurable delays
// and precise timing records. It lets tests assert:
//   - execution order for sequences
//   - overlapping execution for parallel blocks
//   - service lifecycle
type RecordingRuntime struct {
	mu       sync.Mutex
	Events   []StepEvent
	Delay    time.Duration // how long each Run/Build takes
	Services []RecordedService
	Stopped  []string
	nextID   int
}

// RecordedService records a started service.
type RecordedService struct {
	Name        string
	Image       string
	ContainerID string
}

// NewRecordingRuntime creates a new runtime that records step events.
func NewRecordingRuntime(delay time.Duration) *RecordingRuntime {
	return &RecordingRuntime{Delay: delay}
}

// record adds an event to the event log.
func (r *RecordingRuntime) record(name, typ, phase string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, StepEvent{
		Name:      name,
		Type:      typ,
		Phase:     phase,
		Timestamp: time.Now(),
	})
}

// Run implements ContainerRuntime.Run.
func (r *RecordingRuntime) Run(ctx context.Context, step *schema.RunStep, _ *ocw.Scope) (*ocw.StepResult, error) {
	r.record(step.Name, "run", "start")
	select {
	case <-time.After(r.Delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.record(step.Name, "run", "end")
	return &ocw.StepResult{
		ID:     step.ID,
		Status: ocw.StatusSuccess,
		Output: ocw.StepOutput{Values: make(map[string]string)},
	}, nil
}

// Build implements ContainerRuntime.Build.
func (r *RecordingRuntime) Build(ctx context.Context, step *schema.BuildStep, _ *ocw.Scope) (*ocw.StepResult, error) {
	r.record(step.Name, "build", "start")
	select {
	case <-time.After(r.Delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.record(step.Name, "build", "end")
	return &ocw.StepResult{
		ID:     step.ID,
		Status: ocw.StatusSuccess,
		Output: ocw.StepOutput{Values: map[string]string{"image": step.Build.Image}},
	}, nil
}

// StartService implements ContainerRuntime.StartService.
func (r *RecordingRuntime) StartService(_ context.Context, step *schema.RunStep, _ *ocw.Scope) (*ocw.ServiceHandle, error) {
	r.mu.Lock()
	r.nextID++
	cid := fmt.Sprintf("rec-container-%d", r.nextID)
	r.Services = append(r.Services, RecordedService{Name: step.Name, Image: step.Image, ContainerID: cid})
	r.mu.Unlock()

	r.record(step.Name, "service-start", "start")
	r.record(step.Name, "service-start", "end")

	return &ocw.ServiceHandle{
		ID:          step.ID,
		Name:        step.Name,
		ContainerID: cid,
	}, nil
}

// StopService implements ContainerRuntime.StopService.
func (r *RecordingRuntime) StopService(_ context.Context, handle *ocw.ServiceHandle) error {
	r.mu.Lock()
	r.Stopped = append(r.Stopped, handle.ContainerID)
	r.mu.Unlock()
	r.record(handle.Name, "service-stop", "start")
	r.record(handle.Name, "service-stop", "end")
	return nil
}

// CheckHealth implements ContainerRuntime.CheckHealth.
func (r *RecordingRuntime) CheckHealth(_ context.Context, _ *ocw.ServiceHandle, _ *schema.HealthCheck) error {
	return nil
}

// EventsFor returns all events matching a given step name.
func (r *RecordingRuntime) EventsFor(name string) []StepEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []StepEvent
	for _, e := range r.Events {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}

// StartTime returns the timestamp of the first "start" event for a step.
func (r *RecordingRuntime) StartTime(name string) time.Time {
	for _, e := range r.EventsFor(name) {
		if e.Phase == "start" {
			return e.Timestamp
		}
	}
	return time.Time{}
}

// EndTime returns the timestamp of the last "end" event for a step.
func (r *RecordingRuntime) EndTime(name string) time.Time {
	var last time.Time
	for _, e := range r.EventsFor(name) {
		if e.Phase == "end" {
			last = e.Timestamp
		}
	}
	return last
}

// RunNames returns the names of all "run" events in start order.
func (r *RecordingRuntime) RunNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	seen := make(map[string]bool)
	for _, e := range r.Events {
		if e.Type == "run" && e.Phase == "start" && !seen[e.Name] {
			names = append(names, e.Name)
			seen[e.Name] = true
		}
	}
	return names
}

// BuildNames returns the names of all "build" events in start order.
func (r *RecordingRuntime) BuildNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	seen := make(map[string]bool)
	for _, e := range r.Events {
		if e.Type == "build" && e.Phase == "start" && !seen[e.Name] {
			names = append(names, e.Name)
			seen[e.Name] = true
		}
	}
	return names
}
