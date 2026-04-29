package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ocw",
	Short: "ocw is a container-native CI/CD workflow engine that actually runs locally",
	Long: `Open Container Workflow is a CI/CD workflow engine that runs locally.
                In a nutshell all it does is run or build containers for you - both locally
								and in your CI/CD environment through our Github Action.
                Complete documentation is available at https://github.com/uncloud-cc/ocw`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
