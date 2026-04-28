package ocw

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

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
