package ocw

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScope_Clone(t *testing.T) {
	s := NewScope()
	s.Env["FOO"] = "bar"
	s.Secrets["KEY"] = "secret"
	s.Steps["build"] = StepOutput{Values: map[string]string{"image": "myapp"}}
	s.Workflow = WorkflowMeta{Name: "original"}
	s.Job = JobMeta{Name: "build"}

	c := s.Clone()

	// Mutation on clone should not affect original.
	c.Env["FOO"] = "changed"
	c.Secrets["KEY"] = "changed"
	c.Steps["build"] = StepOutput{Values: map[string]string{"image": "changed"}}
	c.Workflow.Name = "changed"
	c.Job.Name = "changed"

	assert.Equal(t, "bar", s.Env["FOO"])
	assert.Equal(t, "secret", s.Secrets["KEY"])
	assert.Equal(t, "myapp", s.Steps["build"].Values["image"])
	assert.Equal(t, "original", s.Workflow.Name)
	assert.Equal(t, "build", s.Job.Name)
}

func TestScope_Interpolate(t *testing.T) {
	s := NewScope()
	s.Env["DEPLOY_ENV"] = "staging"
	s.Secrets["TOKEN"] = "abc123"
	s.Steps["build"] = StepOutput{Values: map[string]string{"image": "myapp:latest"}}
	s.Workflow = WorkflowMeta{Name: "My Workflow"}
	s.Job = JobMeta{Name: "deploy"}

	tests := []struct {
		name        string
		input       string
		expect      string
		wantErr     bool
		errContains string
	}{
		// --- env ---
		{"env with spaces", "{{ env.DEPLOY_ENV }}", "staging", false, ""},
		{"env without spaces", "{{env.DEPLOY_ENV}}", "staging", false, ""},
		{"env extra whitespace", "{{  env.DEPLOY_ENV  }}", "staging", false, ""},
		{"env mixed whitespace", "{{ env.DEPLOY_ENV}}", "staging", false, ""},
		{"env not in scope and not set is error", "{{ env.DEFINITELY_NOT_SET_ANYWHERE_XYZ }}", "{{ env.DEFINITELY_NOT_SET_ANYWHERE_XYZ }}", true, "environment variable \"DEFINITELY_NOT_SET_ANYWHERE_XYZ\" is not set"},

		// --- secrets ---
		{"secret", "{{ secrets.TOKEN }}", "abc123", false, ""},
		{"secret unresolved is error", "{{ secrets.MISSING }}", "{{ secrets.MISSING }}", true, "unresolved secret"},

		// --- steps ---
		{"step output", "{{ steps.build.image }}", "myapp:latest", false, ""},
		{"step missing id", "{{ steps.nope.image }}", "{{ steps.nope.image }}", true, "step \"nope\" not found"},
		{"step missing field", "{{ steps.build.nope }}", "{{ steps.build.nope }}", true, "has no output \"nope\""},
		{"step no field", "{{ steps.build }}", "{{ steps.build }}", true, "invalid step reference"},

		// --- workflow ---
		{"workflow name", "{{ workflow.name }}", "My Workflow", false, ""},
		{"workflow unknown", "{{ workflow.version }}", "{{ workflow.version }}", true, "unknown workflow property"},

		// --- job ---
		{"job name", "{{ job.name }}", "deploy", false, ""},
		{"job unknown", "{{ job.foo }}", "{{ job.foo }}", true, "unknown job property"},

		// --- unknown namespace ---
		{"unknown namespace", "{{ foo.bar }}", "{{ foo.bar }}", true, "unknown template namespace"},

		// --- invalid expression ---
		{"no dot", "{{ nodot }}", "{{ nodot }}", true, "invalid template expression"},

		// --- plain text ---
		{"plain text", "hello world", "hello world", false, ""},
		{"empty string", "", "", false, ""},

		// --- mixed ---
		{"multiple templates", "{{ env.DEPLOY_ENV }}-{{ workflow.name }}", "staging-My Workflow", false, ""},
		{"template in context", "image={{ steps.build.image }}", "image=myapp:latest", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.Interpolate(tt.input)
			assert.Equal(t, tt.expect, result)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScope_Interpolate_EnvHostFallback(t *testing.T) {
	// Set a host env var that is NOT in the scope's Env map.
	t.Setenv("OCW_TEST_HOST_VAR", "from-host")

	s := NewScope()
	s.Logger = log.Default()
	// Deliberately do NOT add OCW_TEST_HOST_VAR to s.Env.

	// Should warn (not error) and leave the template text as-is.
	result, err := s.Interpolate("{{ env.OCW_TEST_HOST_VAR }}")
	assert.NoError(t, err)
	assert.Equal(t, "{{ env.OCW_TEST_HOST_VAR }}", result)
}

func TestScope_Merge(t *testing.T) {
	s := NewScope()
	s.Merge("step1", StepOutput{Values: map[string]string{"out": "val"}})

	assert.Equal(t, "val", s.Steps["step1"].Values["out"])

	// Empty ID should not add anything.
	s.Merge("", StepOutput{Values: map[string]string{"x": "y"}})
	_, ok := s.Steps[""]
	assert.False(t, ok)
}
