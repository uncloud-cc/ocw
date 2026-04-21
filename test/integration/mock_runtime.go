package integration

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/uncloud-cc/ocw/pkg/ocw/workflow"
)

// CallRecord tracks a single call to the mock runtime.
type CallRecord struct {
	Type   string // "run" or "build"
	Image  string
	Opts   interface{}
	Result interface{}
	Error  error
}

// MockRuntime is a test double for workflow.ContainerRuntime that records all calls.
type MockRuntime struct {
	mu      sync.RWMutex
	calls   []CallRecord
	runFn   func(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error)
	buildFn func(ctx context.Context, opts workflow.BuildOptions) (*workflow.BuildResult, error)
}

// NewMockRuntime creates a mock runtime with default behaviors.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		runFn: func(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error) {
			return &workflow.RunResult{
				ContainerID: fmt.Sprintf("container-%s", opts.Image),
				ExitCode:    0,
				Outputs:     map[string]string{},
			}, nil
		},
		buildFn: func(ctx context.Context, opts workflow.BuildOptions) (*workflow.BuildResult, error) {
			imageRef := "mock-image:latest"
			if len(opts.Tags) > 0 && opts.Tags[0] != "" {
				imageRef = opts.Tags[0]
			}
			return &workflow.BuildResult{
				ImageRef: imageRef,
				ImageID:  fmt.Sprintf("sha256:%s", imageRef),
			}, nil
		},
	}
}

// WithRunResult configures the mock to return a specific result for a run call matching the predicate.
func (m *MockRuntime) WithRunResult(image string, result *workflow.RunResult, err error) *MockRuntime {
	oldRunFn := m.runFn
	m.runFn = func(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error) {
		if opts.Image == image {
			return result, err
		}
		return oldRunFn(ctx, opts)
	}
	return m
}

// WithBuildResult configures the mock to return a specific result for a build call matching the predicate.
func (m *MockRuntime) WithBuildResult(imageRef string, result *workflow.BuildResult, err error) *MockRuntime {
	oldBuildFn := m.buildFn
	m.buildFn = func(ctx context.Context, opts workflow.BuildOptions) (*workflow.BuildResult, error) {
		for _, tag := range opts.Tags {
			if tag == imageRef {
				return result, err
			}
		}
		return oldBuildFn(ctx, opts)
	}
	return m
}

// Run implements workflow.ContainerRuntime.
func (m *MockRuntime) Run(ctx context.Context, opts workflow.RunOptions) (*workflow.RunResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, err := m.runFn(ctx, opts)
	m.calls = append(m.calls, CallRecord{
		Type:   "run",
		Image:  opts.Image,
		Opts:   opts,
		Result: result,
		Error:  err,
	})
	return result, err
}

// Build implements workflow.ContainerRuntime.
func (m *MockRuntime) Build(ctx context.Context, opts workflow.BuildOptions) (*workflow.BuildResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, err := m.buildFn(ctx, opts)
	image := ""
	if len(opts.Tags) > 0 {
		image = opts.Tags[0]
	}
	m.calls = append(m.calls, CallRecord{
		Type:   "build",
		Image:  image,
		Opts:   opts,
		Result: result,
		Error:  err,
	})
	return result, err
}

// Stop implements workflow.ContainerRuntime.
func (m *MockRuntime) Stop(ctx context.Context, containerID string) error {
	return nil
}

// Logs implements workflow.ContainerRuntime.
func (m *MockRuntime) Logs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return nil, nil
}

// Calls returns a copy of all recorded calls.
func (m *MockRuntime) Calls() []CallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copy := make([]CallRecord, len(m.calls))
	for i, c := range m.calls {
		copy[i] = c
	}
	return copy
}

// RunCalls returns only the recorded Run calls.
func (m *MockRuntime) RunCalls() []CallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var runs []CallRecord
	for _, c := range m.calls {
		if c.Type == "run" {
			runs = append(runs, c)
		}
	}
	return runs
}

// BuildCalls returns only the recorded Build calls.
func (m *MockRuntime) BuildCalls() []CallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var builds []CallRecord
	for _, c := range m.calls {
		if c.Type == "build" {
			builds = append(builds, c)
		}
	}
	return builds
}

// Reset clears all recorded calls.
func (m *MockRuntime) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}
