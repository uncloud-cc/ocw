// Package runner provides the execution engine for OCW workflows.
//
// The runner package handles:
//   - Workflow execution orchestration
//   - Docker container management
//   - Volume and network setup
//   - File watching and hot reload
//   - Parallel step execution
//
// Basic usage:
//
//	r := runner.NewRunner("/path/to/workflow").
//	    WithVerbose(true)
//	err := r.Run(ctx, workflow)
//
// The Runner supports builder pattern configuration:
//
//	r := runner.NewRunner(workflowDir).
//	    WithVerbose(true).
//	    WithEnvFile(".env").
//	    WithShowSecrets(false).
//	    WithForce(true)
//
// Workflows can be executed directly or by job name:
//
//	// Run entire workflow
//	err := r.Run(ctx, workflow)
//
//	// Run specific job
//	err := r.RunJob(ctx, workflow, "build")
package runner
