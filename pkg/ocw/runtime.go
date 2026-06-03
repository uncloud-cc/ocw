package ocw

import (
	"context"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// Runtime is the interface for executing steps.
type Runtime interface {
	Run(ctx context.Context, step *schema.RunStep, prefix string) (map[string]string, error)
	Build(ctx context.Context, step *schema.BuildStep, prefix string) (map[string]string, error)
}
