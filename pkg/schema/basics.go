package schema

// SchemaVersion is the current schema version
const SchemaVersion = "0.1.0"

// Name is a human readable name used to communicate feedback about a workflow.
// Choose them in a human friendly way (e.g. "Pull Requests", "Production Deployment" etc.)
// If a name happens to be a valid ID (e.g. "postgres", "build", "e2e", etc.) it can be used
// as an ID to reference this step.
type Name = string

// ID must start with a letter or underscore and can contain letters, underscores
// and numbers (but no whitespace). IDs are optional and only needed when wanting
// to reference a step by its ID.
// Pattern: ^[A-Za-z_][A-Za-z0-9_]*$
type ID = string

// Description is a human readable description
type Description = string

// Outputs are workflow outputs as key-value pairs
type Outputs = map[string]string
