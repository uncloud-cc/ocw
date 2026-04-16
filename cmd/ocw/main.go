package main

import (
	"context"
	"fmt"
	"os"

	"github.com/uncloud-cc/ocw/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Run(context.Background(), os.Args[1:], version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
