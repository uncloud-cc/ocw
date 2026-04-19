package steps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStepContext(t *testing.T) {
	sc := NewStepContext()

	assert.NotNil(t, sc)
	assert.NotNil(t, sc.Env)
	assert.NotNil(t, sc.Secrets)
	assert.NotNil(t, sc.Inputs)
	assert.NotNil(t, sc.Steps)
	assert.NotNil(t, sc.Config)
	assert.Empty(t, sc.Env)
	assert.Empty(t, sc.Secrets)
	assert.Empty(t, sc.Inputs)
	assert.Empty(t, sc.Steps)
	assert.Empty(t, sc.Config)
}

func TestStepContextClone(t *testing.T) {
	original := NewStepContext()
	original.Env["VAR1"] = "value1"
	original.Secrets["SECRET"] = "hidden"
	original.Inputs["input1"] = "input_value"
	original.Steps["step1"] = map[string]string{"output1": "output_value"}
	original.Config["key"] = "config_value"

	clone := original.Clone()

	// Verify all values copied
	assert.Equal(t, "value1", clone.Env["VAR1"])
	assert.Equal(t, "hidden", clone.Secrets["SECRET"])
	assert.Equal(t, "input_value", clone.Inputs["input1"])
	assert.Equal(t, "output_value", clone.Steps["step1"]["output1"])
	assert.Equal(t, "config_value", clone.Config["key"])

	// Verify clone is independent
	clone.Env["VAR1"] = "modified"
	assert.Equal(t, "value1", original.Env["VAR1"])
	assert.Equal(t, "modified", clone.Env["VAR1"])

	// Verify nested maps are cloned
	clone.Steps["step1"]["output1"] = "modified_output"
	assert.Equal(t, "output_value", original.Steps["step1"]["output1"])
}

func TestStepContextWithStepOutputs(t *testing.T) {
	base := NewStepContext()
	base.Env["EXISTING"] = "value"
	base.Steps["old_step"] = map[string]string{"out": "old_val"}

	newOutputs := map[string]string{
		"new_out1": "val1",
		"new_out2": "val2",
	}

	result := base.WithStepOutputs("new_step", newOutputs)

	// Original should be unchanged
	assert.Empty(t, base.Steps["new_step"])
	assert.Equal(t, "old_val", base.Steps["old_step"]["out"])

	// New context should have both old and new
	assert.Equal(t, "value", result.Env["EXISTING"])
	assert.Equal(t, "old_val", result.Steps["old_step"]["out"])
	assert.Equal(t, "val1", result.Steps["new_step"]["new_out1"])
	assert.Equal(t, "val2", result.Steps["new_step"]["new_out2"])
}

func TestStepContextWithStepOutputsOverwrites(t *testing.T) {
	base := NewStepContext()
	base.Steps["step1"] = map[string]string{"key": "original"}

	newOutputs := map[string]string{
		"key":     "replaced",
		"new_key": "new_value",
	}

	result := base.WithStepOutputs("step1", newOutputs)

	// Original unchanged
	assert.Equal(t, "original", base.Steps["step1"]["key"])

	// New context has replaced value
	assert.Equal(t, "replaced", result.Steps["step1"]["key"])
	assert.Equal(t, "new_value", result.Steps["step1"]["new_key"])
}

func TestStepContextCloneEmpty(t *testing.T) {
	original := NewStepContext()
	clone := original.Clone()

	assert.Empty(t, clone.Env)
	assert.Empty(t, clone.Secrets)
	assert.Empty(t, clone.Inputs)
	assert.Empty(t, clone.Steps)
	assert.Empty(t, clone.Config)
}
