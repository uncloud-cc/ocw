package ocw

import (
	"errors"
	"testing"
)

func TestErrorsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        error
		b        error
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "first nil",
			a:        nil,
			b:        errors.New("some error"),
			expected: false,
		},
		{
			name:     "second nil",
			a:        errors.New("some error"),
			b:        nil,
			expected: false,
		},
		{
			name:     "same message",
			a:        errors.New("same error"),
			b:        errors.New("same error"),
			expected: true,
		},
		{
			name:     "different message",
			a:        errors.New("error a"),
			b:        errors.New("error b"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorsEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("errorsEqual(%v, %v) = %v; expected %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
