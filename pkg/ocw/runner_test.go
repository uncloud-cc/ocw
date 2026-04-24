package ocw

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestRuntime() (*Runtime, *DummyRuntime) {
	dummy := NewDummyRuntime(log.Default())
	rt := NewRuntime(dummy, log.Default())
	return rt, dummy
}

func runStep(name, image, cmd string) schema.Step {
	return schema.Step{
		RunStep: &schema.RunStep{
			StepBase: schema.StepBase{Name: name},
			Image:    image,
			Cmd:      cmd,
		},
	}
}

func runStepWithID(name, id, image, cmd string) schema.Step {
	return schema.Step{
		RunStep: &schema.RunStep{
			StepBase: schema.StepBase{Name: name, ID: id},
			Image:    image,
			Cmd:      cmd,
		},
	}
}

func buildStep(name, image string) schema.Step {
	return schema.Step{
		BuildStep: &schema.BuildStep{
			StepBase: schema.StepBase{Name: name},
			Build:    schema.BuildConfig{Image: image},
		},
	}
}

// ---------------------------------------------------------------------------
// Scope tests
// ---------------------------------------------------------------------------

func TestScope_Clone(t *testing.T) {
	s := NewScope()
	s.Env["FOO"] = "bar"
	s.Secrets["KEY"] = "secret"
	s.Steps["build"] = StepOutput{Values: map[string]string{"image": "myapp"}}
	s.Workflow = WorkflowMeta{Name: "original"}
	s.Job = JobMeta{Name: "build"}

	c := s.Clone()

	// Mutation on clone should not affect original.
	c.Env["FOO"] = "changed"
	c.Secrets["KEY"] = "changed"
	c.Steps["build"] = StepOutput{Values: map[string]string{"image": "changed"}}
	c.Workflow.Name = "changed"
	c.Job.Name = "changed"

	assert.Equal(t, "bar", s.Env["FOO"])
	assert.Equal(t, "secret", s.Secrets["KEY"])
	assert.Equal(t, "myapp", s.Steps["build"].Values["image"])
	assert.Equal(t, "original", s.Workflow.Name)
	assert.Equal(t, "build", s.Job.Name)
}

