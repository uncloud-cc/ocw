package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStyles(t *testing.T) {
	styles := NewStyles()
	assert.NotNil(t, styles)
}

func TestStylesHeader(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Header("Test Header")
	assert.Contains(t, result, "Test Header")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Header("Test Header")
	assert.Equal(t, "Test Header", resultDisabled)
}

func TestStylesJobHeader(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.JobHeader("Job Name")
	assert.Contains(t, result, "Job Name")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.JobHeader("Job Name")
	assert.Equal(t, "Job Name", resultDisabled)
}

func TestStylesStepHeader(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.StepHeader("Step Header")
	assert.Contains(t, result, "Step Header")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.StepHeader("Step Header")
	assert.Equal(t, "Step Header", resultDisabled)
}

func TestStylesStepName(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.StepName("step-name")
	assert.Contains(t, result, "step-name")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.StepName("step-name")
	assert.Equal(t, "step-name", resultDisabled)
}

func TestStylesSuccess(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Success("Success Message")
	assert.Contains(t, result, "Success Message")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Success("Success Message")
	assert.Equal(t, "Success Message", resultDisabled)
}

func TestStylesError(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Error("Error Message")
	assert.Contains(t, result, "Error Message")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Error("Error Message")
	assert.Equal(t, "Error Message", resultDisabled)
}

func TestStylesWarning(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Warning("Warning Message")
	assert.Contains(t, result, "Warning Message")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Warning("Warning Message")
	assert.Equal(t, "Warning Message", resultDisabled)
}

func TestStylesInfo(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Info("Info Message")
	assert.Contains(t, result, "Info Message")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Info("Info Message")
	assert.Equal(t, "Info Message", resultDisabled)
}

func TestStylesDim(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Dim("Dimmed text")
	assert.Contains(t, result, "Dimmed text")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Dim("Dimmed text")
	assert.Equal(t, "Dimmed text", resultDisabled)
}

func TestStylesLabel(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Label("Label:")
	assert.Contains(t, result, "Label:")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Label("Label:")
	assert.Equal(t, "Label:", resultDisabled)
}

func TestStylesValue(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Value("value")
	assert.Contains(t, result, "value")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Value("value")
	assert.Equal(t, "value", resultDisabled)
}

func TestStylesDivider(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Divider(10)
	assert.Contains(t, result, "─")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Divider(10)
	assert.Contains(t, resultDisabled, "─")
	assert.Equal(t, strings.Repeat("─", 10), resultDisabled)
}

func TestStylesIcon(t *testing.T) {
	styles := &Styles{enabled: true}

	tests := []struct {
		name     string
		iconType string
		expected string
	}{
		{"success", "✓", "✓"},
		{"error", "✗", "✗"},
		{"info", "ℹ", "ℹ"},
		{"warning", "⚠", "⚠"},
		{"pending", "○", "○"},
		{"running", "◉", "◉"},
		{"build", "▶", "▶"},
		{"unknown", "•", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styles.Icon(tt.iconType)
			assert.Equal(t, tt.expected, result)
		})
	}

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Icon("✓")
	assert.Equal(t, "✓", resultDisabled)
}

func TestStylesStepBox(t *testing.T) {
	styles := &Styles{enabled: true}
	extra := map[string]string{
		"Image": "nginx:latest",
		"Mode":  "background",
	}
	result := styles.StepBox("web", "run", extra)
	assert.Contains(t, result, "web")
	assert.Contains(t, result, "run")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.StepBox("web", "run", extra)
	assert.Contains(t, resultDisabled, "web")
	assert.Contains(t, resultDisabled, "run")
}

func TestStylesServiceURL(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.ServiceURL("web", "http://localhost:8080", "http")
	assert.Contains(t, result, "web")
	assert.Contains(t, result, "http://localhost:8080")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.ServiceURL("web", "http://localhost:8080", "http")
	assert.Contains(t, resultDisabled, "web")
	assert.Contains(t, resultDisabled, "http://localhost:8080")
}

func TestStylesStepComplete(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.StepComplete("web", true)
	assert.Contains(t, result, "web")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.StepComplete("web", true)
	assert.Contains(t, resultDisabled, "web")
}

