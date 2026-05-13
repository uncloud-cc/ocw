package main

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

func main() {
	w := new(flow.Workflow)
	// Pipe creates a linear dependency chain:
	// step1 -> step2 -> step3
	w.Add(
		flow.Pipe(
			flow.Func("step1", func(ctx context.Context) error {
				fmt.Println("running step 1")
				return nil
			}),
			flow.Func("step2", func(ctx context.Context) error {
				fmt.Println("running step 2")
				return nil
			}),
			flow.Func("step3", func(ctx context.Context) error {
				fmt.Println("running step 3")
				return nil
			}),
		),
	)
	if err := w.Do(context.Background()); err != nil {
		panic(err)
	}
}
