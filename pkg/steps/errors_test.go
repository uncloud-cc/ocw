package steps

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepErrorWithNameAndID(t *testing.T) {
	err := &StepError{
		StepID:   "step123",
		StepName: "Build Image",
		ExitCode: 1,
		Err:      errors.New("docker build failed"),
	}

	expected := `step "Build Image" (id: step123) failed with exit code 1`
	assert.Equal(t, expected, err.Error())
}

func TestStepErrorWithIDOnly(t *testing.T) {
	err := &StepError{
		StepID:   "step456",
		StepName: "",
		ExitCode: 2,
		Err:      nil,
	}

	expected := `step step456 failed with exit code 2`
	assert.Equal(t, expected, err.Error())
}

func TestStepErrorWithoutID(t *testing.T) {
	err := &StepError{
		StepID:   "",
		StepName: "",
		ExitCode: 137,
		Err:      nil,
	}

	expected := `step failed with exit code 137`
	assert.Equal(t, expected, err.Error())
}

func TestStepErrorUnwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &StepError{
		StepID:   "step1",
		ExitCode: 1,
		Err:      underlying,
	}

	unwrapped := errors.Unwrap(err)
	assert.Equal(t, underlying, unwrapped)
}

func TestStepErrorUnwrapNil(t *testing.T) {
	err := &StepError{
		StepID:   "step1",
		ExitCode: 0,
		Err:      nil,
	}

	unwrapped := errors.Unwrap(err)
	assert.Nil(t, unwrapped)
}

func TestStepErrorIs(t *testing.T) {
	// Test that errors.Is works with StepError
	underlying := errors.New("specific error")
	err := &StepError{
		StepID:   "step1",
		ExitCode: 1,
		Err:      underlying,
	}

	assert.True(t, errors.Is(err, underlying))
}
