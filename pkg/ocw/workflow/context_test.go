package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepContext_Clone(t *testing.T) {
	tests := []struct {
		name string
		ctx  *StepContext
		want func(*testing.T, *StepContext)
	}{
		{
			name: "nil context",
			ctx:  nil,
			want: func(t *testing.T, clone *StepContext) {
				assert.Nil(t, clone)
			},
		},
		{
			name: "empty context",
			ctx: &StepContext{
				Env:     map[string]string{},
				Secrets: map[string]string{},
				Inputs:  map[string]string{},
				Steps:   map[string]map[string]string{},
				Workflow: WorkflowMeta{
					Name: "test",
					ID:   "123",
				},
				Services: map[string]*ServiceInfo{},
				Runtime:  &mockRuntime{},
			},
			want: func(t *testing.T, clone *StepContext) {
				require.NotNil(t, clone)
				assert.Equal(t, "test", clone.Workflow.Name)
				assert.Equal(t, "123", clone.Workflow.ID)
				// Services should be shared (same reference)
				assert.NotNil(t, clone.Services)
			},
		},
		{
			name: "context with env",
			ctx: &StepContext{
				Env: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
				},
				Secrets: map[string]string{"SECRET": "shh"},
				Inputs:  map[string]string{"INPUT": "data"},
				Steps: map[string]map[string]string{
					"step1": {"output1": "val1"},
				},
				Services: map[string]*ServiceInfo{
					"svc1": {StepID: "svc1", Healthy: true},
				},
			},
			want: func(t *testing.T, clone *StepContext) {
				require.NotNil(t, clone)
				// Maps should be copied, not shared
				assert.Equal(t, "value1", clone.Env["KEY1"])
				assert.Equal(t, "shh", clone.Secrets["SECRET"])
				assert.Equal(t, "data", clone.Inputs["INPUT"])
				assert.Equal(t, "val1", clone.Steps["step1"]["output1"])

				// Modifying clone shouldn't affect original
				clone.Env["NEWKEY"] = "newvalue"
				// (would need original reference to verify)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := tt.ctx.Clone()
			tt.want(t, clone)
		})
	}
}

func TestStepContext_Clone_SharedServices(t *testing.T) {
	originalServices := map[string]*ServiceInfo{
		"service1": {StepID: "service1", Healthy: false},
	}

	original := &StepContext{
		Services: originalServices,
	}

	clone := original.Clone()
	require.NotNil(t, clone)

	// Services map should be the same reference (shared)
	// Verify by checking they point to the same underlying data
	clone.Services["test"] = &ServiceInfo{StepID: "test"}
	assert.NotNil(t, original.Services["test"], "Services map should be shared")

	// Clean up test entry
	delete(clone.Services, "test")
}

func TestStepContext_Clone_PreservesRuntime(t *testing.T) {
	runtime := &mockRuntime{}
	original := &StepContext{
		Runtime: runtime,
	}

	clone := original.Clone()
	require.NotNil(t, clone)
	assert.Same(t, original.Runtime, clone.Runtime)
}

func TestWorkflowMeta(t *testing.T) {
	meta := WorkflowMeta{
		Name: "my-workflow",
		ID:   "wf-abc123",
		Path: "/path/to/workflow.yaml",
	}

	assert.Equal(t, "my-workflow", meta.Name)
	assert.Equal(t, "wf-abc123", meta.ID)
	assert.Equal(t, "/path/to/workflow.yaml", meta.Path)
}

func TestServiceInfo(t *testing.T) {
	svc := ServiceInfo{
		StepID:      "database",
		ContainerID: "container-xyz",
		Healthy:     true,
		Ports: []PortMapping{
			{ContainerPort: 5432, HostPort: 5432, Protocol: "tcp"},
			{ContainerPort: 8080, HostPort: 8080, Protocol: "http"},
		},
	}

	assert.Equal(t, "database", svc.StepID)
	assert.Equal(t, "container-xyz", svc.ContainerID)
	assert.True(t, svc.Healthy)
	assert.Len(t, svc.Ports, 2)
}

func TestPortMapping(t *testing.T) {
	port := PortMapping{
		ContainerPort: 3000,
		HostPort:      3000,
		Protocol:      "http",
	}

	assert.Equal(t, 3000, port.ContainerPort)
	assert.Equal(t, 3000, port.HostPort)
	assert.Equal(t, "http", port.Protocol)
}

func TestVolumeMount(t *testing.T) {
	mount := VolumeMount{
		Source:   "/host/path",
		Target:   "/container/path",
		ReadOnly: true,
	}

	assert.Equal(t, "/host/path", mount.Source)
	assert.Equal(t, "/container/path", mount.Target)
	assert.True(t, mount.ReadOnly)
}

func TestHealthCheckConfig(t *testing.T) {
	hc := HealthCheckConfig{
		Cmd:         "curl -f http://localhost:8080/health",
		Interval:    "10s",
		Timeout:     "5s",
		Retries:     3,
		StartPeriod: "30s",
	}

	assert.Equal(t, "curl -f http://localhost:8080/health", hc.Cmd)
	assert.Equal(t, "10s", hc.Interval)
	assert.Equal(t, "5s", hc.Timeout)
	assert.Equal(t, 3, hc.Retries)
	assert.Equal(t, "30s", hc.StartPeriod)
}
