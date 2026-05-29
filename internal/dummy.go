package internal

import (
	"context"
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// DummyRuntime can be used as test for mocks
type DummyRuntime struct{}

func (d *DummyRuntime) Run(ctx context.Context, step *schema.RunStep) (map[string]string, error) {
	fmt.Printf(">>> RUN: %s (image=%s, cmd=%q)\n", step.Name, step.Image, step.Cmd)
	return nil, nil
}

func (d *DummyRuntime) Build(ctx context.Context, step *schema.BuildStep) (map[string]string, error) {
	fmt.Printf(">>> BUILD: %s (image=%s, dockerfile=%s)\n", step.Name, step.Build.Image, step.Build.Dockerfile)
	return nil, nil
}
