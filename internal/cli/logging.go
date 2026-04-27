package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Time    string                 `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// JSONWriter is an io.Writer that formats output as JSON
type JSONWriter struct {
	out   io.Writer
	level string
}

func (w *JSONWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	// Trim trailing newline
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	entry := LogEntry{
		Time:    time.Now().Format(time.RFC3339Nano),
		Level:   w.level,
		Message: msg,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}

	_, err = fmt.Fprintln(w.out, string(data))
	return len(p), err
}

// createLogger creates a logger based on configuration
func createLogger(config *RunConfig) *log.Logger {
	if config.JSONOutput {
		// For JSON output, use a custom JSON writer
		writer := &JSONWriter{out: os.Stderr, level: "info"}
		return log.New(writer, "", 0)
	}
	return log.New(os.Stderr, "", log.LstdFlags)
}
