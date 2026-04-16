package schema

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateParallelStep(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid parallel step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    parallel:
      - image: alpine
        run: echo 1
      - image: alpine
        run: echo 2
`,
			wantErr: false,
		},
		{
			name: "empty parallel step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    parallel: []
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			assert.NoError(t, err)

			err = ocw.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSequenceStep(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid sequence step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - image: alpine
        run: echo 1
      - image: alpine
        run: echo 2
`,
			wantErr: false,
		},
		{
			name: "empty sequence step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence: []
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			assert.NoError(t, err)

			err = ocw.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWorkflowStep(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid workflow step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - workflow:
          from: ./other.yaml
`,
			wantErr: false,
		},
		{
			name: "workflow step with valid inherit",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - workflow:
          from: ./other.yaml
          inherit:
            secrets: all
            env: none
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			assert.NoError(t, err)

			err = ocw.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSwitchStep(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid switch step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - switch: ${{ env.MODE }}
        case:
          dev:
            - image: alpine
              run: echo dev
          prod:
            - image: alpine
              run: echo prod
`,
			wantErr: false,
		},
		{
			name: "switch step with default",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - switch: ${{ env.MODE }}
        case:
          dev:
            - image: alpine
              run: echo dev
        default:
          - image: alpine
            run: echo default
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			assert.NoError(t, err)

			err = ocw.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAndParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid yaml",
			yaml: `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - image: alpine
        run: echo test
`,
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			yaml:    "invalid: [",
			wantErr: true,
		},
		{
			name: "valid yaml but invalid schema",
			yaml: `
schemaVersion: v1
name: test
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndParse([]byte(tt.yaml))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAndParseFile(t *testing.T) {
	// Create a temp file for testing
	content := `
schemaVersion: v1
name: test
jobs:
  test:
    sequence:
      - image: alpine
        run: echo test
`
	tmpFile := t.TempDir() + "/test.yaml"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	assert.NoError(t, err)

	_, err = ValidateAndParseFile(tmpFile)
	assert.NoError(t, err)

	// Test non-existent file
	_, err = ValidateAndParseFile("/non/existent/file.yaml")
	assert.Error(t, err)
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "validation error",
			err:  ValidationErrors{{Path: "test", Message: "test error"}},
			want: true,
		},
		{
			name: "regular error",
			err:  assert.AnError,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidationError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ValidationErrors
	}{
		{
			name: "validation errors",
			err: ValidationErrors{
				{Path: "path1", Message: "error 1"},
				{Path: "path2", Message: "error 2"},
			},
			want: ValidationErrors{
				{Path: "path1", Message: "error 1"},
				{Path: "path2", Message: "error 2"},
			},
		},
		{
			name: "regular error",
			err:  assert.AnError,
			want: nil,
		},
		{
			name: "nil error",
			err:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetValidationErrors(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSwitchFlowValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid switch flow at root",
			yaml: `
schemaVersion: v1
name: test
switch: ${{ env.MODE }}
case:
  dev:
    - image: alpine
      run: echo dev
  prod:
    - image: alpine
      run: echo prod
`,
			wantErr: false,
		},
		{
			name: "switch flow with default at root",
			yaml: `
schemaVersion: v1
name: test
switch: ${{ env.MODE }}
case:
  dev:
    - image: alpine
      run: echo dev
default:
  - image: alpine
    run: echo fallback
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			assert.NoError(t, err)

			err = ocw.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
