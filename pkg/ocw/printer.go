package ocw

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// ── ANSI color codes ──────────────────────────────────────────

const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	italic    = "\033[3m"
	underline = "\033[4m"

	black   = "\033[30m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"

	brightBlack   = "\033[90m"
	brightRed     = "\033[91m"
	brightGreen   = "\033[92m"
	brightYellow  = "\033[93m"
	brightBlue    = "\033[94m"
	brightMagenta = "\033[95m"
	brightCyan    = "\033[96m"
	brightWhite   = "\033[97m"
)

// ── Event types for the JSON protocol ────────────────────────

const (
	EventWorkflowStart    = "workflow.start"
	EventWorkflowComplete = "workflow.complete"
	EventGroupHeader      = "group.header"
	EventStepStart        = "step.start"
	EventStepComplete     = "step.complete"
	EventContainerOutput  = "container.output"
	EventWorkflowOutputs  = "workflow.outputs"
	EventLogDebug         = "log.debug"
	EventLogInfo          = "log.info"
	EventLogWarn          = "log.warn"
	EventLogError         = "log.error"
)

// ── Styles (ANSI formatting) ─────────────────────────────────

type Styles struct {
	enabled bool
}

func NewStyles() *Styles {
	return &Styles{enabled: shouldUseColors()}
}

func shouldUseColors() bool {
	if !isTerminal(int(os.Stdout.Fd())) {
		return false
	}
	if noColor, exists := os.LookupEnv("NO_COLOR"); exists && noColor != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	return true
}

func isTerminal(fd int) bool {
	_, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	return err == nil
}

func (s *Styles) style(style, text string) string {
	if !s.enabled {
		return text
	}
	return style + text + reset
}

func (s *Styles) Header(text string) string      { return s.style(bold+brightWhite, text) }
func (s *Styles) JobHeader(text string) string    { return s.style(bold+brightCyan, text) }
func (s *Styles) StepHeader(text string) string  { return s.style(bold+blue, text) }
func (s *Styles) StepName(text string) string    { return s.style(bold+white, text) }
func (s *Styles) Success(text string) string     { return s.style(bold+green, text) }
func (s *Styles) Error(text string) string       { return s.style(bold+red, text) }
func (s *Styles) Warning(text string) string     { return s.style(yellow, text) }
func (s *Styles) Info(text string) string        { return s.style(cyan, text) }
func (s *Styles) Dim(text string) string         { return s.style(dim, text) }
func (s *Styles) Label(text string) string       { return s.style(dim, text) }
func (s *Styles) Value(text string) string       { return s.style(white, text) }
func (s *Styles) Command(text string) string     { return s.style(dim+italic, text) }
func (s *Styles) OutputKey(text string) string   { return s.style(cyan, text) }
func (s *Styles) OutputValue(text string) string { return s.style(white, text) }

func (s *Styles) StatusIcon(success bool) string {
	if !s.enabled {
		if success {
			return "[OK]"
		}
		return "[FAIL]"
	}
	if success {
		return s.style(green, "✓")
	}
	return s.style(red, "✗")
}

func (s *Styles) Divider(width int) string {
	if width <= 0 {
		width = 60
	}
	return s.Dim(strings.Repeat("─", width))
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
	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")
	sb.WriteString(s.JobHeader("  " + jobName))
	sb.WriteString("\n")
	if dir != "" {
		sb.WriteString(s.Dim("  Directory: "))
		sb.WriteString(s.Value(dir))
		sb.WriteString("\n")
	}
	for _, f := range loadedFiles {
		sb.WriteString(s.Dim("  Loaded: "))
		sb.WriteString(s.Value(f))
		sb.WriteString("\n")
	}
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")
	return sb.String()
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
	sb.WriteString("\n")
	sb.WriteString(s.Header("  " + title))
	sb.WriteString("\n")
	sb.WriteString(s.Divider(40))
	sb.WriteString("\n")
	for key, value := range outputs {
		sb.WriteString("  ")
		sb.WriteString(s.OutputKey(key))
		sb.WriteString(s.Dim(":" + strings.Repeat(" ", maxLen-len(key)+1)))
		sb.WriteString(s.OutputValue(value))
		sb.WriteString("\n")
	}
	sb.WriteString(s.Divider(40))
	sb.WriteString("\n")
	return sb.String()
}

func (s *Styles) SectionHeader(text string) string {
	return s.Dim(">>> ") + s.Info(text) + "\n"
}

func (s *Styles) Duration(d string) string {
	return s.Dim("(" + d + ")")
}

func (s *Styles) CompletionBanner(name, duration string, success bool) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")
	if success {
		sb.WriteString(s.Success("  ✓ " + name + " completed successfully"))
	} else {
		sb.WriteString(s.Error("  ✗ " + name + " failed"))
	}
	sb.WriteString(" ")
	sb.WriteString(s.Duration(duration))
	sb.WriteString("\n")
	sb.WriteString(s.Divider(60))
	sb.WriteString("\n")
	return sb.String()
}

// ── Printer: pretty output OR NDJSON protocol ───────────────

// Printer provides either:
//   • Pretty ANSI output to stdout (default)
//   • NDJSON event stream to stdout (when jsonMode is true)
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

// PrintSectionHeader prints a group header.
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
