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
		assertions []assertion
	}{
		{
			name:    "hello world",
			fixture: "1_hello_world.yaml",
			assertions: []assertion{
				{eventType: "step.start"},
				{eventType: "container.output", contains: "Hello OCW World!xxxx"},
				{eventType: "step.complete"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			fixture := filepath.Join("testdata", tc.fixture)
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
			require.NoError(t, err)

			engine.Bus.Close()
			collector.Wait()

			for _, a := range tc.assertions {
				if a.contains != "" {
					ocw.AssertContainsEventWith(t, collector.Events, a.eventType, a.contains)
				} else {
					ocw.AssertContainsEvent(t, collector.Events, a.eventType)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// assertion
// ---------------------------------------------------------------------------

type assertion struct {
	eventType string
	contains  string
}
