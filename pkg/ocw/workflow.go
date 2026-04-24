package ocw

import "github.com/uncloud-cc/ocw/pkg/schema"

// Workflow holds the parsed schema and the entry point for execution.
type Workflow struct {
	Schema *schema.OCW
	Job    string // empty string means run the top-level flow
}

// WorkflowEngine loads and runs workflows.
type WorkflowEngine interface {
	Load(schema *schema.OCW, job string)
}
