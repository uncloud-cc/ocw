// schema-gen generates a JSON Schema from the OCW Go structs.
//
// Usage:
//
//	go run ./cmd/schema-gen > ../schema.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

func main() {
	r := &jsonschema.Reflector{
		// Don't use $ref for all types - inline simple ones
		DoNotReference: false,
		// Expand embedded structs
		ExpandedStruct: true,
		// Use "additionalProperties": false by default
		AllowAdditionalProperties: false,
	}

	// Generate schema from OCW struct
	s := r.Reflect(&schema.OCW{})

	// Add definitions for step types that participate in the Step
	// discriminated union. These types are only referenced via $ref in
	// Step.JSONSchema() and are not traversed automatically by the
	// reflector. Reflecting them also pulls in their nested types
	// (e.g., BuildConfig, HealthCheck, Watch, etc.).
	for _, stepType := range schema.StepTypes {
		addDefinition(r, s, stepType.Name, stepType.Type)
	}

	// Set schema metadata
	s.Version = "https://json-schema.org/draft/2020-12/schema"
	s.ID = "https://ocw.dev/schema.json"
	s.Title = "OCW Workflow Schema"
	s.Description = "Open Container Workflow schema definition"

	// Marshal with indentation
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	// Write to stdout
	fmt.Println(string(out))
}

// addDefinition adds a type's schema to the definitions
func addDefinition(r *jsonschema.Reflector, root *jsonschema.Schema, name string, v any) {
	t := reflect.TypeOf(v)
	schema := r.ReflectFromType(t)

	// Copy any nested definitions
	if schema.Definitions != nil {
		for defName, def := range schema.Definitions {
			if root.Definitions[defName] == nil {
				root.Definitions[defName] = def
			}
		}
	}

	// Add the type itself (clear out nested definitions to avoid duplication)
	schema.Definitions = nil
	if root.Definitions[name] == nil {
		root.Definitions[name] = schema
	}
}
