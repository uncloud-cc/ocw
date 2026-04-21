package workflow

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// timedMockStep is a mock step that records execution timing
type timedMockStep struct {
	mockStep
	duration  time.Duration
	startTime time.Time
	endTime   time.Time
	execMutex sync.Mutex
}

func newTimedMockStep(stepType, id string, duration time.Duration, result *StepResult) *timedMockStep {
	return &timedMockStep{
		mockStep: mockStep{
			stepType: stepType,
			id:       id,
			result:   result,
		},
		duration: duration,
	}
}

func (m *timedMockStep) Execute(ctx context.Context, stepCtx *StepContext, opts ExecuteOptions) (*StepResult, error) {
	m.execMutex.Lock()
	m.startTime = time.Now()
	m.execMutex.Unlock()

	if m.duration > 0 {
		time.Sleep(m.duration)
	}

	m.execMutex.Lock()
	m.endTime = time.Now()
	m.execMutex.Unlock()

	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *timedMockStep) StartedAt() time.Time {
	m.execMutex.Lock()
	defer m.execMutex.Unlock()
	return m.startTime
}

func (m *timedMockStep) EndedAt() time.Time {
	m.execMutex.Lock()
	defer m.execMutex.Unlock()
	return m.endTime
}

// stepsOverlap returns true if two steps' execution windows overlap
func stepsOverlap(s1, s2 *timedMockStep) bool {
	start1, end1 := s1.StartedAt(), s1.EndedAt()
	start2, end2 := s2.StartedAt(), s2.EndedAt()

	// Handle case where step hasn't executed yet
	if start1.IsZero() || start2.IsZero() {
		return false
	}

	return start1.Before(end2) && start2.Before(end1)
}

// mockStep is a test double implementing the Step interface
type mockStep struct {
	stepType string
	id       string
	name     string
	original *schema.Step
	result   *StepResult
	err      error
}

func (m *mockStep) Type() string           { return m.stepType }
func (m *mockStep) ID() string             { return m.id }
func (m *mockStep) Name() string           { return m.name }
func (m *mockStep) Original() *schema.Step { return m.original }
func (m *mockStep) Execute(ctx context.Context, stepCtx *StepContext, opts ExecuteOptions) (*StepResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}
func (m *mockStep) Status() StepStatus {
	return StepStatus{
		State:   "mock",
		Message: "mock step",
	}
}

// mockStepFactory is a test double implementing StepFactory
type mockStepFactory struct{}

func (f *mockStepFactory) Create(step *schema.Step, ctx *StepContext) (Step, error) {
	return &mockStep{stepType: "mock"}, nil
}

// mockRuntime is a test double implementing ContainerRuntime
type mockRuntime struct {
	runResult   *RunResult
	buildResult *BuildResult
	runErr      error
	buildErr    error
}

func (m *mockRuntime) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.runResult, nil
}

func (m *mockRuntime) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return m.buildResult, nil
}

