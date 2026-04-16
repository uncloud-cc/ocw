// Package schema defines the OCW workflow configuration types.
//
// The schema package provides:
//   - YAML parsing for OCW workflow files
//   - Validation of workflow configurations
//   - JSON Schema generation for IDE support
//
// Basic usage:
//
//	ocw, err := schema.ParseFile("workflow.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := ocw.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//
// Parsing from bytes:
//
//	data := []byte(`
//	schemaVersion: v1
//	name: my-workflow
//	jobs:
//	  build:
//	    sequence:
//	      - image: golang:1.24
//	        run: go build
//	`)
//	ocw, err := schema.Parse(data)
//
// The schema supports multiple workflow patterns:
//   - Job-based workflows with named jobs
//   - Direct flow control (parallel, sequence, switch)
//   - Nested steps with parallel/sequence execution
//   - Service containers and health checks
//   - Volume mounting and environment variables
//   - Build steps for Docker images
//   - Hot reload with file watching
package schema
