package integration_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
	"github.com/uncloud-cc/ocw/test/helpers"
)

// parseFixture loads a workflow file from testdata.
func parseFixture(t *testing.T, filename string) *schema.OCW {
	t.Helper()
	w, err := ocw.ParseFile("testdata/" + filename)
	require.NoError(t, err, "failed to parse %s", filename)
	require.NotNil(t, w)
	return w
}

// newEngine creates a new engine with the recording runtime.
func newEngine(rec *testhelpers.RecordingRuntime) *ocw.Engine {
	return ocw.NewEngine(rec, log.New(log.Writer(), "", 0))
}

func TestIntegration_HelloWorld(t *testing.T) {
	w := parseFixture(t, "hello_world.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 1)
	assert.Equal(t, "hello", names[0])
}

func TestIntegration_Build(t *testing.T) {
	w := parseFixture(t, "build.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// A build event was recorded, not a run event.
	builds := rec.BuildNames()
	require.Len(t, builds, 1)
	assert.Equal(t, "Build the container", builds[0])
	assert.Empty(t, rec.RunNames())
}

func TestIntegration_BuildAndRun(t *testing.T) {
	w := parseFixture(t, "build_and_run.yaml")
	rec := testhelpers.NewRecordingRuntime(50 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// One build and one run, in that order.
	builds := rec.BuildNames()
	require.Len(t, builds, 1)
	assert.Equal(t, "Build the container", builds[0])

	runs := rec.RunNames()
	require.Len(t, runs, 1)
	assert.Equal(t, "Run the container", runs[0])

	// Build finished before run started.
	testhelpers.AssertSequentialOrder(t, rec, []string{"Build the container", "Run the container"})
}

func TestIntegration_Sequence(t *testing.T) {
	w := parseFixture(t, "sequence.yaml")
	rec := testhelpers.NewRecordingRuntime(50 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 2)
	assert.Equal(t, []string{"Step A", "Step B"}, names)

	// Verify sequential: A ended before B started.
	testhelpers.AssertSequentialOrder(t, rec, []string{"Step A", "Step B"})
}

func TestIntegration_Parallel(t *testing.T) {
	w := parseFixture(t, "parallel.yaml")
	rec := testhelpers.NewRecordingRuntime(100 * time.Millisecond)
	rt := newEngine(rec)

	start := time.Now()
	result, err := rt.RunWorkflow(context.Background(), w, "")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 3)

	// All three steps should have overlapping execution windows.
	testhelpers.AssertParallelOverlap(t, rec, []string{"Step A", "Step B", "Step C"})

	// Wall-clock time should be closer to 1x delay than 3x delay.
	// With 100ms delay and 3 parallel steps: sequential would take ~300ms,
	// parallel should take ~100ms. We allow generous margin for CI.
	assert.Less(t, elapsed, 250*time.Millisecond,
		"parallel execution took %v, expected ~100ms (not 300ms sequential)", elapsed)
}

func TestIntegration_Nested(t *testing.T) {
	w := parseFixture(t, "nested.yaml")
	rec := testhelpers.NewRecordingRuntime(50 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// All 6 leaf steps ran (Setup + 3 parallel + Build + Deploy).
	names := rec.RunNames()
	require.Len(t, names, 6)

	// Setup ran first.
	assert.Equal(t, "Setup", names[0])

	// Setup ended before any parallel step started.
	setupEnd := rec.EndTime("Setup")
	for _, pName := range []string{"Unit Tests", "Integration Tests", "Lint"} {
		pStart := rec.StartTime(pName)
		assert.True(t, setupEnd.Before(pStart) || setupEnd.Equal(pStart),
			"Setup should finish before %s starts", pName)
	}

	// The three test steps ran in parallel.
	testhelpers.AssertParallelOverlap(t, rec, []string{"Unit Tests", "Integration Tests", "Lint"})

	// All parallel steps ended before Build started.
	for _, pName := range []string{"Unit Tests", "Integration Tests", "Lint"} {
		pEnd := rec.EndTime(pName)
		buildStart := rec.StartTime("Build")
		assert.True(t, pEnd.Before(buildStart) || pEnd.Equal(buildStart),
			"%s should finish before Build starts", pName)
	}

	// Build ended before Deploy started.
	testhelpers.AssertSequentialOrder(t, rec, []string{"Build", "Deploy"})
}

func TestIntegration_Switch_Staging(t *testing.T) {
	w := parseFixture(t, "switch.yaml")
	// Seed the env so the switch expression resolves.
	w.Env = schema.Env{"DEPLOY_ENV": schema.EnvVar{Value: "staging"}}

	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 1)
	assert.Equal(t, "Deploy to Staging", names[0])
}

func TestIntegration_Switch_Production(t *testing.T) {
	w := parseFixture(t, "switch.yaml")
	w.Env = schema.Env{"DEPLOY_ENV": schema.EnvVar{Value: "production"}}

	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// Production branch is a sequence of two steps.
	names := rec.RunNames()
	require.Len(t, names, 2)
	assert.Equal(t, "Deploy to Production", names[0])
	assert.Equal(t, "Notify Team", names[1])
	testhelpers.AssertSequentialOrder(t, rec, names)
}

func TestIntegration_Switch_Default(t *testing.T) {
	w := parseFixture(t, "switch.yaml")
	w.Env = schema.Env{"DEPLOY_ENV": schema.EnvVar{Value: "something-else"}}

	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 1)
	assert.Equal(t, "Deploy to Development", names[0])
}

func TestIntegration_Jobs_Build(t *testing.T) {
	w := parseFixture(t, "jobs.yaml")
	rec := testhelpers.NewRecordingRuntime(50 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "build")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 2)
	assert.Equal(t, []string{"Install", "Build"}, names)
	testhelpers.AssertSequentialOrder(t, rec, names)
}

