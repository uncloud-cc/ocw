package ocw

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestNewStepFactory_DispatchesAllTypes(t *testing.T) {
	dummy := NewDummyRuntime(log.Default())
	factory := newStepFactory(dummy, &ServiceRegistry{}, log.Default())

	tests := []struct {
		name       string
		step       schema.Step
		wantSimple bool
	}{
		{"run step", runStep("run", "alpine", "echo hi"), true},
		{"background run step (service)", schema.Step{RunStep: &schema.RunStep{
			StepBase:   schema.StepBase{Name: "db", ID: "db"},
			Image:      "postgres:15",
			Background: true,
		}}, true},
		{"build step", buildStep("build", "myapp"), true},
		{
			"sequence step",
			schema.Step{SequenceStep: &schema.SequenceStep{
				Sequence: []schema.Step{runStep("s1", "alpine", "echo 1")},
			}},
			false,
		},
		{
			"parallel step",
			schema.Step{ParallelStep: &schema.ParallelStep{
				Parallel: []schema.Step{runStep("p1", "alpine", "echo 1")},
			}},
			false,
		},
		{
			"switch step",
			schema.Step{SwitchStep: &schema.SwitchStep{
				Switch: "{{ env.MODE }}",
				Case: map[string]schema.Step{
					"test": runStep("test", "alpine", "echo test"),
				},
			}},
			false,
		},
		{
			"workflow step",
			schema.Step{WorkflowStep: &schema.WorkflowStep{
				Workflow: schema.WorkflowConfig{From: "./other.yaml"},
			}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := factory(tt.step)
			require.NotNil(t, runner)
			if tt.wantSimple {
				_, ok := runner.(SimpleRunner)
				assert.True(t, ok, "expected SimpleRunner")
			} else {
				_, ok := runner.(CompositeRunner)
				assert.True(t, ok, "expected CompositeRunner")
			}
		})
	}
}
