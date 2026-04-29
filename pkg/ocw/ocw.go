package ocw

import (
	"fmt"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// OCW is the main entry point for running an open container workflow.
// It's the main orchestrator that ties everything together.
type OCW struct {
	Path   string
	Schema *schema.OCW
}

// Returns a new ocw instance from file
func New(path string) (*OCW, error) {
	schema, err := ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse file %s: %w", path, err)
	}

	return &OCW{
		Path:   path,
		Schema: schema,
	}, nil
}

// Returns a new ocw instance from a buffer
func NewFromBytes(data []byte) (*OCW, error) {
	schema, err := ParseBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse schema: %w", err)
	}

	return &OCW{
		Schema: schema,
	}, nil
}

func (o *OCW) Validate() error {
	if err := o.Schema.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return nil
}