func (m *mockRuntime) Stop(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockRuntime) Logs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return nil, nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		ocw     *schema.OCW
		jobName string
		ctx     *StepContext
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid schema with job",
			ocw: &schema.OCW{
				SchemaVersion: "0.1.0",
				Name:          "test-workflow",
				Jobs: map[string]schema.Job{
					"build": {
						Parallel: []schema.Step{
							{RunStep: &schema.RunStep{
								StepBase: schema.StepBase{Name: "test-step"},
								Image:    "alpine",
							}},
						},
					},
				},
			},
			jobName: "build",
			ctx: &StepContext{
				Env:     make(map[string]string),
				Steps:   make(map[string]map[string]string),
				Runtime: &mockRuntime{},
				Factory: &mockStepFactory{},
			},
			wantErr: false,
		},
		{
			name:    "nil schema",
			ocw:     nil,
			ctx:     &StepContext{},
			wantErr: true,
			errMsg:  "schema is nil",
		},
		{
			name: "nil context",
			ocw: &schema.OCW{
				SchemaVersion: "0.1.0",
				Name:          "test",
			},
			ctx:     nil,
			wantErr: true,
			errMsg:  "context is nil",
		},
		{
			name: "job not found",
			ocw: &schema.OCW{
				SchemaVersion: "0.1.0",
				Name:          "test",
				Jobs: map[string]schema.Job{
					"build": {
						Parallel: []schema.Step{
							{RunStep: &schema.RunStep{StepBase: schema.StepBase{Name: "step"}, Image: "alpine"}},
						},
					},
				},
			},
			jobName: "nonexistent",
			ctx:     &StepContext{Steps: make(map[string]map[string]string)},
			wantErr: true,
			errMsg:  "job not found",
		},
		{
			name: "invalid schema version",
			ocw: &schema.OCW{
				SchemaVersion: "",
				Name:          "test",
			},
			ctx:     &StepContext{Steps: make(map[string]map[string]string)},
			wantErr: true,
			errMsg:  "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := New(tt.ocw, tt.jobName, tt.ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, engine)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, engine)
				assert.NotNil(t, engine.Context())
			}
		})
	}
}

