package main

import (
	"context"
	"fmt"
	"time"

	flow "github.com/Azure/go-workflow"
)

func main() {
	w := new(flow.Workflow)
	// These three steps have no dependencies on each other, so they run in parallel.
	w.Add(
		flow.Steps(
			flow.Func("fetch-a", func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				fmt.Println("fetched A")
				return nil
			}),
			flow.Func("fetch-b", func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				fmt.Println("fetched B")
				return nil
			}),
			flow.Func("fetch-c", func(ctx context.Context) error {
				time.Sleep(150 * time.Millisecond)
				fmt.Println("fetched C")
				return nil
			}),
		),
	)
	if err := w.Do(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("all done!")
}
