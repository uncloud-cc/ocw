package schema

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// Helper to create an ordered properties map
func newProperties() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
	return orderedmap.New[string, *jsonschema.Schema]()
}

// JSONSchema implements jsonschema.JSONSchemaer for StringOrStringSlice
// It produces an anyOf schema allowing either a string or an array of strings
func (StringOrStringSlice) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for StringMapOrSlice
// It produces an anyOf schema allowing either a map[string]string or []string
func (StringMapOrSlice) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Type: "string"},
			},
			{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for NumberOrString
// It produces an anyOf schema allowing either a number or a string
func (NumberOrString) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "number"},
			{Type: "string"},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for BoolOrString
// It produces an anyOf schema allowing either a boolean or a string
func (BoolOrString) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "boolean"},
			{Type: "string"},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for SecretValue
// It produces a oneOf schema allowing either a plain string or a secure object
func (SecretValue) JSONSchema() *jsonschema.Schema {
	secureProps := newProperties()
	secureProps.Set("secure", &jsonschema.Schema{Type: "string"})

	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{
				Type:                 "object",
				Properties:           secureProps,
				Required:             []string{"secure"},
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for BuildOutput
// It produces an anyOf schema for the various output formats
func (BuildOutput) JSONSchema() *jsonschema.Schema {
	annotationProps := newProperties()

	outputConfigProps := newProperties()
	outputConfigProps.Set("type", &jsonschema.Schema{
		Type: "string",
		Enum: []any{"docker", "image", "local", "tar", "oci", "registry"},
	})
	outputConfigProps.Set("dest", &jsonschema.Schema{Type: "string"})
	outputConfigProps.Set("push", &jsonschema.Schema{Type: "boolean"})
	outputConfigProps.Set("compression", &jsonschema.Schema{
		Type: "string",
		Enum: []any{"gzip", "estargz", "zstd", "uncompressed"},
	})
	outputConfigProps.Set("compressionLevel", &jsonschema.Schema{
		Type:    "number",
		Minimum: json.Number("0"),
		Maximum: json.Number("9"),
	})
	outputConfigProps.Set("forceCompression", &jsonschema.Schema{Type: "boolean"})
	outputConfigProps.Set("ociMediatypes", &jsonschema.Schema{Type: "boolean"})
	outputConfigProps.Set("annotation", &jsonschema.Schema{
		Type:                 "object",
		Properties:           annotationProps,
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	})

	outputConfigSchema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           outputConfigProps,
		Required:             []string{"type"},
		AdditionalProperties: jsonschema.FalseSchema,
	}

	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
			outputConfigSchema,
			{Type: "array", Items: outputConfigSchema},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for BuildSecrets
// It produces an anyOf schema for map or array of secret configs
func (BuildSecrets) JSONSchema() *jsonschema.Schema {
	secretConfigProps := newProperties()
	secretConfigProps.Set("id", &jsonschema.Schema{Type: "string"})
	secretConfigProps.Set("src", &jsonschema.Schema{Type: "string"})
	secretConfigProps.Set("env", &jsonschema.Schema{Type: "string"})

	secretConfigSchema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           secretConfigProps,
		Required:             []string{"id"},
		AdditionalProperties: jsonschema.FalseSchema,
	}

	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Type: "string"},
			},
			{Type: "array", Items: secretConfigSchema},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for StepOrSteps
// It produces a schema that accepts either a single step or array of steps
func (StepOrSteps) JSONSchema() *jsonschema.Schema {
	stepRef := &jsonschema.Schema{Ref: "#/$defs/Step"}
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			stepRef,
			{Type: "array", Items: stepRef},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for Step (discriminated union)
// It produces a oneOf schema with all step type variants
func (Step) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Ref: "#/$defs/RunStep"},
			{Ref: "#/$defs/BuildStep"},
			{Ref: "#/$defs/ParallelStep"},
			{Ref: "#/$defs/SequenceStep"},
			{Ref: "#/$defs/WorkflowStep"},
			{Ref: "#/$defs/SwitchStep"},
		},
	}
}

// JSONSchema implements jsonschema.JSONSchemaer for Input (discriminated union)
// It produces a oneOf schema with all input type variants
func (Input) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Ref: "#/$defs/StringInput"},
			{Ref: "#/$defs/NumberInput"},
			{Ref: "#/$defs/BooleanInput"},
			{Ref: "#/$defs/ChoiceInput"},
		},
	}
}