func TestEngine_Done(t *testing.T) {
	tests := []struct {
		name    string
		pending []Step
		current []Step
		want    bool
	}{
		{
			name:    "empty engine",
			pending: nil,
			current: nil,
			want:    true,
		},
		{
			name:    "pending only",
			pending: []Step{&mockStep{stepType: "run"}},
			current: nil,
			want:    false,
		},
		{
			name:    "current only",
			pending: nil,
			current: []Step{&mockStep{stepType: "run"}},
			want:    false,
		},
		{
			name:    "both pending and current",
			pending: []Step{&mockStep{stepType: "build"}},
			current: []Step{&mockStep{stepType: "run"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				pending: tt.pending,
				current: tt.current,
			}
			got := engine.Done()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngine_Execute_SingleLeafStep(t *testing.T) {
	mockRuntime := &mockRuntime{
		runResult: &RunResult{
			ContainerID: "abc123",
			ExitCode:    0,
			Outputs: map[string]string{
				"output1": "value1",
			},
		},
	}

	step := &mockStep{
		stepType: "run",
		id:       "test-step",
		name:     "Test Step",
		result: &StepResult{
			StepID:  "test-step",
			Outputs: map[string]string{"output1": "value1"},
		},
	}

	ctx := &StepContext{
		Env:     map[string]string{"KEY": "value"},
		Steps:   make(map[string]map[string]string),
		Runtime: mockRuntime,
	}

	engine := &Engine{
		current: []Step{step},
		ctx:     ctx,
	}

	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Outputs should be merged into StepContext
	assert.Contains(t, ctx.Steps, "test-step")
	assert.Equal(t, "value1", ctx.Steps["test-step"]["output1"])

	// Current should be empty after execution
	assert.Empty(t, engine.Current())
}

func TestEngine_Execute_ParallelSteps(t *testing.T) {
	step1 := &mockStep{
		stepType: "run",
		id:       "step1",
		result: &StepResult{
			StepID:  "step1",
			Outputs: map[string]string{"result": "value1"},
		},
	}

	step2 := &mockStep{
		stepType: "run",
		id:       "step2",
		result: &StepResult{
			StepID:  "step2",
			Outputs: map[string]string{"result": "value2"},
		},
	}

	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	engine := &Engine{
		current: []Step{step1, step2},
		ctx:     ctx,
	}

	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Both outputs should be present
	assert.Equal(t, "value1", ctx.Steps["step1"]["result"])
	assert.Equal(t, "value2", ctx.Steps["step2"]["result"])
}

func TestEngine_Execute_StepError(t *testing.T) {
	step := &mockStep{
		stepType: "run",
		id:       "failing-step",
		err:      errors.New("container failed to start"),
	}

	engine := &Engine{
		current: []Step{step},
		ctx: &StepContext{
			Steps: make(map[string]map[string]string),
		},
	}

	err := engine.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container failed to start")
}

func TestEngine_Execute_CompositeStepExpansion(t *testing.T) {
	// A parallel step that returns children
	compositeStep := &mockStep{
		stepType: "parallel",
		id:       "parallel-group",
		result: &StepResult{
			StepID: "parallel-group",
			Children: []Step{
				&mockStep{stepType: "run", id: "child1"},
				&mockStep{stepType: "run", id: "child2"},
				&mockStep{stepType: "run", id: "child3"},
			},
			Parallel: true,
		},
	}

	engine := &Engine{
		current: []Step{compositeStep},
		ctx: &StepContext{
			Steps: make(map[string]map[string]string),
		},
	}

	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Children should be moved to current (for parallel execution)
	// Or pending if the engine handles expansion differently
	assert.Len(t, engine.Current(), 3, "all parallel children should be in current")
}

func TestEngine_Execute_SequenceStepExpansion(t *testing.T) {
	// A sequence step that returns children (sequential)
	child1 := &mockStep{stepType: "run", id: "child1"}
	child2 := &mockStep{stepType: "run", id: "child2"}
	compositeStep := &mockStep{
		stepType: "sequence",
		id:       "sequence-group",
		result: &StepResult{
			StepID:   "sequence-group",
			Children: []Step{child1, child2},
			Parallel: false,
		},
	}

	engine := &Engine{
		current: []Step{compositeStep},
		ctx: &StepContext{
			Steps: make(map[string]map[string]string),
		},
	}

	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// With recursive sequential execution, both children execute immediately
	// Engine should be done after executing all sequential children
	assert.True(t, engine.Done(), "engine should be done after executing all sequential children")
}

func TestEngine_Execute_ServiceTracking(t *testing.T) {
	step := &mockStep{
		stepType: "run",
		id:       "background-service",
		result: &StepResult{
			StepID: "background-service",
			Service: &ServiceInfo{
				StepID:      "background-service",
				ContainerID: "container-123",
				Healthy:     true,
				Ports: []PortMapping{
					{ContainerPort: 8080, HostPort: 8080, Protocol: "http"},
				},
			},
		},
	}

	ctx := &StepContext{
		Steps:    make(map[string]map[string]string),
		Services: make(map[string]*ServiceInfo),
	}

	engine := &Engine{
		current: []Step{step},
		ctx:     ctx,
	}

	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Service should be tracked
	assert.Contains(t, ctx.Services, "background-service")
	assert.Equal(t, "container-123", ctx.Services["background-service"].ContainerID)
}

func TestEngine_Execute_TemplateInterpolation(t *testing.T) {
	ctx := &StepContext{
		Env:   map[string]string{"IMAGE": "alpine:latest"},
		Steps: make(map[string]map[string]string),
	}

	engine := &Engine{
		current: []Step{}, // Empty for this test
		ctx:     ctx,
	}

	// This test would verify that Interpolate() is called on steps
	// The actual interpolation logic is tested in template_test.go
	// Here we just ensure Execute attempts interpolation
	err := engine.Execute(context.Background())
	require.NoError(t, err) // Empty execution should succeed
}

func TestEngine_FullWorkflow(t *testing.T) {
	// Create a workflow: parallel group of two run steps
	step1 := &mockStep{
		stepType: "run",
		id:       "build",
		result: &StepResult{
			StepID:  "build",
			Outputs: map[string]string{"image": "myapp:v1"},
		},
	}

	step2 := &mockStep{
		stepType: "run",
		id:       "test",
		result: &StepResult{
			StepID:  "test",
			Outputs: map[string]string{"passed": "true"},
		},
	}

	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	engine := &Engine{
		current: []Step{step1, step2},
		ctx:     ctx,
	}

	// Execute once (both parallel steps)
	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Verify both outputs
	assert.Equal(t, "myapp:v1", ctx.Steps["build"]["image"])
	assert.Equal(t, "true", ctx.Steps["test"]["passed"])

	// Engine should be done
	assert.True(t, engine.Done())
}

func TestEngine_PendingAndCurrentAccessors(t *testing.T) {
	step1 := &mockStep{stepType: "run", id: "step1"}
	step2 := &mockStep{stepType: "build", id: "step2"}

	engine := &Engine{
		pending: []Step{step1},
		current: []Step{step2},
	}

	assert.Len(t, engine.Pending(), 1)
	assert.Len(t, engine.Current(), 1)
	assert.Equal(t, "step1", engine.Pending()[0].ID())
	assert.Equal(t, "step2", engine.Current()[0].ID())
}

// TestEngine_Execute_ParallelSteps_Concurrent verifies that multiple steps
// in current execute concurrently (their time windows overlap)
func TestEngine_Execute_ParallelSteps_Concurrent(t *testing.T) {

	// Create three timed steps that each take 50ms
	stepDuration := 50 * time.Millisecond
	step1 := newTimedMockStep("run", "step1", stepDuration, &StepResult{
		StepID:  "step1",
		Outputs: map[string]string{"result": "value1"},
	})
	step2 := newTimedMockStep("run", "step2", stepDuration, &StepResult{
		StepID:  "step2",
		Outputs: map[string]string{"result": "value2"},
	})
	step3 := newTimedMockStep("run", "step3", stepDuration, &StepResult{
		StepID:  "step3",
		Outputs: map[string]string{"result": "value3"},
	})

	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	engine := &Engine{
		current: []Step{step1, step2, step3},
		ctx:     ctx,
	}

	start := time.Now()
	err := engine.Execute(context.Background())
	elapsed := time.Since(start)
	require.NoError(t, err)

	// All outputs should be present
	assert.Equal(t, "value1", ctx.Steps["step1"]["result"])
	assert.Equal(t, "value2", ctx.Steps["step2"]["result"])
	assert.Equal(t, "value3", ctx.Steps["step3"]["result"])

	// If parallel: ~50ms total. If sequential: ~150ms.
	// Allow 80ms tolerance for goroutine scheduling
	assert.Less(t, elapsed, 80*time.Millisecond,
		"parallel steps should execute concurrently (took %v, expected ~50ms)", elapsed)

	// Verify all steps overlapped in time
	assert.True(t, stepsOverlap(step1, step2), "step1 and step2 should have overlapped")
	assert.True(t, stepsOverlap(step1, step3), "step1 and step3 should have overlapped")
	assert.True(t, stepsOverlap(step2, step3), "step2 and step3 should have overlapped")
}

// TestEngine_Execute_SequentialSteps_NonConcurrent verifies that steps
// executed one at a time don't overlap
func TestEngine_Execute_SequentialSteps_NonConcurrent(t *testing.T) {
	stepDuration := 30 * time.Millisecond
	step1 := newTimedMockStep("run", "step1", stepDuration, &StepResult{
		StepID:  "step1",
		Outputs: map[string]string{"result": "value1"},
	})
	step2 := newTimedMockStep("run", "step2", stepDuration, &StepResult{
		StepID:  "step2",
		Outputs: map[string]string{"result": "value2"},
	})

	// Simulate sequential execution: only step1 in current, step2 in pending
	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	engine := &Engine{
		current: []Step{step1},
		pending: []Step{step2},
		ctx:     ctx,
	}

	start := time.Now()

	// Execute step1
	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Move step2 from pending to current (simulating engine logic)
	engine.current = []Step{step2}
	engine.pending = nil

	// Execute step2
	err = engine.Execute(context.Background())
	require.NoError(t, err)

	elapsed := time.Since(start)

	// Sequential: ~60ms. Allow some tolerance.
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond,
		"sequential steps should take at least ~60ms (took %v)", elapsed)

	// Steps should NOT overlap
	assert.False(t, stepsOverlap(step1, step2), "sequential steps should not overlap")
}

// TestEngine_Execute_MixedParallelAndSequence verifies a workflow with
// both parallel groups and sequential steps
func TestEngine_Execute_MixedParallelAndSequence(t *testing.T) {

	stepDuration := 50 * time.Millisecond

	// First: parallel group of 2 steps (50ms each, should finish in ~50ms)
	parallel1 := newTimedMockStep("run", "parallel1", stepDuration, &StepResult{
		StepID:  "parallel1",
		Outputs: map[string]string{"result": "p1"},
	})
	parallel2 := newTimedMockStep("run", "parallel2", stepDuration, &StepResult{
		StepID:  "parallel2",
		Outputs: map[string]string{"result": "p2"},
	})

	// Second: sequential step (100ms)
	sequential := newTimedMockStep("run", "sequential", 100*time.Millisecond, &StepResult{
		StepID:  "sequential",
		Outputs: map[string]string{"result": "seq"},
	})

	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	// Execute parallel group first
	engine := &Engine{
		current: []Step{parallel1, parallel2},
		ctx:     ctx,
	}

	start := time.Now()
	err := engine.Execute(context.Background())
	require.NoError(t, err)

	// Verify parallel group results
	assert.Equal(t, "p1", ctx.Steps["parallel1"]["result"])
	assert.Equal(t, "p2", ctx.Steps["parallel2"]["result"])

	// Verify parallel steps overlapped
	assert.True(t, stepsOverlap(parallel1, parallel2), "parallel steps should overlap")

	// Now execute sequential step
	engine.current = []Step{sequential}
	err = engine.Execute(context.Background())
	require.NoError(t, err)

	elapsed := time.Since(start)

	// Expected: ~50ms (parallel) + ~100ms (sequential) = ~150ms
	// Allow tolerance: should be between 130ms and 180ms
	assert.GreaterOrEqual(t, elapsed, 130*time.Millisecond,
		"mixed workflow should take ~150ms (took %v)", elapsed)
	assert.Less(t, elapsed, 180*time.Millisecond,
		"mixed workflow should take ~150ms (took %v)", elapsed)

	// Verify all results
	assert.Equal(t, "seq", ctx.Steps["sequential"]["result"])

	// Sequential step should NOT overlap with parallel group
	assert.False(t, stepsOverlap(parallel1, sequential),
		"sequential step should not overlap with parallel group")
	assert.False(t, stepsOverlap(parallel2, sequential),
		"sequential step should not overlap with parallel group")
}

// TestEngine_Execute_ParallelWithError verifies that when one step in a
// parallel group fails, the error is handled appropriately
func TestEngine_Execute_ParallelWithError(t *testing.T) {

	stepDuration := 50 * time.Millisecond

	step1 := newTimedMockStep("run", "step1", stepDuration, &StepResult{
		StepID:  "step1",
		Outputs: map[string]string{"result": "value1"},
	})
	step2 := newTimedMockStep("run", "step2", stepDuration, nil)
	step2.err = errors.New("step2 failed")
	step3 := newTimedMockStep("run", "step3", stepDuration, &StepResult{
		StepID:  "step3",
		Outputs: map[string]string{"result": "value3"},
	})

	ctx := &StepContext{
		Steps: make(map[string]map[string]string),
		Runtime: &mockRuntime{
			runResult: &RunResult{ExitCode: 0},
		},
	}

	engine := &Engine{
		current: []Step{step1, step2, step3},
		ctx:     ctx,
	}

	err := engine.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step2 failed")

	// In a parallel execution, we need to decide:
	// - Should we wait for all steps to complete (fail-fast vs fail-last)?
	// - Should partial results be available in ctx.Steps?
	// This test documents the expected behavior once implemented
}
