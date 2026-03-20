package runner

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI color codes
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	italic    = "\033[3m"
	underline = "\033[4m"

	// Foreground colors
	black   = "\033[30m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"

	// Bright foreground colors
	brightBlack   = "\033[90m"
	brightRed     = "\033[91m"
	brightGreen   = "\033[92m"
	brightYellow  = "\033[93m"
	brightBlue    = "\033[94m"
	brightMagenta = "\033[95m"
	brightCyan    = "\033[96m"
	brightWhite   = "\033[97m"

	// Background colors
	bgBlack   = "\033[40m"
	bgRed     = "\033[41m"
	bgGreen   = "\033[42m"
	bgYellow  = "\033[43m"
	bgBlue    = "\033[44m"
	bgMagenta = "\033[45m"
	bgCyan    = "\033[46m"
	bgWhite   = "\033[47m"
)

// Styles provides pre-defined styles for different output types
type Styles struct {
	// Whether colors are enabled
	enabled bool
}

// NewStyles creates a new Styles instance, auto-detecting color support
func NewStyles() *Styles {
	return &Styles{
		enabled: shouldUseColors(),
	}
}

// shouldUseColors determines if we should use colors based on terminal capabilities
func shouldUseColors() bool {
	// Check if stdout is a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	// Check for NO_COLOR environment variable (https://no-color.org/)
	// When present and not an empty string, prevents the addition of ANSI color
	if noColor, exists := os.LookupEnv("NO_COLOR"); exists && noColor != "" {
		return false
	}

	// Check for TERM=dumb
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	// Check for explicit color forcing
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	return true
}

// style applies a style if colors are enabled
func (s *Styles) style(style, text string) string {
	if !s.enabled {
		return text
	}
	return style + text + reset
}

// Header styles - for major sections
func (s *Styles) Header(text string) string {
	return s.style(bold+brightWhite, text)
}

// JobHeader styles the job/workflow header
func (s *Styles) JobHeader(text string) string {
	return s.style(bold+brightCyan, text)
}

// StepHeader styles step headers
func (s *Styles) StepHeader(text string) string {
	return s.style(bold+blue, text)
}

// StepName styles step names
func (s *Styles) StepName(text string) string {
	return s.style(bold+white, text)
}

// Success styles success messages
func (s *Styles) Success(text string) string {
	return s.style(bold+green, text)
}

// Error styles error messages
func (s *Styles) Error(text string) string {
	return s.style(bold+red, text)
}

// Warning styles warning messages
func (s *Styles) Warning(text string) string {
	return s.style(yellow, text)
}

// Info styles informational text
func (s *Styles) Info(text string) string {
	return s.style(cyan, text)
}

// Dim styles dimmed/secondary text
func (s *Styles) Dim(text string) string {
	return s.style(dim, text)
}

// Label styles for labels (like "Type:", "Image:")
func (s *Styles) Label(text string) string {
	return s.style(dim, text)
}

// Value styles for values
func (s *Styles) Value(text string) string {
	return s.style(white, text)
}

// Command styles command text
func (s *Styles) Command(text string) string {
	return s.style(dim+italic, text)
}

// OutputKey styles output keys
func (s *Styles) OutputKey(text string) string {
	return s.style(cyan, text)
}

// OutputValue styles output values
func (s *Styles) OutputValue(text string) string {
	return s.style(white, text)
}

// Icon returns a styled icon
func (s *Styles) Icon(icon string) string {
	return icon
}

// StatusIcon returns an appropriate icon for a status
func (s *Styles) StatusIcon(status StepStatus) string {
	if !s.enabled {
		switch status {
		case StepStatusCompleted:
			return "[OK]"
		case StepStatusFailed:
			return "[FAIL]"
		case StepStatusRunning:
			return "[..]"
		case StepStatusSkipped:
			return "[SKIP]"
		default:
			return "[--]"
		}
	}

	switch status {
	case StepStatusCompleted:
		return s.style(green, "✓")
	case StepStatusFailed:
		return s.style(red, "✗")
	case StepStatusRunning:
		return s.style(blue, "●")
	case StepStatusSkipped:
		return s.style(dim, "○")
	default:
		return s.style(dim, "○")
	}
}

// Divider returns a horizontal divider line
func (s *Styles) Divider(width int) string {
	if width <= 0 {
		width = 60
	}
	return s.Dim(strings.Repeat("─", width))
}

