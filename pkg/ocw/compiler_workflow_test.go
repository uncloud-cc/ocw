package ocw

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestCompileWorkflowStep_Local(t *testing.T) {
	tmpDir := t.TempDir()
	fetcher := NewFetcherWithDir(tmpDir)
	printer := NewPrinter(false, false, nil)
	state := &State{
		Meta:    map[string]string{},
		Inputs:  map[string]string{"foo": "bar"},
		Secrets: map[string]string{},
		Steps:   map[string]map[string]string{},
	}

	// Create a referenced workflow
	refWorkflow := filepath.Join(tmpDir, "ref.yaml")
	err := os.WriteFile(refWorkflow, []byte(`
schemaVersion: "0.1.0"
name: referenced
sequence:
  - image: alpine:latest
    name: hello
    cmd: echo hello
`), 0644)
	require.NoError(t, err)

	step := &schema.WorkflowStep{
		OptionalStepBase: schema.OptionalStepBase{Name: "run-ref"},
		Workflow: schema.WorkflowConfig{
			Uses: "./ref.yaml",
		},
	}

	compiled, err := compileWorkflowStep(step, nil, state, printer, tmpDir, fetcher)
	require.NoError(t, err)
	assert.NotNil(t, compiled)
	if s, ok := compiled.(interface{ String() string }); ok {
		assert.Equal(t, "./ref.yaml", s.String())
	}
}

func TestCompileWorkflowStep_InheritanceNone(t *testing.T) {
	tmpDir := t.TempDir()
	fetcher := NewFetcherWithDir(tmpDir)
	printer := NewPrinter(false, false, nil)
	state := &State{
		Meta:    map[string]string{},
		Inputs:  map[string]string{"foo": "bar"},
		Secrets: map[string]string{"token": "secret123"},
		Steps:   map[string]map[string]string{},
	}

	refWorkflow := filepath.Join(tmpDir, "ref.yaml")
	err := os.WriteFile(refWorkflow, []byte(`
schemaVersion: "0.1.0"
name: referenced
sequence:
  - image: alpine:latest
    name: hello
    cmd: echo hello
`), 0644)
	require.NoError(t, err)

	step := &schema.WorkflowStep{
		OptionalStepBase: schema.OptionalStepBase{Name: "run-ref"},
		Workflow: schema.WorkflowConfig{
			Uses: "./ref.yaml",
			Inherit: &schema.InheritConfig{
				Env:     schema.InheritNone,
				Secrets: schema.InheritNone,
			},
		},
	}

	compiled, err := compileWorkflowStep(step, nil, state, printer, tmpDir, fetcher)
	require.NoError(t, err)
	assert.NotNil(t, compiled)
}

func TestCompileWorkflowStep_InheritanceAll(t *testing.T) {
	tmpDir := t.TempDir()
	fetcher := NewFetcherWithDir(tmpDir)
	printer := NewPrinter(false, false, nil)
	state := &State{
		Meta:    map[string]string{},
		Inputs:  map[string]string{"foo": "bar"},
		Secrets: map[string]string{"token": "secret123"},
		Steps:   map[string]map[string]string{},
	}

	refWorkflow := filepath.Join(tmpDir, "ref.yaml")
	err := os.WriteFile(refWorkflow, []byte(`
schemaVersion: "0.1.0"
name: referenced
sequence:
  - image: alpine:latest
    name: hello
    cmd: echo hello
`), 0644)
	require.NoError(t, err)

	step := &schema.WorkflowStep{
		OptionalStepBase: schema.OptionalStepBase{Name: "run-ref"},
		Workflow: schema.WorkflowConfig{
			Uses: "./ref.yaml",
			Inherit: &schema.InheritConfig{
				Env:     schema.InheritAll,
				Secrets: schema.InheritAll,
			},
		},
	}

	compiled, err := compileWorkflowStep(step, nil, state, printer, tmpDir, fetcher)
	require.NoError(t, err)
	assert.NotNil(t, compiled)
}

func TestCompileWorkflowStep_ExplicitEnv(t *testing.T) {
	tmpDir := t.TempDir()
	fetcher := NewFetcherWithDir(tmpDir)
	printer := NewPrinter(false, false, nil)
	state := &State{
		Meta:    map[string]string{},
		Inputs:  map[string]string{"foo": "bar"},
		Secrets: map[string]string{},
		Steps:   map[string]map[string]string{},
	}

	refWorkflow := filepath.Join(tmpDir, "ref.yaml")
	err := os.WriteFile(refWorkflow, []byte(`
schemaVersion: "0.1.0"
name: referenced
sequence:
  - image: alpine:latest
    name: hello
    cmd: echo hello
`), 0644)
	require.NoError(t, err)

	step := &schema.WorkflowStep{
		OptionalStepBase: schema.OptionalStepBase{Name: "run-ref"},
		Workflow: schema.WorkflowConfig{
			Uses: "./ref.yaml",
			Inputs: schema.Inputs{
				"myvar": {Value: "hello-world"},
			},
		},
	}

	compiled, err := compileWorkflowStep(step, nil, state, printer, tmpDir, fetcher)
	require.NoError(t, err)
	assert.NotNil(t, compiled)
}

func TestCompileWorkflowStep_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	fetcher := NewFetcherWithDir(tmpDir)
	printer := NewPrinter(false, false, nil)
	state := &State{
		Meta:    map[string]string{},
		Inputs:  map[string]string{},
		Secrets: map[string]string{},
		Steps:   map[string]map[string]string{},
	}

	step := &schema.WorkflowStep{
		OptionalStepBase: schema.OptionalStepBase{Name: "run-ref"},
		Workflow: schema.WorkflowConfig{
			Uses: "./missing.yaml",
		},
	}

	_, err := compileWorkflowStep(step, nil, state, printer, tmpDir, fetcher)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow file not found")
}

func TestIsLocalWorkflowRef(t *testing.T) {
	tests := []struct {
		from string
		want bool
	}{
		{"./local.yaml", true},
		{"../parent.yaml", true},
		{"/abs/path.yaml", true},
		{"\\windows\\path.yaml", true},
		{"github.com/owner/repo", false},
		{"owner/repo", false},
		{"github.com/owner/repo#branch", false},
		{"github.com/owner/repo@v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.from, func(t *testing.T) {
			got := isLocalWorkflowRef(tt.from)
			assert.Equal(t, tt.want, got)
		})
	}
}