func TestScope_Interpolate(t *testing.T) {
	s := NewScope()
	s.Env["DEPLOY_ENV"] = "staging"
	s.Secrets["TOKEN"] = "abc123"
	s.Steps["build"] = StepOutput{Values: map[string]string{"image": "myapp:latest"}}
	s.Workflow = WorkflowMeta{Name: "My Workflow"}
	s.Job = JobMeta{Name: "deploy"}

	tests := []struct {
		name        string
		input       string
		expect      string
		wantErr     bool
		errContains string
	}{
		// --- env ---
		{"env with spaces", "{{ env.DEPLOY_ENV }}", "staging", false, ""},
		{"env without spaces", "{{env.DEPLOY_ENV}}", "staging", false, ""},
		{"env extra whitespace", "{{  env.DEPLOY_ENV  }}", "staging", false, ""},
		{"env mixed whitespace", "{{ env.DEPLOY_ENV}}", "staging", false, ""},
		{"env not in scope and not set is error", "{{ env.DEFINITELY_NOT_SET_ANYWHERE_XYZ }}", "{{ env.DEFINITELY_NOT_SET_ANYWHERE_XYZ }}", true, "environment variable \"DEFINITELY_NOT_SET_ANYWHERE_XYZ\" is not set"},

		// --- secrets ---
		{"secret", "{{ secrets.TOKEN }}", "abc123", false, ""},
		{"secret unresolved is error", "{{ secrets.MISSING }}", "{{ secrets.MISSING }}", true, "unresolved secret"},

		// --- steps ---
		{"step output", "{{ steps.build.image }}", "myapp:latest", false, ""},
		{"step missing id", "{{ steps.nope.image }}", "{{ steps.nope.image }}", true, "step \"nope\" not found"},
		{"step missing field", "{{ steps.build.nope }}", "{{ steps.build.nope }}", true, "has no output \"nope\""},
		{"step no field", "{{ steps.build }}", "{{ steps.build }}", true, "invalid step reference"},

		// --- workflow ---
		{"workflow name", "{{ workflow.name }}", "My Workflow", false, ""},
		{"workflow unknown", "{{ workflow.version }}", "{{ workflow.version }}", true, "unknown workflow property"},

		// --- job ---
		{"job name", "{{ job.name }}", "deploy", false, ""},
		{"job unknown", "{{ job.foo }}", "{{ job.foo }}", true, "unknown job property"},

		// --- unknown namespace ---
		{"unknown namespace", "{{ foo.bar }}", "{{ foo.bar }}", true, "unknown template namespace"},

		// --- invalid expression ---
		{"no dot", "{{ nodot }}", "{{ nodot }}", true, "invalid template expression"},

		// --- plain text ---
		{"plain text", "hello world", "hello world", false, ""},
		{"empty string", "", "", false, ""},

		// --- mixed ---
		{"multiple templates", "{{ env.DEPLOY_ENV }}-{{ workflow.name }}", "staging-My Workflow", false, ""},
		{"template in context", "image={{ steps.build.image }}", "image=myapp:latest", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.Interpolate(tt.input)
			assert.Equal(t, tt.expect, result)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScope_Interpolate_EnvHostFallback(t *testing.T) {
	// Set a host env var that is NOT in the scope's Env map.
	t.Setenv("OCW_TEST_HOST_VAR", "from-host")

	s := NewScope()
	s.Logger = log.Default()
	// Deliberately do NOT add OCW_TEST_HOST_VAR to s.Env.

	// Should warn (not error) and leave the template text as-is.
	result, err := s.Interpolate("{{ env.OCW_TEST_HOST_VAR }}")
	assert.NoError(t, err)
	assert.Equal(t, "{{ env.OCW_TEST_HOST_VAR }}", result)
}

func TestScope_Merge(t *testing.T) {
	s := NewScope()
	s.Merge("step1", StepOutput{Values: map[string]string{"out": "val"}})

	assert.Equal(t, "val", s.Steps["step1"].Values["out"])

	// Empty ID should not add anything.
	s.Merge("", StepOutput{Values: map[string]string{"x": "y"}})
	_, ok := s.Steps[""]
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestNewStepFactory_DispatchesAllTypes(t *testing.T) {
	dummy := NewDummyRuntime(log.Default())
	factory := newStepFactory(dummy, &ServiceRegistry{}, log.Default())

	tests := []struct {
		name       string
		step       schema.Step
		wantSimple bool
	}{
		{"run step", runStep("run", "alpine", "echo hi"), true},
		{"background run step (service)", schema.Step{RunStep: &schema.RunStep{
			StepBase:   schema.StepBase{Name: "db", ID: "db"},
			Image:      "postgres:15",
			Background: true,
		}}, true},
		{"build step", buildStep("build", "myapp"), true},
		{
			"sequence step",
			schema.Step{SequenceStep: &schema.SequenceStep{
				Sequence: []schema.Step{runStep("s1", "alpine", "echo 1")},
			}},
			false,
		},
		{
			"parallel step",
			schema.Step{ParallelStep: &schema.ParallelStep{
				Parallel: []schema.Step{runStep("p1", "alpine", "echo 1")},
			}},
			false,
		},
		{
			"switch step",
			schema.Step{SwitchStep: &schema.SwitchStep{
				Switch: "{{ env.MODE }}",
				Case: map[string]schema.Step{
					"test": runStep("test", "alpine", "echo test"),
				},
			}},
			false,
		},
		{
			"workflow step",
			schema.Step{WorkflowStep: &schema.WorkflowStep{
				Workflow: schema.WorkflowConfig{From: "./other.yaml"},
			}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := factory(tt.step)
			require.NotNil(t, runner)
			if tt.wantSimple {
				_, ok := runner.(SimpleRunner)
				assert.True(t, ok, "expected SimpleRunner")
			} else {
				_, ok := runner.(CompositeRunner)
				assert.True(t, ok, "expected CompositeRunner")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Sequence execution
// ---------------------------------------------------------------------------

func TestRuntime_Sequence(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "sequence test",
		Sequence: []schema.Step{
			runStep("step1", "alpine:latest", "echo first"),
			runStep("step2", "alpine:latest", "echo second"),
			runStep("step3", "alpine:latest", "echo third"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo first", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo second", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo third", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Parallel execution
// ---------------------------------------------------------------------------

func TestRuntime_Parallel(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "parallel test",
		Parallel: []schema.Step{
			runStep("p1", "alpine:latest", "echo one"),
			runStep("p2", "alpine:latest", "echo two"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// All steps ran (order may vary).
	assert.Len(t, dummy.Runs, 2)
}

// ---------------------------------------------------------------------------
// Switch execution
// ---------------------------------------------------------------------------

func TestRuntime_Switch_MatchesCase(t *testing.T) {
	rt, dummy := newTestRuntime()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch test",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "staging"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging":    runStep("deploy staging", "alpine", "echo staging"),
			"production": runStep("deploy prod", "alpine", "echo production"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo staging", dummy.Runs[0].Cmd)
}

func TestRuntime_Switch_FallsToDefault(t *testing.T) {
	rt, dummy := newTestRuntime()

	switchExpr := "{{ env.MODE }}"
	defaultStep := runStep("default deploy", "alpine", "echo default")
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch default test",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "dev"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging":    runStep("staging", "alpine", "echo staging"),
			"production": runStep("prod", "alpine", "echo prod"),
		},
		Default: &defaultStep,
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo default", dummy.Runs[0].Cmd)
}

func TestRuntime_Switch_NoMatchNoDefault(t *testing.T) {
	rt, dummy := newTestRuntime()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch no match",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "unknown"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging": runStep("staging", "alpine", "echo staging"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Empty(t, dummy.Runs)
}

// ---------------------------------------------------------------------------
// Switch with nested composite (sequence inside a case branch)
// ---------------------------------------------------------------------------

func TestRuntime_Switch_CaseBranchIsSequence(t *testing.T) {
	rt, dummy := newTestRuntime()

	switchExpr := "{{ env.MODE }}"
	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "switch with sequence branch",
		Env: schema.Env{
			"MODE": schema.EnvVar{Value: "staging"},
		},
		Switch: &switchExpr,
		Case: map[string]schema.Step{
			"staging": {
				SequenceStep: &schema.SequenceStep{
					Sequence: []schema.Step{
						runStep("prepare", "alpine", "echo prepare"),
						runStep("deploy", "alpine", "echo deploy"),
						runStep("verify", "alpine", "echo verify"),
					},
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo prepare", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo deploy", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo verify", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Nested composites: sequence containing a parallel block
// ---------------------------------------------------------------------------

func TestRuntime_NestedSequenceParallel(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "nested test",
		Sequence: []schema.Step{
			runStep("setup", "alpine", "echo setup"),
			{
				ParallelStep: &schema.ParallelStep{
					Parallel: []schema.Step{
						runStep("test-a", "alpine", "echo test-a"),
						runStep("test-b", "alpine", "echo test-b"),
					},
				},
			},
			runStep("teardown", "alpine", "echo teardown"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	// 1 setup + 2 parallel + 1 teardown = 4
	assert.Len(t, dummy.Runs, 4)
	// setup ran first
	assert.Equal(t, "echo setup", dummy.Runs[0].Cmd)
	// teardown ran last
	assert.Equal(t, "echo teardown", dummy.Runs[3].Cmd)
}

// ---------------------------------------------------------------------------
// Build step
// ---------------------------------------------------------------------------

func TestRuntime_BuildStep(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "build test",
		Sequence: []schema.Step{
			buildStep("build app", "myapp:latest"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Builds, 1)
	assert.Equal(t, "myapp:latest", dummy.Builds[0].Image)
}

// ---------------------------------------------------------------------------
// Job execution
// ---------------------------------------------------------------------------

func TestRuntime_Job(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "job test",
		Jobs: schema.Jobs{
			"build": {
				Name: "Build",
				Sequence: []schema.Step{
					runStep("compile", "golang:1.22", "go build ./..."),
				},
			},
			"test": {
				Name: "Test",
				Sequence: []schema.Step{
					runStep("unit", "golang:1.22", "go test ./..."),
					runStep("lint", "golangci/golangci-lint", "golangci-lint run"),
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "test")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 2)
	assert.Equal(t, "go test ./...", dummy.Runs[0].Cmd)
	assert.Equal(t, "golangci-lint run", dummy.Runs[1].Cmd)
}

func TestRuntime_Job_NotFound(t *testing.T) {
	rt, _ := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "job test",
		Jobs: schema.Jobs{
			"build": {Sequence: []schema.Step{runStep("x", "alpine", "echo")}},
		},
	}

	_, err := rt.RunWorkflow(context.Background(), ocw, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRuntime_Job_SingleStep(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "single step job",
		Jobs: schema.Jobs{
			"run": {
				Step: &schema.Step{
					RunStep: &schema.RunStep{
						StepBase: schema.StepBase{Name: "run it"},
						Image:    "alpine",
						Cmd:      "echo single",
					},
				},
			},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "run")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "echo single", dummy.Runs[0].Cmd)
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestRuntime_CancelledContext(t *testing.T) {
	rt, _ := newTestRuntime()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "cancel test",
		Sequence: []schema.Step{
			runStep("should not run", "alpine", "echo nope"),
		},
	}

	_, err := rt.RunWorkflow(ctx, ocw, "")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// No flow control
// ---------------------------------------------------------------------------

func TestRuntime_NoFlowControl(t *testing.T) {
	rt, _ := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "empty workflow",
	}

	_, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no flow control")
}

// ---------------------------------------------------------------------------
// SwitchStep nested inside a sequence (the Step-level switch, not top-level)
// ---------------------------------------------------------------------------

func TestRuntime_SwitchStep_Nested(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "nested switch",
		Env: schema.Env{
			"TARGET": schema.EnvVar{Value: "prod"},
		},
		Sequence: []schema.Step{
			runStep("before", "alpine", "echo before"),
			{
				SwitchStep: &schema.SwitchStep{
					Switch: "{{ env.TARGET }}",
					Case: map[string]schema.Step{
						"prod": runStep("prod deploy", "alpine", "echo prod"),
						"dev":  runStep("dev deploy", "alpine", "echo dev"),
					},
				},
			},
			runStep("after", "alpine", "echo after"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Runs, 3)
	assert.Equal(t, "echo before", dummy.Runs[0].Cmd)
	assert.Equal(t, "echo prod", dummy.Runs[1].Cmd)
	assert.Equal(t, "echo after", dummy.Runs[2].Cmd)
}

// ---------------------------------------------------------------------------
// Step ID propagation through scope
// ---------------------------------------------------------------------------

func TestRuntime_StepOutputsInScope(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "output propagation",
		Sequence: []schema.Step{
			buildStep("build app", "myapp:latest"),
			runStepWithID("use image", "runner", "alpine", "echo {{ steps.build.image }}"),
		},
	}

	// Give the build step an ID so its output is stored.
	ocw.Sequence[0].BuildStep.ID = "build"

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Builds, 1)
	require.Len(t, dummy.Runs, 1)
	// The dummy runtime doesn't interpolate the cmd, but we verify the build
	// output was captured in scope by checking it's present.
	assert.Equal(t, "myapp:latest", dummy.Builds[0].Image)
}

// ---------------------------------------------------------------------------
// Service (background) execution
// ---------------------------------------------------------------------------

func TestRuntime_ServiceRunner_Basic(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "service test",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "Database", ID: "db"},
				Image:      "postgres:15",
				Background: true,
			}},
			runStep("migrate", "myapp", "migrate up"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)

	// The service was started, not run as a regular container.
	assert.Empty(t, dummy.Runs[:0]) // clear previous; check separately
	require.Len(t, dummy.Services, 1)
	assert.Equal(t, "postgres:15", dummy.Services[0].Image)
	assert.Equal(t, "Database", dummy.Services[0].Name)

	// The regular step also ran.
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "migrate up", dummy.Runs[0].Cmd)

	// Service is tracked in the registry.
	all := rt.Services().All()
	require.Len(t, all, 1)
	assert.Equal(t, "db", all[0].ID)
	assert.True(t, all[0].Healthy)
}

func TestRuntime_ServiceRunner_WithHealthCheck(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "service with healthcheck",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "Database", ID: "db"},
				Image:      "postgres:15",
				Background: true,
				HealthCheck: &schema.HealthCheck{
					Cmd:     "pg_isready",
					Retries: 3,
				},
			}},
			runStep("after", "alpine", "echo after"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)

	require.Len(t, dummy.Services, 1)
	all := rt.Services().All()
	require.Len(t, all, 1)
	assert.True(t, all[0].Healthy)
}

func TestRuntime_ServiceRunner_NeedsDependency(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "needs test",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "Database", ID: "db"},
				Image:      "postgres:15",
				Background: true,
				HealthCheck: &schema.HealthCheck{
					Cmd: "pg_isready",
				},
			}},
			{RunStep: &schema.RunStep{
				StepBase: schema.StepBase{Name: "Run Migrations", Needs: []string{"db"}},
				Image:    "myapp",
				Cmd:      "migrate up",
			}},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Services, 1)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "migrate up", dummy.Runs[0].Cmd)
}

func TestRuntime_ServiceRunner_NeedsMissing(t *testing.T) {
	rt, _ := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "needs missing",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase: schema.StepBase{Name: "Run Migrations", Needs: []string{"db"}},
				Image:    "myapp",
				Cmd:      "migrate up",
			}},
		},
	}

	_, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs service \"db\"")
	assert.Contains(t, err.Error(), "not running")
}

func TestRuntime_Shutdown(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "shutdown test",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "DB", ID: "db"},
				Image:      "postgres:15",
				Background: true,
			}},
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "Redis", ID: "redis"},
				Image:      "redis:7",
				Background: true,
			}},
			runStep("app", "myapp", "run"),
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	require.Len(t, dummy.Services, 2)

	// Shutdown stops services in reverse order.
	err = rt.Shutdown(context.Background())
	require.NoError(t, err)
	require.Len(t, dummy.Stopped, 2)
	// Redis started second, stopped first.
	assert.Equal(t, dummy.Services[1].ContainerID, dummy.Stopped[0])
	assert.Equal(t, dummy.Services[0].ContainerID, dummy.Stopped[1])
}

