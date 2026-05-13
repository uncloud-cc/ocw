package main

import (
	"context"
	"fmt"
	"maps"

	flow "github.com/Azure/go-workflow"
)

func main() {
	// Producer: seed the initial map
	producer := flow.FuncO("producer", func(ctx context.Context) (map[string]string, error) {
		return map[string]string{"initial": "setup"}, nil
	})
	// Step 1: add region
	step1 := flow.FuncIO("add-region", func(ctx context.Context, m map[string]string) (map[string]string, error) {
		out := maps.Clone(m)
		out["region"] = "us-west-2"
		return out, nil
	})
	// Step 2: add version
	step2 := flow.FuncIO("add-version", func(ctx context.Context, m map[string]string) (map[string]string, error) {
		out := maps.Clone(m)
		out["version"] = "v2.1.0"
		return out, nil
	})
	// Step 3: add env
	step3 := flow.FuncIO("add-env", func(ctx context.Context, m map[string]string) (map[string]string, error) {
		out := maps.Clone(m)
		out["env"] = "production"
		return out, nil
	})
	// Collector: prints the final accumulated map
	collector := flow.Func("collector", func(ctx context.Context) error {
		return nil
	})
	w := new(flow.Workflow)
	w.Add(
		// Chain: producer → step1 → step2 → step3 → collector
		flow.Step(step1).DependsOn(producer).Input(func(ctx context.Context, f *flow.Function[map[string]string, map[string]string]) error {
			f.Input = producer.Output
			return nil
		}),
		flow.Step(step2).DependsOn(step1).Input(func(ctx context.Context, f *flow.Function[map[string]string, map[string]string]) error {
			f.Input = step1.Output
			return nil
		}),
		flow.Step(step3).DependsOn(step2).Input(func(ctx context.Context, f *flow.Function[map[string]string, map[string]string]) error {
			f.Input = step2.Output
			return nil
		}),
		flow.Step(collector).DependsOn(step3).Input(func(ctx context.Context, _ *flow.Function[struct{}, struct{}]) error {
			fmt.Printf("Result: %v\n", step3.Output)
			return nil
		}),
		flow.Step(producer),
	)
	if err := w.Do(context.Background()); err != nil {
		panic(err)
	}
}
