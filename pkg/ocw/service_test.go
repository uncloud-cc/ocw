package ocw

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestEngine_ServiceRunner_Basic(t *testing.T) {
	rt, dummy := newTestEngine()

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

func TestEngine_ServiceRunner_WithHealthCheck(t *testing.T) {
	rt, dummy := newTestEngine()

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

func TestEngine_ServiceRunner_NeedsDependency(t *testing.T) {
	rt, dummy := newTestEngine()

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

func TestEngine_ServiceRunner_NeedsMissing(t *testing.T) {
	rt, _ := newTestEngine()

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

func TestEngine_Shutdown(t *testing.T) {
	rt, dummy := newTestEngine()

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

func TestEngine_MultipleServicesInSequence(t *testing.T) {
	rt, dummy := newTestEngine()

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
