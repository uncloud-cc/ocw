// Package schema defines the OCW (Open Container Workflow) data types, validation, and YAML marshalling.
//
// This package contains:
//   - Go struct definitions for the OCW schema
//   - Schema validation (ensures data conforms to schema structure)
//   - YAML marshalling/unmarshalling logic (custom MarshalYAML/UnmarshalYAML methods)
//
// This package does NOT contain:
//   - Business logic or workflow execution (see pkg/ocw)
//   - Parsing functions for reading workflow files (see pkg/ocw)
//   - Helper methods for traversing/processing workflows (see pkg/ocw)
//
// If you need to add logic that operates on these types, put it in pkg/ocw instead.
//
// Design Guidelines:
//   - Keep types pure and focused on data representation
//   - Validation checks structural integrity (required fields, valid formats, flow control rules)
//   - Ensure YAML tags match the JSON schema
//   - Use custom unmarshaling for flexible types (string or slice, etc.)
//   - Test marshalling and validation in *_test.go files
//
// Example type definition with validation:
//
//	// Validate ensures the step has valid configuration
//	func (s *Step) Validate() error {
//	    if s.Image == "" {
//	        return fmt.Errorf("step image is required")
//	    }
//	    return nil
//	}
//
// For parsing workflows and working with them, use pkg/ocw instead:
//
//	ocw, err := ocw.ParseFile("workflow.yaml")  // Use pkg/ocw
//	// Access helpers like:
//	if err := ocw.Validate(); err != nil {      // Schema validation
//	    return err
//	}
//	flowType := ocw.GetFlowType(ocw)            // Function in pkg/ocw
//	steps := ocw.GetSteps(ocw)                  // Function in pkg/ocw
package schema
