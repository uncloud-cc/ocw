package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProperties(t *testing.T) {
	props := newProperties()
	assert.NotNil(t, props)
	assert.Equal(t, 0, props.Len())
}

func TestStringOrStringSliceJSONSchema(t *testing.T) {
	var s StringOrStringSlice
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)
	assert.Equal(t, "string", schema.AnyOf[0].Type)
	assert.Equal(t, "array", schema.AnyOf[1].Type)
	assert.NotNil(t, schema.AnyOf[1].Items)
	assert.Equal(t, "string", schema.AnyOf[1].Items.Type)
}

func TestStringMapOrSliceJSONSchema(t *testing.T) {
	var s StringMapOrSlice
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)
	assert.Equal(t, "object", schema.AnyOf[0].Type)
	assert.NotNil(t, schema.AnyOf[0].AdditionalProperties)
	assert.Equal(t, "string", schema.AnyOf[0].AdditionalProperties.Type)
	assert.Equal(t, "array", schema.AnyOf[1].Type)
	assert.NotNil(t, schema.AnyOf[1].Items)
	assert.Equal(t, "string", schema.AnyOf[1].Items.Type)
}

func TestNumberOrStringJSONSchema(t *testing.T) {
	var s NumberOrString
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)
	assert.Equal(t, "number", schema.AnyOf[0].Type)
	assert.Equal(t, "string", schema.AnyOf[1].Type)
}

func TestBoolOrStringJSONSchema(t *testing.T) {
	var s BoolOrString
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)
	assert.Equal(t, "boolean", schema.AnyOf[0].Type)
	assert.Equal(t, "string", schema.AnyOf[1].Type)
}

func TestSecretValueJSONSchema(t *testing.T) {
	var s SecretValue
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.OneOf)
	assert.Len(t, schema.OneOf, 2)

	// First option: plain string
	assert.Equal(t, "string", schema.OneOf[0].Type)

	// Second option: secure object
	assert.Equal(t, "object", schema.OneOf[1].Type)
	assert.NotNil(t, schema.OneOf[1].Properties)
	assert.NotNil(t, schema.OneOf[1].Required)
	assert.Contains(t, schema.OneOf[1].Required, "secure")
}

func TestBuildOutputJSONSchema(t *testing.T) {
	var b BuildOutput
	schema := b.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 4)

	// String option
	assert.Equal(t, "string", schema.AnyOf[0].Type)

	// Array of strings option
	assert.Equal(t, "array", schema.AnyOf[1].Type)
	assert.Equal(t, "string", schema.AnyOf[1].Items.Type)

	// Object config option
	assert.Equal(t, "object", schema.AnyOf[2].Type)
	assert.NotNil(t, schema.AnyOf[2].Properties)
	assert.Contains(t, schema.AnyOf[2].Required, "type")

	// Array of object configs option
	assert.Equal(t, "array", schema.AnyOf[3].Type)
	assert.NotNil(t, schema.AnyOf[3].Items)
	assert.Equal(t, "object", schema.AnyOf[3].Items.Type)
}

func TestBuildSecretsJSONSchema(t *testing.T) {
	var s BuildSecrets
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)

	// Map option
	assert.Equal(t, "object", schema.AnyOf[0].Type)
	assert.NotNil(t, schema.AnyOf[0].AdditionalProperties)
	assert.Equal(t, "string", schema.AnyOf[0].AdditionalProperties.Type)

	// Array of secret configs option
	assert.Equal(t, "array", schema.AnyOf[1].Type)
	assert.NotNil(t, schema.AnyOf[1].Items)
	assert.Equal(t, "object", schema.AnyOf[1].Items.Type)
	assert.Contains(t, schema.AnyOf[1].Items.Required, "id")
}

func TestStepOrStepsJSONSchema(t *testing.T) {
	var s StepOrSteps
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.AnyOf)
	assert.Len(t, schema.AnyOf, 2)

	// Single step reference
	assert.Equal(t, "#/$defs/Step", schema.AnyOf[0].Ref)

	// Array of steps
	assert.Equal(t, "array", schema.AnyOf[1].Type)
	assert.NotNil(t, schema.AnyOf[1].Items)
	assert.Equal(t, "#/$defs/Step", schema.AnyOf[1].Items.Ref)
}

func TestStepJSONSchema(t *testing.T) {
	var s Step
	schema := s.JSONSchema()

	assert.NotNil(t, schema)
	assert.NotNil(t, schema.OneOf)
	assert.Len(t, schema.OneOf, 6)

	// Check all step type references
	expectedRefs := []string{
		"#/$defs/RunStep",
		"#/$defs/BuildStep",
		"#/$defs/ParallelStep",
		"#/$defs/SequenceStep",
		"#/$defs/WorkflowStep",
		"#/$defs/SwitchStep",
	}

	for i, expectedRef := range expectedRefs {
		assert.Equal(t, expectedRef, schema.OneOf[i].Ref, "Step type reference %d should match", i)
	}
}

func TestBuildOutputEnumValues(t *testing.T) {
	var b BuildOutput
	schema := b.JSONSchema()

	// Find the object config schema (third option in AnyOf)
	objectConfig := schema.AnyOf[2]

	// Check that type field has proper enum values
	typeProp, exists := objectConfig.Properties.Get("type")
	assert.True(t, exists)
	assert.NotNil(t, typeProp.Enum)

	expectedTypes := []any{"docker", "image", "local", "tar", "oci", "registry"}
	assert.ElementsMatch(t, expectedTypes, typeProp.Enum)
}

func TestBuildOutputCompressionEnumValues(t *testing.T) {
	var b BuildOutput
	schema := b.JSONSchema()

	// Find the object config schema
	objectConfig := schema.AnyOf[2]

	// Check compression field enum
	compressionProp, exists := objectConfig.Properties.Get("compression")
	assert.True(t, exists)
	assert.NotNil(t, compressionProp.Enum)

	expectedCompressions := []any{"gzip", "estargz", "zstd", "uncompressed"}
	assert.ElementsMatch(t, expectedCompressions, compressionProp.Enum)
}

func TestSecretValueRequired(t *testing.T) {
	var s SecretValue
	schema := s.JSONSchema()

	// Check the object variant has required fields
	objectSchema := schema.OneOf[1]
	assert.NotNil(t, objectSchema.Required)
	assert.Contains(t, objectSchema.Required, "secure")
	assert.NotNil(t, objectSchema.AdditionalProperties)
}