// Box creates a simple box around text
func (s *Styles) Box(title, content string) string {
	lines := strings.Split(content, "\n")
	maxLen := len(title)
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	width := maxLen + 4
	if width < 40 {
		width = 40
	}

	var sb strings.Builder

	// Top border with title
	sb.WriteString(s.Dim("┌─"))
	sb.WriteString(s.Header(" " + title + " "))
	sb.WriteString(s.Dim(strings.Repeat("─", width-len(title)-4)))
	sb.WriteString(s.Dim("┐"))
	sb.WriteString("\n")

	// Content lines
	for _, line := range lines {
		if line == "" {
			continue
		}
		sb.WriteString(s.Dim("│ "))
		sb.WriteString(line)
		sb.WriteString(strings.Repeat(" ", width-len(line)-3))
		sb.WriteString(s.Dim("│"))
		sb.WriteString("\n")
	}

	// Bottom border
	sb.WriteString(s.Dim("└"))
	sb.WriteString(s.Dim(strings.Repeat("─", width-1)))
	sb.WriteString(s.Dim("┘"))
	sb.WriteString("\n")

	return sb.String()
}

// StepBox creates a styled step header box
func (s *Styles) StepBox(name, stepType string, extra map[string]string) string {
	var sb strings.Builder

	// Step header with icon
	sb.WriteString("\n")
	sb.WriteString(s.StepHeader("▶ "))
	sb.WriteString(s.StepName(name))
	sb.WriteString(s.Dim(" ["))
	sb.WriteString(s.Info(stepType))
	sb.WriteString(s.Dim("]"))
	sb.WriteString("\n")

	// Extra info
	for key, value := range extra {
		sb.WriteString("  ")
		sb.WriteString(s.Label(key + ": "))
		sb.WriteString(s.Value(value))
		sb.WriteString("\n")
	}

	return sb.String()
}

// StepComplete formats a step completion message
func (s *Styles) StepComplete(name string, success bool) string {
	if success {
		return fmt.Sprintf("%s %s\n", s.StatusIcon(StepStatusCompleted), s.Success(name+" completed"))
	}
	return fmt.Sprintf("%s %s\n", s.StatusIcon(StepStatusFailed), s.Error(name+" failed"))
}

// JobBox creates the job header
func (s *Styles) JobBox(jobName, workflowName, description string) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")
	sb.WriteString(s.JobHeader("  " + jobName))
	sb.WriteString("\n")
	if workflowName != "" {
		sb.WriteString(s.Dim("  Workflow: "))
		sb.WriteString(s.Value(workflowName))
		sb.WriteString("\n")
	}
	if description != "" {
		sb.WriteString(s.Dim("  "))
		// Truncate long descriptions
		desc := description
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}
		// Replace newlines with spaces for compact display
		desc = strings.ReplaceAll(desc, "\n", " ")
		sb.WriteString(s.Dim(desc))
		sb.WriteString("\n")
	}
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")

	return sb.String()
}

// OutputsBox creates a styled outputs section
func (s *Styles) OutputsBox(title string, outputs map[string]string) string {
	if len(outputs) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(s.Header("  " + title))
	sb.WriteString("\n")
	sb.WriteString(s.Divider(40))
	sb.WriteString("\n")

	for key, value := range outputs {
		sb.WriteString("  ")
		sb.WriteString(s.OutputKey(key))
		sb.WriteString(s.Dim(": "))
		sb.WriteString(s.OutputValue(value))
		sb.WriteString("\n")
	}

	return sb.String()
}

// LogPrefix returns a styled prefix for log output
func (s *Styles) LogPrefix() string {
	if !s.enabled {
		return "  | "
	}
	return s.Dim("  │ ")
}

// SectionHeader creates a section header (like ">>> Running 4 steps in sequence")
func (s *Styles) SectionHeader(text string) string {
	return s.Dim(">>> ") + s.Info(text) + "\n"
}

// Duration formats a duration nicely
func (s *Styles) Duration(d string) string {
	return s.Dim("(" + d + ")")
}

// CompletionBanner creates the final completion banner
func (s *Styles) CompletionBanner(jobName string, duration string, success bool) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")

	if success {
		sb.WriteString(s.Success("  ✓ " + jobName + " completed successfully"))
	} else {
		sb.WriteString(s.Error("  ✗ " + jobName + " failed"))
	}

	sb.WriteString(" ")
	sb.WriteString(s.Duration(duration))
	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")

	return sb.String()
}

// ServiceURL formats a service URL
func (s *Styles) ServiceURL(name, url, protocol string) string {
	return fmt.Sprintf("  %s %s %s\n",
		s.OutputKey(name),
		s.Value(url),
		s.Dim("("+protocol+")"))
}