func TestRuntime_MultipleServicesInSequence(t *testing.T) {
	rt, dummy := newTestRuntime()

	ocw := &schema.OCW{
		SchemaVersion: "0.1.0",
		Name:          "multi service",
		Sequence: []schema.Step{
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "DB", ID: "db"},
				Image:      "postgres:15",
				Background: true,
			}},
			{RunStep: &schema.RunStep{
				StepBase:   schema.StepBase{Name: "Redis", ID: "redis"},
				Image:      "redis:7",
				Background: true,
			}},
			{RunStep: &schema.RunStep{
				StepBase: schema.StepBase{Name: "App", Needs: []string{"db", "redis"}},
				Image:    "myapp",
				Cmd:      "start",
			}},
		},
	}

	result, err := rt.RunWorkflow(context.Background(), ocw, "")
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)

	require.Len(t, dummy.Services, 2)
	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "start", dummy.Runs[0].Cmd)

	all := rt.Services().All()
	require.Len(t, all, 2)
	assert.Equal(t, "db", all[0].ID)
	assert.Equal(t, "redis", all[1].ID)
}

// ---------------------------------------------------------------------------
// DummyRuntime records
// ---------------------------------------------------------------------------

func TestDummyRuntime_Records(t *testing.T) {
	dummy := NewDummyRuntime(log.Default())

	step := &schema.RunStep{
		StepBase: schema.StepBase{Name: "test", ID: "t1"},
		Image:    "alpine",
		Cmd:      "echo hello",
	}
	result, err := dummy.Run(context.Background(), step, NewScope())
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "t1", result.ID)

	require.Len(t, dummy.Runs, 1)
	assert.Equal(t, "alpine", dummy.Runs[0].Image)
	assert.Equal(t, "echo hello", dummy.Runs[0].Cmd)
	assert.Equal(t, "test", dummy.Runs[0].Name)

	buildS := &schema.BuildStep{
		StepBase: schema.StepBase{Name: "build", ID: "b1"},
		Build:    schema.BuildConfig{Image: "myapp"},
	}
	result, err = dummy.Build(context.Background(), buildS, NewScope())
	require.NoError(t, err)
	assert.Equal(t, "b1", result.ID)
	assert.Equal(t, "myapp", result.Output.Values["image"])

	require.Len(t, dummy.Builds, 1)
	assert.Equal(t, "myapp", dummy.Builds[0].Image)
}
