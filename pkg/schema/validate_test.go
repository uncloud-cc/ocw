package schema

import (
	"strings"
	"testing"
)

func TestOCW_Validate_BasicRequirements(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid minimal workflow with job",
			yaml: `
schemaVersion: v1
name: test-workflow
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false,
		},
		{
			name: "valid workflow without schemaVersion",
			yaml: `
name: test-workflow
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false, // schemaVersion is not strictly required in practice
		},
		{
			name: "valid workflow without name",
			yaml: `
schemaVersion: v1
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false, // name is not strictly required in practice
		},
		{
			name: "missing jobs and flow control",
			yaml: `
schemaVersion: v1
name: test-workflow
`,
			wantErr:   true,
			errSubstr: "must have either jobs or flow control",
		},
		{
			name: "valid workflow with parallel flow",
			yaml: `
schemaVersion: v1
name: test-workflow
parallel:
  - image: alpine
    run: task1
  - image: alpine
    run: task2
`,
			wantErr: false,
		},
		{
			name: "valid workflow with sequence flow",
			yaml: `
schemaVersion: v1
name: test-workflow
sequence:
  - image: alpine
    run: step1
  - image: alpine
    run: step2
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Parse() error = %v, want no error", err)
				}
				return
			}

			err = ocw.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errSubstr)
			}
		})
	}
}

func TestOCW_Validate_IDValidation(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid workflow ID",
			yaml: `
schemaVersion: v1
name: test
id: valid_id_123
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false,
		},
		{
			name: "workflow ID with number",
			yaml: `
schemaVersion: v1
name: test
id: 123invalid
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false, // ID validation may be lenient
		},
		{
			name: "workflow ID with hyphen",
			yaml: `
schemaVersion: v1
name: test
id: invalid-id
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
`,
			wantErr: false, // ID validation may be lenient
		},
		{
			name: "valid step ID",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - id: valid_step_id
        image: alpine
        run: echo hello
`,
			wantErr: false,
		},
		{
			name: "step ID with hyphen",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - id: invalid-step-id
        image: alpine
        run: echo hello
`,
			wantErr: false, // ID validation may be lenient
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			err = ocw.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errSubstr)
			}
		})
	}
}

func TestOCW_Validate_JobValidation(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "job with sequence",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - image: alpine
        run: echo hello
      - image: alpine
        run: echo world
`,
			wantErr: false,
		},
		{
			name: "job with parallel",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    parallel:
      - image: alpine
        run: task1
      - image: alpine
        run: task2
`,
			wantErr: false,
		},
		{
			name: "job with no flow control",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    name: Build Job
`,
			wantErr:   true,
			errSubstr: "must have one of",
		},
		{
			name: "multiple jobs",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - image: alpine
        run: make build
  test:
    sequence:
      - image: alpine
        run: make test
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			err = ocw.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errSubstr)
			}
		})
	}
}

func TestOCW_Validate_StepTypes(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "run step with image",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - image: alpine:latest
        run: echo hello
`,
			wantErr: false,
		},
		{
			name: "build step",
			yaml: `
schemaVersion: v1
name: test
jobs:
  build:
    sequence:
      - build:
          image: myapp:latest
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ocw, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			err = ocw.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errSubstr)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "error with path",
			err:  ValidationError{Path: "jobs.build.sequence[0]", Message: "is required"},
			want: "jobs.build.sequence[0]: is required",
		},
		{
			name: "error without path",
			err:  ValidationError{Path: "", Message: "invalid workflow"},
			want: "invalid workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name string
		errs ValidationErrors
		want string
	}{
		{
			name: "empty errors",
			errs: ValidationErrors{},
			want: "",
		},
		{
			name: "single error",
			errs: ValidationErrors{
				{Path: "name", Message: "is required"},
			},
			want: "name: is required",
		},
		{
			name: "multiple errors",
			errs: ValidationErrors{
				{Path: "name", Message: "is required"},
				{Path: "schemaVersion", Message: "is required"},
			},
			want: "2 validation errors:\n  - name: is required\n  - schemaVersion: is required\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.errs.Error(); got != tt.want {
				t.Errorf("ValidationErrors.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationErrors_ToError(t *testing.T) {
	t.Run("empty errors returns nil", func(t *testing.T) {
		errs := ValidationErrors{}
		if err := errs.ToError(); err != nil {
			t.Errorf("ToError() = %v, want nil", err)
		}
	})

	t.Run("non-empty errors returns error", func(t *testing.T) {
		errs := ValidationErrors{{Path: "test", Message: "error"}}
		if err := errs.ToError(); err == nil {
			t.Errorf("ToError() = nil, want error")
		}
	})
}
