package e2e

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/ocw"
)

func TestEngine(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker daemon not reachable")
	}

	cases := []struct {
		name       string
		fixture    string
		jobName    string
		assertions []ocw.EventAssertion
	}{
		{
			name:    "hello world",
			fixture: "1_hello_world.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Hello OCW World! 🎉"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "container build",
			fixture: "2_build.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "naming to docker.io/library/my-first-ocw-container done"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "build and run",
			fixture: "3_build_and_run.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Hello ocw 🤖 ✨"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "sequence",
			fixture: "4_sequence.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Processing... 1"},
				{EventType: "container.output", Contains: "Processing... 2"},
				{EventType: "container.output", Contains: "Processing... 3"},
				{EventType: "container.output", Contains: "< Second step! >"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "parallel",
			fixture: "5_parallel.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start", Contains: "a"},
				{EventType: "step.start", Contains: "b"},
				{EventType: "container.output", Contains: "Processing step a)"},
				{EventType: "container.output", Contains: "Processing step b)"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "nested",
			fixture: "6_nested.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Setting up..."},
				{EventType: "container.output", Contains: "Running unit tests..."},
				{EventType: "container.output", Contains: "Running integration tests..."},
				{EventType: "container.output", Contains: "Running linter..."},
				{EventType: "container.output", Contains: "Building (only after all tests pass)..."},
				{EventType: "container.output", Contains: "Deploying..."},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "templates",
			fixture: "7_templates.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Workflow: Template Demo"},
				{EventType: "container.output", Contains: "User:"},
				{EventType: "container.output", Contains: "Home:"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "switch staging",
			fixture: "8_switch.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Deploying to development (default)..."},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "jobs build",
			fixture: "9_jobs.yaml",
			jobName: "build",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Installing dependencies..."},
				{EventType: "container.output", Contains: "Building..."},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "jobs test",
			fixture: "9_jobs.yaml",
			jobName: "test",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Running unit tests..."},
				{EventType: "container.output", Contains: "Running linter..."},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "jobs dev",
			fixture: "9_jobs.yaml",
			jobName: "dev",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Starting dev server on http://localhost:3000..."},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "outputs",
			fixture: "10_outputs.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "✅ Generated version"},
				{EventType: "container.output", Contains: "✅ Generated timestamp"},
				{EventType: "container.output", Contains: "Version: 1.0.0"},
				{EventType: "container.output", Contains: "Build Time:"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
		{
			name:    "context",
			fixture: "11_context.yaml",
			assertions: []ocw.EventAssertion{
				{EventType: "workflow.start"},
				{EventType: "step.start"},
				{EventType: "container.output", Contains: "Current folder:"},
				{EventType: "container.output", Contains: "Contents:"},
				{EventType: "step.complete", Fields: map[string]any{"Success": true}},
				{EventType: "workflow.complete", Fields: map[string]any{"Success": true}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			fixture := filepath.Join("../../examples/", tc.fixture)
			schema, err := ocw.ParseFile(fixture)
			require.NoError(t, err)
			require.NoError(t, schema.Validate())

			state, err := ocw.NewState(&schema.Inputs, "")
			require.NoError(t, err)

			opts := ocw.EngineOptions{JobName: tc.jobName}
			engine, err := ocw.NewEngine(schema, state, filepath.Dir(fixture), opts)
			require.NoError(t, err)
			defer engine.Close()

			collector := ocw.CollectEvents(engine.Bus)

			err = engine.Run(ctx)
			require.NoError(t, err)

			engine.Bus.Close()
			collector.Wait()

			ocw.AssertEvents(t, collector.Events, tc.assertions)
		})
	}
}

func TestEngineFailure(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker daemon not reachable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fixture := filepath.Join("testdata", "failing.yaml")
	schema, err := ocw.ParseFile(fixture)
	require.NoError(t, err)
	require.NoError(t, schema.Validate())

	state, err := ocw.NewState(&schema.Inputs, "")
	require.NoError(t, err)

	engine, err := ocw.NewEngine(schema, state, filepath.Dir(fixture), ocw.EngineOptions{})
	require.NoError(t, err)
	defer engine.Close()

	collector := ocw.CollectEvents(engine.Bus)

	err = engine.Run(ctx)
	require.Error(t, err)

	engine.Bus.Close()
	collector.Wait()

	ocw.AssertEvents(t, collector.Events, []ocw.EventAssertion{
		{EventType: "workflow.start"},
		{EventType: "step.start"},
		{EventType: "container.output", Contains: "About to fail..."},
		{EventType: "step.complete", Fields: map[string]any{"Success": false}},
		{EventType: "workflow.complete", Fields: map[string]any{"Success": false}},
	})

	// Ensure the second step never ran
	for _, ev := range collector.Events {
		if ev.EventType() == "container.output" {
			require.False(t, ocw.EventContains(ev, "This should not appear"), "second step should not have run")
		}
	}
}