func TestIntegration_Jobs_Test(t *testing.T) {
	w := parseFixture(t, "jobs.yaml")
	rec := testhelpers.NewRecordingRuntime(100 * time.Millisecond)
	rt := newEngine(rec)

	start := time.Now()
	result, err := rt.RunWorkflow(context.Background(), w, "test")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 2)
	testhelpers.AssertParallelOverlap(t, rec, []string{"Unit Tests", "Lint"})

	// Should take ~1x delay not 2x.
	assert.Less(t, elapsed, 180*time.Millisecond,
		"parallel job took %v, expected ~100ms", elapsed)
}

func TestIntegration_Jobs_Dev(t *testing.T) {
	w := parseFixture(t, "jobs.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "dev")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	names := rec.RunNames()
	require.Len(t, names, 1)
	assert.Equal(t, "Start Dev", names[0])
}

func TestIntegration_Jobs_NotFound(t *testing.T) {
	w := parseFixture(t, "jobs.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	_, err := rt.RunWorkflow(context.Background(), w, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIntegration_ServiceBasic(t *testing.T) {
	w := parseFixture(t, "service_basic.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// Redis was started as a service, not as a regular run.
	require.Len(t, rec.Services, 1)
	assert.Equal(t, "redis:7-alpine", rec.Services[0].Image)
	assert.Equal(t, "Start Redis", rec.Services[0].Name)

	// The dependent step ran.
	names := rec.RunNames()
	require.Len(t, names, 1)
	assert.Equal(t, "Use Redis", names[0])

	// The service started before the dependent step.
	svcEnd := rec.EndTime("Start Redis")
	runStart := rec.StartTime("Use Redis")
	assert.True(t, svcEnd.Before(runStart) || svcEnd.Equal(runStart))

	// Service is in the registry.
	all := rt.Services().All()
	require.Len(t, all, 1)
	assert.Equal(t, "redis", all[0].ID)
	assert.True(t, all[0].Healthy)

	// Shutdown cleans up.
	err = rt.Shutdown(context.Background())
	require.NoError(t, err)
	require.Len(t, rec.Stopped, 1)
}

func TestIntegration_ServiceHealthCheck(t *testing.T) {
	w := parseFixture(t, "service_healthcheck.yaml")
	rec := testhelpers.NewRecordingRuntime(10 * time.Millisecond)
	rt := newEngine(rec)

	result, err := rt.RunWorkflow(context.Background(), w, "")
	require.NoError(t, err)
	assert.Equal(t, ocw.StatusSuccess, result.Status)

	// Two services started.
	require.Len(t, rec.Services, 2)
	assert.Equal(t, "Database", rec.Services[0].Name)
	assert.Equal(t, "Cache", rec.Services[1].Name)

	// Two regular steps ran.
	names := rec.RunNames()
	require.Len(t, names, 2)
	assert.Equal(t, "Run Migrations", names[0])
	assert.Equal(t, "Run Application", names[1])

	// Both services are healthy.
	all := rt.Services().All()
	require.Len(t, all, 2)
	for _, svc := range all {
		assert.True(t, svc.Healthy, "service %q should be healthy", svc.Name)
	}

	// Shutdown stops in reverse order (Cache first, then Database).
	err = rt.Shutdown(context.Background())
	require.NoError(t, err)
	require.Len(t, rec.Stopped, 2)
	assert.Equal(t, rec.Services[1].ContainerID, rec.Stopped[0], "Cache should stop first")
	assert.Equal(t, rec.Services[0].ContainerID, rec.Stopped[1], "Database should stop second")
}

func TestIntegration_ContextCancellation(t *testing.T) {
	w := parseFixture(t, "sequence.yaml")
	// Use a long delay so the step is in-flight when we cancel.
	rec := testhelpers.NewRecordingRuntime(5 * time.Second)
	rt := newEngine(rec)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rt.RunWorkflow(ctx, w, "")
	require.Error(t, err)
	// Should not have completed all steps.
	assert.Less(t, len(rec.RunNames()), 2)
}
