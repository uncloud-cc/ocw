package ocw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecrets(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		secrets []string
		want    string
	}{
		{
			name:    "no secrets",
			text:    "hello world",
			secrets: []string{},
			want:    "hello world",
		},
		{
			name:    "single secret match",
			text:    "token is abc123",
			secrets: []string{"abc123"},
			want:    "token is [secret]",
		},
		{
			name:    "multiple secrets",
			text:    "user=admin password=secret123",
			secrets: []string{"admin", "secret123"},
			want:    "user=[secret] password=[secret]",
		},
		{
			name:    "secret appears multiple times",
			text:    "abc123 and abc123 again",
			secrets: []string{"abc123"},
			want:    "[secret] and [secret] again",
		},
		{
			name:    "empty string secret is ignored",
			text:    "hello world",
			secrets: []string{""},
			want:    "hello world",
		},
		{
			name:    "substring match is masked",
			text:    "the password is secret1234",
			secrets: []string{"secret123"},
			want:    "the password is [secret]4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSecrets(tt.text, tt.secrets)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskStringMap(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]string
		secrets []string
		want    map[string]string
	}{
		{
			name:    "nil map returns nil",
			m:       nil,
			secrets: []string{"secret"},
			want:    nil,
		},
		{
			name:    "no secrets returns copy",
			m:       map[string]string{"key": "value"},
			secrets: []string{},
			want:    map[string]string{"key": "value"},
		},
		{
			name:    "masks values not keys",
			m:       map[string]string{"password": "secret123"},
			secrets: []string{"secret123"},
			want:    map[string]string{"password": "[secret]"},
		},
		{
			name:    "multiple entries",
			m:       map[string]string{"a": "secret", "b": "safe", "c": "secret"},
			secrets: []string{"secret"},
			want:    map[string]string{"a": "[secret]", "b": "safe", "c": "[secret]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskStringMap(tt.m, tt.secrets)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskAny(t *testing.T) {
	tests := []struct {
		name    string
		v       any
		secrets []string
		want    any
	}{
		{
			name:    "string",
			v:       "password is secret123",
			secrets: []string{"secret123"},
			want:    "password is [secret]",
		},
		{
			name:    "int passes through",
			v:       42,
			secrets: []string{"secret"},
			want:    42,
		},
		{
			name:    "bool passes through",
			v:       true,
			secrets: []string{"secret"},
			want:    true,
		},
		{
			name:    "map[string]any",
			v:       map[string]any{"password": "secret123"},
			secrets: []string{"secret123"},
			want:    map[string]any{"password": "[secret]"},
		},
		{
			name:    "map[string]string",
			v:       map[string]string{"password": "secret123"},
			secrets: []string{"secret123"},
			want:    map[string]string{"password": "[secret]"},
		},
		{
			name:    "[]any",
			v:       []any{"secret123", "safe"},
			secrets: []string{"secret123"},
			want:    []any{"[secret]", "safe"},
		},
		{
			name:    "[]string",
			v:       []string{"secret123", "safe"},
			secrets: []string{"secret123"},
			want:    []string{"[secret]", "safe"},
		},
		{
			name:    "nested map[string]any",
			v:       map[string]any{"nested": map[string]any{"password": "secret123"}},
			secrets: []string{"secret123"},
			want:    map[string]any{"nested": map[string]any{"password": "[secret]"}},
		},
		{
			name:    "nested []any with map inside",
			v:       []any{map[string]any{"password": "secret123"}},
			secrets: []string{"secret123"},
			want:    []any{map[string]any{"password": "[secret]"}},
		},
		{
			name:    "nil passes through",
			v:       nil,
			secrets: []string{"secret"},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskAny(tt.v, tt.secrets)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskMap(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		secrets []string
		want    map[string]any
	}{
		{
			name:    "nil map returns nil",
			m:       nil,
			secrets: []string{"secret"},
			want:    nil,
		},
		{
			name:    "flat map",
			m:       map[string]any{"password": "secret123"},
			secrets: []string{"secret123"},
			want:    map[string]any{"password": "[secret]"},
		},
		{
			name:    "nested map",
			m:       map[string]any{"data": map[string]any{"token": "abc123"}},
			secrets: []string{"abc123"},
			want:    map[string]any{"data": map[string]any{"token": "[secret]"}},
		},
		{
			name:    "mixed types",
			m:       map[string]any{"count": 5, "token": "abc123"},
			secrets: []string{"abc123"},
			want:    map[string]any{"count": 5, "token": "[secret]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskMap(tt.m, tt.secrets)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskEvent(t *testing.T) {
	secrets := []string{"secret123"}

	tests := []struct {
		name        string
		ev          Event
		secrets     []string
		showSecrets bool
		want        Event
	}{
		{
			name:        "showSecrets=true returns unchanged",
			ev:          &ContainerOutput{Line: "token is secret123"},
			secrets:     secrets,
			showSecrets: true,
			want:        &ContainerOutput{Line: "token is secret123"},
		},
		{
			name:        "ContainerOutput masks Line",
			ev:          &ContainerOutput{Line: "token is secret123"},
			secrets:     secrets,
			showSecrets: false,
			want:        &ContainerOutput{Line: "token is [secret]"},
		},
		{
			name:        "LogDebug masks Message and Fields",
			ev:          &LogDebug{Message: "token is secret123", Fields: map[string]any{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &LogDebug{Message: "token is [secret]", Fields: map[string]any{"token": "[secret]"}},
		},
		{
			name:        "LogInfo masks Message and Fields",
			ev:          &LogInfo{Message: "token is secret123", Fields: map[string]any{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &LogInfo{Message: "token is [secret]", Fields: map[string]any{"token": "[secret]"}},
		},
		{
			name:        "LogWarn masks Message and Fields",
			ev:          &LogWarn{Message: "token is secret123", Fields: map[string]any{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &LogWarn{Message: "token is [secret]", Fields: map[string]any{"token": "[secret]"}},
		},
		{
			name:        "LogError masks Message and Fields",
			ev:          &LogError{Message: "token is secret123", Fields: map[string]any{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &LogError{Message: "token is [secret]", Fields: map[string]any{"token": "[secret]"}},
		},
		{
			name:        "WorkflowOutputs masks Outputs",
			ev:          &WorkflowOutputs{Outputs: map[string]string{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &WorkflowOutputs{Outputs: map[string]string{"token": "[secret]"}},
		},
		{
			name:        "StepStart masks Extra",
			ev:          &StepStart{Extra: map[string]string{"token": "secret123"}},
			secrets:     secrets,
			showSecrets: false,
			want:        &StepStart{Extra: map[string]string{"token": "[secret]"}},
		},
		{
			name:        "WorkflowStart has no secrets to mask",
			ev:          &WorkflowStart{Name: "deploy"},
			secrets:     secrets,
			showSecrets: false,
			want:        &WorkflowStart{Name: "deploy"},
		},
		{
			name:        "WorkflowComplete has no secrets to mask",
			ev:          &WorkflowComplete{Name: "deploy", Success: true, DurationMs: 100},
			secrets:     secrets,
			showSecrets: false,
			want:        &WorkflowComplete{Name: "deploy", Success: true, DurationMs: 100},
		},
		{
			name:        "GroupHeader has no secrets to mask",
			ev:          &GroupHeader{Text: "hello"},
			secrets:     secrets,
			showSecrets: false,
			want:        &GroupHeader{Text: "hello"},
		},
		{
			name:        "StepComplete has no secrets to mask",
			ev:          &StepComplete{Name: "build", Success: true},
			secrets:     secrets,
			showSecrets: false,
			want:        &StepComplete{Name: "build", Success: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskEvent(tt.ev, tt.secrets, tt.showSecrets)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMaskEventMutatesInPlace verifies that MaskEvent mutates the
// original event rather than returning a copy.
func TestMaskEventMutatesInPlace(t *testing.T) {
	output := &ContainerOutput{Line: "token is secret123"}
	secrets := []string{"secret123"}

	result := MaskEvent(output, secrets, false)

	assert.Equal(t, "token is [secret]", output.Line)
	assert.Equal(t, "token is [secret]", result.(*ContainerOutput).Line)
	assert.Same(t, output, result)
}
