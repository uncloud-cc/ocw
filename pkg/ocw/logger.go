package ocw

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// JSONLogger consumes events from a channel and writes them as NDJSON lines.
type JSONLogger struct {
	out io.Writer
	mu  sync.Mutex
}

// NewJSONLogger creates a new NDJSON consumer that writes to out.
func NewJSONLogger(out io.Writer) *JSONLogger {
	return &JSONLogger{out: out}
}

// Run reads events from ch until it is closed.
func (j *JSONLogger) Run(ch <-chan IngestedEvent) {
	for ev := range ch {
		data, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		j.mu.Lock()
		fmt.Fprintln(j.out, string(data))
		j.mu.Unlock()
	}
}

// PrettyPrinter consumes events from a channel and renders them as pretty ANSI output.
type PrettyPrinter struct {
	out    io.Writer
	styles *Styles
}

// NewPrettyPrinter creates a new pretty consumer that writes to out.
func NewPrettyPrinter(out io.Writer) *PrettyPrinter {
	return &PrettyPrinter{out: out, styles: NewStyles()}
}

// Run reads events from ch until it is closed.
func (p *PrettyPrinter) Run(ch <-chan IngestedEvent) {
	for ev := range ch {
		switch e := ev.Event.(type) {
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
}