func TestStylesJobBox(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.JobBox("test-job", "workflow", "Test description")
	assert.Contains(t, result, "test-job")
	assert.Contains(t, result, "workflow")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.JobBox("test-job", "workflow", "Test description")
	assert.Contains(t, resultDisabled, "test-job")
	assert.Contains(t, resultDisabled, "workflow")
}

func TestStylesOutputsBox(t *testing.T) {
	styles := &Styles{enabled: true}
	outputs := map[string]string{
		"image": "nginx:latest",
		"tag":   "v1.0.0",
	}
	result := styles.OutputsBox("Build Outputs", outputs)
	assert.Contains(t, result, "Build Outputs")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.OutputsBox("Build Outputs", outputs)
	assert.Contains(t, resultDisabled, "Build Outputs")
}

func TestStylesSectionHeader(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.SectionHeader("Test Section")
	assert.Contains(t, result, "Test Section")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.SectionHeader("Test Section")
	assert.Contains(t, resultDisabled, "Test Section")
}

func TestStylesDuration(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Duration("2.5s")
	assert.Contains(t, result, "2.5s")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Duration("2.5s")
	assert.Contains(t, resultDisabled, "2.5s")
}

func TestStylesCompletionBanner(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.CompletionBanner("test-job", "5.2s", true)
	assert.Contains(t, result, "test-job")
	assert.Contains(t, result, "5.2s")

	resultFail := styles.CompletionBanner("test-job", "5.2s", false)
	assert.Contains(t, resultFail, "test-job")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.CompletionBanner("test-job", "5.2s", true)
	assert.Contains(t, resultDisabled, "test-job")
}

func TestStylesCommand(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Command("npm install")
	assert.Contains(t, result, "npm install")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Command("npm install")
	assert.Equal(t, "npm install", resultDisabled)
}

func TestStylesOutputKey(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.OutputKey("image")
	assert.Contains(t, result, "image")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.OutputKey("image")
	assert.Equal(t, "image", resultDisabled)
}

func TestStylesOutputValue(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.OutputValue("nginx:latest")
	assert.Contains(t, result, "nginx:latest")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.OutputValue("nginx:latest")
	assert.Equal(t, "nginx:latest", resultDisabled)
}

func TestStylesBox(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.Box("Title", "Content goes here")
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Content goes here")

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.Box("Title", "Content goes here")
	assert.Contains(t, resultDisabled, "Title")
	assert.Contains(t, resultDisabled, "Content goes here")
}

func TestStylesLogPrefix(t *testing.T) {
	styles := &Styles{enabled: true}
	result := styles.LogPrefix()
	assert.NotEmpty(t, result)

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.LogPrefix()
	assert.NotEmpty(t, resultDisabled)
}

func TestStylesStatusIcon(t *testing.T) {
	styles := &Styles{enabled: true}

	tests := []struct {
		name   string
		status StepStatus
	}{
		{"pending", StepStatusPending},
		{"running", StepStatusRunning},
		{"completed", StepStatusCompleted},
		{"failed", StepStatusFailed},
		{"skipped", StepStatusSkipped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styles.StatusIcon(tt.status)
			assert.NotEmpty(t, result)
		})
	}

	stylesDisabled := &Styles{enabled: false}
	resultDisabled := stylesDisabled.StatusIcon(StepStatusCompleted)
	assert.NotEmpty(t, resultDisabled)
}

func TestStylesColorsEnabledByDefault(t *testing.T) {
	styles := NewStyles()
	// We can't reliably test the actual value as it depends on the environment
	// Just verify the struct is created properly
	assert.NotNil(t, styles)
}

func TestStyleMethod(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		style   string
		text    string
	}{
		{
			name:    "colors enabled",
			enabled: true,
			style:   "\033[1m",
			text:    "test",
		},
		{
			name:    "colors disabled",
			enabled: false,
			style:   "\033[1m",
			text:    "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			styles := &Styles{enabled: tt.enabled}
			result := styles.style(tt.style, tt.text)

			if tt.enabled {
				assert.Contains(t, result, tt.text)
				assert.Contains(t, result, tt.style)
			} else {
				assert.Equal(t, tt.text, result)
			}
		})
	}
}
