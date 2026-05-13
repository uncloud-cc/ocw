package main

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

func main() {
	w := new(flow.Workflow)
	// Producer: returns a runtime value that the switch cases inspect.
	getEnv := flow.FuncO("get-env", func(ctx context.Context) (string, error) {
		return "staging", nil
	})
	// Branch steps — simple functions, no structs.
	deployProd := flow.Func("deploy-prod", func(ctx context.Context) error {
		fmt.Println("deploying to production")
		return nil
	})
	deployStaging := flow.Func("deploy-staging", func(ctx context.Context) error {
		fmt.Println("deploying to staging")
		return nil
	})
	deployDev := flow.Func("deploy-dev", func(ctx context.Context) error {
		fmt.Println("deploying to dev")
		return nil
	})
	// Switch evaluates the producer's output and runs every matching case.
	// Use mutually exclusive predicates if you want exactly one branch.
	w.Add(
		flow.Switch(getEnv).
			Case(deployProd, func(ctx context.Context, f *flow.Function[struct{}, string]) (bool, error) {
				return f.Output == "production", nil
			}).
			Case(deployStaging, func(ctx context.Context, f *flow.Function[struct{}, string]) (bool, error) {
				return f.Output == "staging", nil
			}).
			Case(deployDev, func(ctx context.Context, f *flow.Function[struct{}, string]) (bool, error) {
				return f.Output == "dev", nil
			}),
	)
	if err := w.Do(context.Background()); err != nil {
		panic(err)
	}
	// Output:
	// deploying to staging
}
