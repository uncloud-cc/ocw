package ocw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	ocw, err := ParseFile("testdata/workflow.yaml")
	require.NoError(t, err)
	require.NotNil(t, ocw)

	assert.Equal(t, "0.1.0", ocw.SchemaVersion)
	assert.Equal(t, "Test Workflow", ocw.Name)
	require.Len(t, ocw.Sequence, 1)

	step := ocw.Sequence[0]
	require.NotNil(t, step.RunStep)
	assert.Equal(t, "step1", step.RunStep.Name)
	assert.Equal(t, "alpine:latest", step.RunStep.Image)
	assert.Equal(t, "echo \"hello\"", step.RunStep.Cmd)
}

func TestParseFile_NotFound(t *testing.T) {
	ocw, err := ParseFile("testdata/nonexistent.yaml")
	require.Error(t, err)
	assert.Nil(t, ocw)
	assert.Contains(t, err.Error(), "not found")
}

func TestParseBytes(t *testing.T) {
	content := []byte(`schemaVersion: "0.1.0"
name: ParseBytes Test
sequence:
  - name: step1
    image: alpine:latest
    cmd: echo "from bytes"`)

	ocw, err := ParseBytes(content)
	require.NoError(t, err)
	require.NotNil(t, ocw)

	assert.Equal(t, "0.1.0", ocw.SchemaVersion)
	assert.Equal(t, "ParseBytes Test", ocw.Name)
	require.Len(t, ocw.Sequence, 1)

	step := ocw.Sequence[0]
	require.NotNil(t, step.RunStep)
	assert.Equal(t, "step1", step.RunStep.Name)
	assert.Equal(t, "alpine:latest", step.RunStep.Image)
}

func TestParseBytes_InvalidYAML(t *testing.T) {
	content := []byte(`invalid: yaml: content: [unclosed`)

	ocw, err := ParseBytes(content)
	require.Error(t, err)
	assert.Nil(t, ocw)
	assert.Contains(t, err.Error(), "parsing workflow")
}

func TestParseString(t *testing.T) {
	content := `schemaVersion: "0.1.0"
name: ParseString Test
sequence:
  - name: step1
    image: alpine:latest
    cmd: echo "from string"`

	ocw, err := ParseString(content)
	require.NoError(t, err)
	require.NotNil(t, ocw)

	assert.Equal(t, "0.1.0", ocw.SchemaVersion)
	assert.Equal(t, "ParseString Test", ocw.Name)
	require.Len(t, ocw.Sequence, 1)

	step := ocw.Sequence[0]
	require.NotNil(t, step.RunStep)
	assert.Equal(t, "step1", step.RunStep.Name)
	assert.Equal(t, "alpine:latest", step.RunStep.Image)
}

func TestParseString_InvalidYAML(t *testing.T) {
	content := `invalid: yaml: [`

	ocw, err := ParseString(content)
	require.Error(t, err)
	assert.Nil(t, ocw)
}
