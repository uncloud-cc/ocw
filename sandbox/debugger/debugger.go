package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	flow "github.com/Azure/go-workflow"
)

// StepDebugger implements flow.StepInterceptor for interactive step-by-step
// debugging.  It pauses before each step, shows the step name, and waits for
// a command from stdin.
type StepDebugger struct{}

func (d *StepDebugger) InterceptStep(ctx context.Context, step flow.Steper, next func(context.Context) error) error {
	name := flow.String(step)

	fmt.Printf("\n═══════════════════════════════════════\n")
	fmt.Printf("STEP: %s\n", name)
	fmt.Printf("───────────────────────────────────────\n")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("[c]ontinue  [r]eload  [s]kip  [q]uit > ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("debugger read error: %w", err)
		}

		switch strings.TrimSpace(strings.ToLower(input)) {
		case "c", "continue", "":
			fmt.Printf(">>> Running %s...\n", name)
			start := time.Now()
			err := next(ctx)
			duration := time.Since(start)

			status := flow.StatusFromError(err)
			fmt.Printf("<<< Finished %s in %v\n", name, duration)
			fmt.Printf("    Status: %s\n", status)
			if err != nil {
				fmt.Printf("    Error:  %v\n", err)
			}
			return err

		case "r", "reload":
			fmt.Printf(">>> Reloading %s...\n", name)
			start := time.Now()
			err := next(ctx)
			duration := time.Since(start)

			status := flow.StatusFromError(err)
			fmt.Printf("<<< Finished %s in %v\n", name, duration)
			fmt.Printf("    Status: %s\n", status)
			if err != nil {
				fmt.Printf("    Error:  %v\n", err)
			}
			fmt.Println()
			// Loop back to prompt — let the user decide again.

		case "s", "skip":
			fmt.Printf("--- Skipping %s\n", name)
			return flow.Skip(nil)

		case "q", "quit":
			fmt.Printf("!!! Aborting workflow at %s\n", name)
			return fmt.Errorf("user quit at step %s", name)

		default:
			fmt.Println("Unknown command. Try: c, r, s, q")
		}
	}
}

func ptr[T any](v T) *T { return &v }

func main() {
	// Build a 3-step pipeline with a debugger interceptor.
	// MaxConcurrency=1 forces sequential execution so the debugger output is
	// easy to follow.
	w := &flow.Workflow{
		Option: flow.WorkflowOption{
			MaxConcurrency:     ptr(1),
			StepInterceptors:   []flow.StepInterceptor{&StepDebugger{}},
		},
	}

	w.Add(
		flow.Pipe(
			flow.Func("build", func(ctx context.Context) error {
				fmt.Println("    [build]  compiling...")
				return nil
			}),
			flow.Func("test", func(ctx context.Context) error {
				fmt.Println("    [test]   running tests...")
				// Uncomment the next line to simulate a failure:
				// return fmt.Errorf("unit test #3 failed")
				return nil
			}),
			flow.Func("deploy", func(ctx context.Context) error {
				fmt.Println("    [deploy] pushing to registry...")
				return nil
			}),
		),
	)

	fmt.Println("Starting workflow in DEBUG mode")
	fmt.Println("Commands: c = continue, r = reload, s = skip, q = quit")

	if err := w.Do(context.Background()); err != nil {
		fmt.Printf("\nWorkflow finished with error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nWorkflow completed successfully")
}
