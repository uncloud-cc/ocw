package ocw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	ocw, err := New("testdata/workflow.yaml")
	require.NoError(t, err)
	require.NotNil(t, ocw)

	assert.NotNil(t, ocw.Schema)
	assert.Equal(t, ocw.Schema.SchemaVersion, "0.1.0")
	assert.Equal(t, "testdata/workflow.yaml", ocw.Path)
}

func TestNewFromBuffer(t *testing.T) {
	content := []byte(`schemaVersion: "0.1.0"
name: ParseBytes Test
sequence:
  - name: step1
    image: alpine:latest
    cmd: echo "from bytes"`)

	ocw, err := NewFromBytes(content)
	require.NoError(t, err)
	require.NotNil(t, ocw)

	assert.NotNil(t, ocw.Schema)
	assert.Equal(t, ocw.Schema.SchemaVersion, "0.1.0")
	assert.Equal(t, "", ocw.Path)
}
