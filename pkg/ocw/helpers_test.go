package ocw

import (
	"log"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// ---------------------------------------------------------------------------
// Shared test helpers
// ---------------------------------------------------------------------------

func newTestEngine() (*Engine, *DummyRuntime) {
	dummy := NewDummyRuntime(log.Default())
	rt := NewEngine(dummy, log.Default())
	return rt, dummy
}

func runStep(name, image, cmd string) schema.Step {
	return schema.Step{
		RunStep: &schema.RunStep{
			StepBase: schema.StepBase{Name: name},
			Image:    image,
			Cmd:      cmd,
		},
	}
}

func runStepWithID(name, id, image, cmd string) schema.Step {
	return schema.Step{
		RunStep: &schema.RunStep{
			StepBase: schema.StepBase{Name: name, ID: id},
			Image:    image,
			Cmd:      cmd,
		},
	}
}

func buildStep(name, image string) schema.Step {
	return schema.Step{
		BuildStep: &schema.BuildStep{
			StepBase: schema.StepBase{Name: name},
			Build:    schema.BuildConfig{Image: image},
		},
	}
}
