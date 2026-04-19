package steps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterpolateEnv(t *testing.T) {
	sc := NewStepContext()
	sc.Env["HOME"] = "/home/user"
	sc.Env["USER"] = "testuser"

	result, err := Interpolate("Home is {{ env.HOME }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Home is /home/user", result)

	result, err = Interpolate("User: {{ env.USER }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "User: testuser", result)
}

func TestInterpolateSecrets(t *testing.T) {
	sc := NewStepContext()
	sc.Secrets["API_KEY"] = "secret123"

	result, err := Interpolate("Key: {{ secrets.API_KEY }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Key: secret123", result)
}

func TestInterpolateInputs(t *testing.T) {
	sc := NewStepContext()
	sc.Inputs["version"] = "1.2.3"

	result, err := Interpolate("Version: {{ inputs.version }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Version: 1.2.3", result)
}

func TestInterpolateStepOutputs(t *testing.T) {
	sc := NewStepContext()
	sc.Steps["build"] = map[string]string{
		"image_id": "abc123",
		"tag":      "latest",
	}

	result, err := Interpolate("Image: {{ steps.build.image_id }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Image: abc123", result)

	result, err = Interpolate("Tag: {{ steps.build.tag }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Tag: latest", result)
}

func TestInterpolateConfig(t *testing.T) {
	sc := NewStepContext()
	sc.Config["database"] = map[string]any{
		"host": "localhost",
		"port": 5432,
	}

	result, err := Interpolate("Host: {{ config.database.host }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Host: localhost", result)

	result, err = Interpolate("Port: {{ config.database.port }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "Port: 5432", result)
}

func TestInterpolateMultiple(t *testing.T) {
	sc := NewStepContext()
	sc.Env["USER"] = "alice"
	sc.Steps["setup"] = map[string]string{"dir": "/tmp"}

	result, err := Interpolate("{{ env.USER }} works in {{ steps.setup.dir }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "alice works in /tmp", result)
}

func TestInterpolateNoTemplates(t *testing.T) {
	sc := NewStepContext()

	result, err := Interpolate("plain text", sc)
	require.NoError(t, err)
	assert.Equal(t, "plain text", result)
}

func TestInterpolateEmptyString(t *testing.T) {
	sc := NewStepContext()

	result, err := Interpolate("", sc)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestInterpolateWhitespace(t *testing.T) {
	sc := NewStepContext()
	sc.Env["VAR"] = "value"

	result, err := Interpolate("{{  env.VAR  }}", sc)
	require.NoError(t, err)
	assert.Equal(t, "value", result)
}

func TestInterpolateMissingEnv(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ env.MISSING }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
	assert.Contains(t, err.Error(), "env variable")
}

func TestInterpolateMissingSecret(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ secrets.MISSING }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
	assert.Contains(t, err.Error(), "secret")
}

func TestInterpolateMissingInput(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ inputs.MISSING }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
	assert.Contains(t, err.Error(), "input")
}

func TestInterpolateMissingStep(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ steps.missing.key }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
	assert.Contains(t, err.Error(), "not found")
}

func TestInterpolateMissingStepOutput(t *testing.T) {
	sc := NewStepContext()
	sc.Steps["step1"] = map[string]string{"key1": "val1"}

	_, err := Interpolate("{{ steps.step1.missing_key }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step1")
	assert.Contains(t, err.Error(), "missing_key")
}

func TestInterpolateMissingConfig(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ config.missing }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestInterpolateInvalidExpression(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ singleword }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expression")
}

func TestInterpolateUnknownType(t *testing.T) {
	sc := NewStepContext()

	_, err := Interpolate("{{ unknown.type }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown reference type")
}

func TestInterpolateMap(t *testing.T) {
	sc := NewStepContext()
	sc.Env["VAL1"] = "a"
	sc.Env["VAL2"] = "b"

	input := map[string]string{
		"key1": "{{ env.VAL1 }}",
		"key2": "{{ env.VAL2 }}",
	}

	result, err := InterpolateMap(input, sc)
	require.NoError(t, err)
	assert.Equal(t, "a", result["key1"])
	assert.Equal(t, "b", result["key2"])
}

func TestInterpolateMapError(t *testing.T) {
	sc := NewStepContext()

	input := map[string]string{
		"key1": "{{ env.MISSING }}",
	}

	_, err := InterpolateMap(input, sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key1")
}

func TestInterpolateSlice(t *testing.T) {
	sc := NewStepContext()
	sc.Env["A"] = "1"
	sc.Env["B"] = "2"

	input := []string{"{{ env.A }}", "{{ env.B }}"}

	result, err := InterpolateSlice(input, sc)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, result)
}

func TestInterpolateSliceError(t *testing.T) {
	sc := NewStepContext()

	input := []string{"{{ env.MISSING }}"}

	_, err := InterpolateSlice(input, sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index 0")
}

func TestInterpolationError(t *testing.T) {
	err := &InterpolationError{
		Template:   "{{ env.MISSING }}",
		Expression: "env.MISSING",
		Reason:     "not found",
	}

	msg := err.Error()
	assert.Contains(t, msg, "env.MISSING")
	assert.Contains(t, msg, "not found")
	assert.Contains(t, msg, "{{ env.MISSING }}")
}

func TestInterpolatePartialFailure(t *testing.T) {
	// When one template fails, the error is returned but partial result contains original
	sc := NewStepContext()
	sc.Env["VAR"] = "value"

	_, err := Interpolate("{{ env.VAR }} and {{ env.MISSING }}", sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
}
