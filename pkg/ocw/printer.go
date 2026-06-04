package ocw

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

// ── Styles (lipgloss-based) ──────────────────────────────────

type Styles struct {
	enabled bool
	// Text styles
	headerStyle      lipgloss.Style
	jobHeaderStyle   lipgloss.Style
	stepHeaderStyle  lipgloss.Style
	stepNameStyle    lipgloss.Style
	successStyle     lipgloss.Style
	errorStyle       lipgloss.Style
	warningStyle     lipgloss.Style
	infoStyle        lipgloss.Style
	dimStyle         lipgloss.Style
	labelStyle       lipgloss.Style
	valueStyle       lipgloss.Style
	commandStyle     lipgloss.Style
	outputKeyStyle   lipgloss.Style
	outputValueStyle lipgloss.Style
	// Box styles
	jobBoxStyle            lipgloss.Style
	outputsBoxStyle        lipgloss.Style
	completionSuccessStyle lipgloss.Style
	completionErrorStyle   lipgloss.Style
}

func isTerminal(fd int) bool {
	_, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	return err == nil
}

func NewStyles() *Styles {
	enabled := isTerminal(int(os.Stdout.Fd()))
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		enabled = false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		enabled = true
	}

	// Adaptive colors that flip between light & dark backgrounds
	adaptiveWhite := lipgloss.AdaptiveColor{Light: "0", Dark: "15"}   // black / bright white
	adaptiveGray := lipgloss.AdaptiveColor{Light: "240", Dark: "250"} // dark gray / light gray
	adaptiveCyan := lipgloss.AdaptiveColor{Light: "6", Dark: "14"}    // cyan / bright cyan
	adaptiveBlue := lipgloss.AdaptiveColor{Light: "4", Dark: "12"}    // blue / bright blue
	adaptiveGreen := lipgloss.AdaptiveColor{Light: "2", Dark: "10"}   // green / bright green
	adaptiveRed := lipgloss.AdaptiveColor{Light: "1", Dark: "9"}      // red / bright red
	adaptiveYellow := lipgloss.AdaptiveColor{Light: "3", Dark: "11"}  // yellow / bright yellow

	return &Styles{
		enabled:          enabled,
		headerStyle:      lipgloss.NewStyle().Bold(true).Foreground(adaptiveWhite),
		jobHeaderStyle:   lipgloss.NewStyle().Bold(true).Foreground(adaptiveCyan),
		stepHeaderStyle:  lipgloss.NewStyle().Bold(true).Foreground(adaptiveBlue),
		stepNameStyle:    lipgloss.NewStyle().Bold(true).Foreground(adaptiveWhite),
		successStyle:     lipgloss.NewStyle().Bold(true).Foreground(adaptiveGreen),
		errorStyle:       lipgloss.NewStyle().Bold(true).Foreground(adaptiveRed),
		warningStyle:     lipgloss.NewStyle().Foreground(adaptiveYellow),
		infoStyle:        lipgloss.NewStyle().Foreground(adaptiveCyan),
		dimStyle:         lipgloss.NewStyle().Foreground(adaptiveGray),
		labelStyle:       lipgloss.NewStyle().Foreground(adaptiveGray),
		valueStyle:       lipgloss.NewStyle().Foreground(adaptiveWhite),
		commandStyle:     lipgloss.NewStyle().Foreground(adaptiveGray).Italic(true),
		outputKeyStyle:   lipgloss.NewStyle().Foreground(adaptiveCyan),
		outputValueStyle: lipgloss.NewStyle().Foreground(adaptiveWhite),
		jobBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(adaptiveCyan).
			Padding(1, 1),
		outputsBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false).
			BorderForeground(adaptiveGray).
			Padding(0, 2),
		completionSuccessStyle: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false).
			BorderForeground(adaptiveGreen).
			Padding(0, 2),
		completionErrorStyle: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false).
			BorderForeground(adaptiveRed).
			Padding(0, 2),
	}
}

func (s *Styles) render(sty lipgloss.Style, text string) string {
	if !s.enabled {
		return text
	}
	return sty.Render(text)
}

func (s *Styles) Header(text string) string      { return s.render(s.headerStyle, text) }
func (s *Styles) JobHeader(text string) string   { return s.render(s.jobHeaderStyle, text) }
func (s *Styles) StepHeader(text string) string  { return s.render(s.stepHeaderStyle, text) }
func (s *Styles) StepName(text string) string    { return s.render(s.stepNameStyle, text) }
func (s *Styles) Success(text string) string     { return s.render(s.successStyle, text) }
func (s *Styles) Error(text string) string       { return s.render(s.errorStyle, text) }
func (s *Styles) Warning(text string) string     { return s.render(s.warningStyle, text) }
func (s *Styles) Info(text string) string        { return s.render(s.infoStyle, text) }
func (s *Styles) Dim(text string) string         { return s.render(s.dimStyle, text) }
func (s *Styles) Label(text string) string       { return s.render(s.labelStyle, text) }
func (s *Styles) Value(text string) string       { return s.render(s.valueStyle, text) }
func (s *Styles) Command(text string) string     { return s.render(s.commandStyle, text) }
func (s *Styles) OutputKey(text string) string   { return s.render(s.outputKeyStyle, text) }
func (s *Styles) OutputValue(text string) string { return s.render(s.outputValueStyle, text) }

