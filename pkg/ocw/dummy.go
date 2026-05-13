package ocw

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

type DummyExec struct{}

func (d *DummyExec) Run(ctx context.Context, step *schema.RunStep, env map[string]string) error {
	fmt.Printf(">>> RUN: %s (image=%s, cmd=%q)\n", step.Name, step.Image, step.Cmd)
	return nil
}

func (d *DummyExec) Build(ctx context.Context, step *schema.BuildStep, env map[string]string) error {
	fmt.Printf(">>> BUILD: %s (image=%s, dockerfile=%s)\n", step.Name, step.Build.Image, step.Build.Dockerfile)
	return nil
}
