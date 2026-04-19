package steps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccess(t *testing.T) {
	result := Success()

	assert.Equal(t, 0, result.ExitCode)
	assert.NotNil(t, result.Outputs)
	assert.Empty(t, result.Outputs)
	assert.Empty(t, result.ContainerID)
	assert.False(t, result.IsBackground)
}

func TestSuccessWithOutputs(t *testing.T) {
	outputs := map[string]string{
		"foo": "bar",
		"baz": "qux",
	}
	result := SuccessWithOutputs(outputs)

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, outputs, result.Outputs)
}

func TestFailed(t *testing.T) {
	result := Failed(1)

	assert.Equal(t, 1, result.ExitCode)
	assert.NotNil(t, result.Outputs)
	assert.Empty(t, result.Outputs)
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		results []*Result
		want    *Result
	}{
		{
			name:    "empty",
			results: []*Result{},
			want: &Result{
				ExitCode: 0,
				Outputs:  map[string]string{},
			},
		},
		{
			name: "single success",
			results: []*Result{
				SuccessWithOutputs(map[string]string{"key1": "val1"}),
			},
			want: &Result{
				ExitCode: 0,
				Outputs:  map[string]string{"key1": "val1"},
			},
		},
		{
			name: "multiple outputs merged",
			results: []*Result{
				SuccessWithOutputs(map[string]string{"a": "1", "b": "2"}),
				SuccessWithOutputs(map[string]string{"c": "3"}),
			},
			want: &Result{
				ExitCode: 0,
				Outputs:  map[string]string{"a": "1", "b": "2", "c": "3"},
			},
		},
		{
			name: "last write wins for duplicate keys",
			results: []*Result{
				SuccessWithOutputs(map[string]string{"key": "first"}),
				SuccessWithOutputs(map[string]string{"key": "second"}),
			},
			want: &Result{
				ExitCode: 0,
				Outputs:  map[string]string{"key": "second"},
			},
		},
		{
			name: "exit code is non-zero if any failed",
			results: []*Result{
				Success(),
				Failed(1),
				Success(),
			},
			want: &Result{
				ExitCode: 1,
				Outputs:  map[string]string{},
			},
		},
		{
			name:    "nil results are skipped",
			results: []*Result{nil, Success(), nil},
			want: &Result{
				ExitCode: 0,
				Outputs:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.results)
			assert.Equal(t, tt.want.ExitCode, got.ExitCode)
			assert.Equal(t, tt.want.Outputs, got.Outputs)
		})
	}
}