func (s *Styles) StatusIcon(success bool) string {
	if !s.enabled {
		if success {
			return "[OK]"
		}
		return "[FAIL]"
	}
	if success {
		return s.successStyle.Render("✓")
	}
	return s.errorStyle.Render("✗")
}

func (s *Styles) Divider(width int) string {
	if width <= 0 {
		width = 60
	}
	return s.render(s.dimStyle, strings.Repeat("─", width))
}

func (s *Styles) StepBox(name, stepType string, extra map[string]string) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(s.StepHeader("▶ "))
	sb.WriteString(s.StepName(name))
	sb.WriteString(s.Dim(" ["))
	sb.WriteString(s.Info(stepType))
	sb.WriteString(s.Dim("]"))
	sb.WriteString("\n")
	for key, value := range extra {
		sb.WriteString("  ")
		sb.WriteString(s.Label(key + ": "))
		sb.WriteString(s.Value(value))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (s *Styles) StepComplete(name string, success bool) string {
	if success {
		return fmt.Sprintf("%s %s\n", s.StatusIcon(true), s.Success(name+" completed"))
	}
	return fmt.Sprintf("%s %s\n", s.StatusIcon(false), s.Error(name+" failed"))
}

func (s *Styles) JobBox(jobName, dir string, loadedFiles []string) string {
	var sb strings.Builder
	sb.WriteString(s.JobHeader(jobName))
	if dir != "" {
		sb.WriteString("\n")
		sb.WriteString(s.Dim("Directory: "))
		sb.WriteString(s.Value(dir))
	}
	for _, f := range loadedFiles {
		sb.WriteString("\n")
		sb.WriteString(s.Dim("Loaded: "))
		sb.WriteString(s.Value(f))
	}
	return "\n" + s.jobBoxStyle.Render(sb.String()) + "\n"
}

func (s *Styles) OutputsBox(title string, outputs map[string]string) string {
	if len(outputs) == 0 {
		return ""
	}
	maxLen := 0
	for k := range outputs {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	var sb strings.Builder
	sb.WriteString(s.Header(title))
	sb.WriteString("\n")
	first := true
	for key, value := range outputs {
		if first {
			first = false
		} else {
			sb.WriteString("\n")
		}
		sb.WriteString(s.OutputKey(key))
		sb.WriteString(s.Dim(":" + strings.Repeat(" ", maxLen-len(key)+1)))
		sb.WriteString(s.OutputValue(value))
	}
	return "\n" + s.outputsBoxStyle.Render(sb.String()) + "\n"
}

func (s *Styles) SectionHeader(text string) string {
	return s.Dim(">>> ") + s.Info(text) + "\n"
}

func (s *Styles) Duration(d string) string {
	return s.Dim("(" + d + ")")
}

func (s *Styles) CompletionBanner(name, duration string, success bool) string {
	var text string
	if success {
		text = s.Success("✓ " + name + " completed successfully")
	} else {
		text = s.Error("✗ " + name + " failed")
	}
	text += " " + s.Duration(duration)
	if success {
		return "\n" + s.completionSuccessStyle.Render(text) + "\n"
	}
	return "\n" + s.completionErrorStyle.Render(text) + "\n"
}

// ── Printer: pretty output OR NDJSON protocol ───────────────

// Printer provides either:
//   - Pretty ANSI output to stdout (default)
//   - NDJSON event stream to stdout (when jsonMode is true)
//
// The NDJSON mode is a stable machine protocol for third-party tools.
// Every pretty-printed concept has a corresponding JSON event.
type Printer struct {
	styles      *Styles
	stdout      io.Writer
	secrets     []string
	showSecrets bool
	jsonMode    bool
	mu          sync.Mutex
}

// NewPrinter creates a new Printer.
// If jsonMode is true, all output is emitted as NDJSON events to stdout.
// secrets are values that will be masked in output (unless showSecrets is true).
func NewPrinter(jsonMode, showSecrets bool, secrets []string) *Printer {
	return &Printer{
		styles:      NewStyles(),
		stdout:      os.Stdout,
		secrets:     secrets,
		showSecrets: showSecrets,
		jsonMode:    jsonMode,
	}
}

// maskSecrets replaces secret values with [secret] in text.
func (p *Printer) maskSecrets(text string) string {
	if p.showSecrets {
		return text
	}
	result := text
	for _, s := range p.secrets {
		if s != "" {
			result = strings.ReplaceAll(result, s, "[secret]")
		}
	}
	return result
}

// maskAny recursively masks secret values inside maps, slices, and strings.
func (p *Printer) maskAny(v any) any {
	if p.showSecrets {
		return v
	}
	switch x := v.(type) {
	case string:
		return p.maskSecrets(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = p.maskAny(v)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = p.maskSecrets(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = p.maskAny(v)
		}
		return out
	case []string:
		out := make([]string, len(x))
		for i, v := range x {
			out[i] = p.maskSecrets(v)
		}
		return out
	default:
		return v
	}
}

// emitJSON writes a single NDJSON line to stdout with secret masking.
func (p *Printer) emitJSON(event string, fields map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := map[string]any{
		"event":     event,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}
	for k, v := range fields {
		m[k] = p.maskAny(v)
	}
	data, _ := json.Marshal(m)
	fmt.Fprintln(p.stdout, string(data))
}

// ── Pretty output helpers (no-ops when jsonMode) ────────────

func (p *Printer) writePretty(s string) {
	if p.jsonMode {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(p.stdout, p.maskSecrets(s))
}

func (p *Printer) Printf(format string, args ...any) {
	p.writePretty(fmt.Sprintf(format, args...))
}

// ── Protocol events (JSON + pretty) ──────────────────────────

// PrintJobStart prints the workflow/job header.
func (p *Printer) PrintJobStart(jobName, dir string, loadedFiles []string) {
	if p.jsonMode {
		p.emitJSON(EventWorkflowStart, map[string]any{
			"name":         jobName,
			"directory":    dir,
			"loaded_files": loadedFiles,
		})
		return
	}
	p.writePretty(p.styles.JobBox(jobName, dir, loadedFiles))
}

// PrintStepStart prints a step start.
func (p *Printer) PrintStepStart(name, stepType string, extra map[string]string) {
	if p.jsonMode {
		fields := map[string]any{
			"name": name,
			"type": stepType,
		}
		for k, v := range extra {
			fields[k] = v
		}
		p.emitJSON(EventStepStart, fields)
		return
	}
	p.writePretty(p.styles.StepBox(name, stepType, extra))
}

// PrintStepComplete prints a step completion.
func (p *Printer) PrintStepComplete(name string, success bool) {
	if p.jsonMode {
		p.emitJSON(EventStepComplete, map[string]any{
			"name":    name,
			"success": success,
		})
		return
	}
	p.writePretty(p.styles.StepComplete(name, success))
}

// PrintContainerOutput prints a single container output line.
// In pretty mode it formats with prefix and separator.
// In JSON mode it emits a container.output event.
func (p *Printer) PrintContainerOutput(step, stream, line string) {
	if p.jsonMode {
		p.emitJSON(EventContainerOutput, map[string]any{
			"step":   step,
			"stream": stream,
			"line":   p.maskSecrets(line),
		})
		return
	}
	if step != "" {
		p.Printf("%s %s %s\n", p.styles.Value(step), p.styles.Dim("|"), line)
	} else {
		p.Printf("%s\n", line)
	}
}

// PrintOutputs prints workflow outputs.
func (p *Printer) PrintOutputs(title string, outputs map[string]string) {
	if p.jsonMode {
		p.emitJSON(EventWorkflowOutputs, map[string]any{
			"title":   title,
			"outputs": outputs,
		})
		return
	}
	p.writePretty(p.styles.OutputsBox(title, outputs))
}

// PrintCompletionBanner prints the final banner.
func (p *Printer) PrintCompletionBanner(name string, duration time.Duration, success bool) {
	if p.jsonMode {
		p.emitJSON(EventWorkflowComplete, map[string]any{
			"name":        name,
			"success":     success,
			"duration_ms": duration.Milliseconds(),
		})
		return
	}
	p.writePretty(p.styles.CompletionBanner(name, duration.Round(time.Millisecond).String(), success))
}

// PrintSectionHeader prints a group header (e.g. ">>> Running 4 steps in sequence").
func (p *Printer) PrintSectionHeader(text string) {
	if p.jsonMode {
		p.emitJSON(EventGroupHeader, map[string]any{"text": text})
		return
	}
	p.writePretty(p.styles.SectionHeader(text))
}

// PrintDivider prints a divider line.
func (p *Printer) PrintDivider(width int) {
	if p.jsonMode {
		return
	}
	p.writePretty(p.styles.Divider(width) + "\n")
}

// ── Structured logging (JSON events or no-op) ───────────────

func (p *Printer) log(level, msg string, fields map[string]any) {
	if !p.jsonMode {
		return
	}
	m := map[string]any{"message": msg}
	for k, v := range fields {
		m[k] = v
	}
	p.emitJSON(level, m)
}

// Debug logs a debug event.
func (p *Printer) Debug(msg string, fields map[string]any) {
	p.log(EventLogDebug, msg, fields)
}

// Info logs an info event.
func (p *Printer) Info(msg string, fields map[string]any) {
	p.log(EventLogInfo, msg, fields)
}

// Warn logs a warning event.
func (p *Printer) Warn(msg string, fields map[string]any) {
	p.log(EventLogWarn, msg, fields)
}

// Error logs an error event.
func (p *Printer) Error(msg string, fields map[string]any) {
	p.log(EventLogError, msg, fields)
}
