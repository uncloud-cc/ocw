package ocw

import (
	"fmt"
	"io"
	"sync"
)

// ── PrettyPrinter: ANSI output, no-ops for logs ────────────

type PrettyPrinter struct {
	out         io.Writer
	styles      *Styles
	secrets     []string
	showSecrets bool
	mu          sync.Mutex
}

func NewPrettyPrinter(out io.Writer, showSecrets bool, secrets []string) *PrettyPrinter {
	return &PrettyPrinter{
		out:         out,
		styles:      NewStyles(),
		secrets:     secrets,
		showSecrets: showSecrets,
	}
}

// Log methods are intentionally no-ops in pretty mode.
func (p *PrettyPrinter) Debug(msg string, fields map[string]any) {}
func (p *PrettyPrinter) Info(msg string, fields map[string]any)  {}
func (p *PrettyPrinter) Warn(msg string, fields map[string]any)  {}
func (p *PrettyPrinter) Error(msg string, fields map[string]any) {}

func (p *PrettyPrinter) Event(ev Event) {
	ev = MaskEvent(ev, p.secrets, p.showSecrets)

	p.mu.Lock()
	defer p.mu.Unlock()

	switch e := ev.(type) {
	case *WorkflowStart:
		fmt.Fprint(p.out, p.styles.JobBox(e.Name, e.Directory, e.LoadedFiles))
	case *WorkflowComplete:
		d := fmt.Sprintf("%dms", e.DurationMs)
		fmt.Fprint(p.out, p.styles.CompletionBanner(e.Name, d, e.Success))
	case *GroupHeader:
		fmt.Fprint(p.out, p.styles.SectionHeader(e.Text))
	case *StepStart:
		fmt.Fprint(p.out, p.styles.StepBox(e.Name, e.StepType, e.Extra))
	case *StepComplete:
		fmt.Fprint(p.out, p.styles.StepComplete(e.Name, e.Success))
	case *ContainerOutput:
		if e.Step != "" {
			fmt.Fprintf(p.out, "%s %s %s\n", p.styles.Value(e.Step), p.styles.Dim("|"), e.Line)
		} else {
			fmt.Fprintln(p.out, e.Line)
		}
	case *WorkflowOutputs:
		fmt.Fprint(p.out, p.styles.OutputsBox(e.Title, e.Outputs))
	}
}


