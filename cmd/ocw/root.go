package main

import (
	"github.com/spf13/cobra"
	"github.com/uncloud-cc/ocw/pkg/ocw"
)

var (
	envFiles    []string
	inputsFile  string
	outputsFile string
	debugMode   bool
	showSecrets bool
	ciMode      bool
)

var rootCmd = &cobra.Command{
	Use:   "ocw [file.yaml] [job-name]",
	Short: "ocw is a container-native CI/CD workflow engine that actually runs locally",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cli := ocw.NewCLI(ocw.CLIOptions{
			EnvFiles:    envFiles,
			InputsFile:  inputsFile,
			OutputsFile: outputsFile,
			DebugMode:   debugMode,
			ShowSecrets: showSecrets,
			CIMode:      ciMode,
		})
		return cli.Run(cmd.Context(), args)
	},
}

func init() {
	rootCmd.PersistentFlags().StringSliceVarP(&envFiles, "env-file", "e", nil, ".env file(s) to load")
	rootCmd.PersistentFlags().StringVarP(&inputsFile, "inputs", "i", "", "JSON file with input overrides")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Emit pure NDJSON protocol to stdout (machine-readable)")
	rootCmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in output")
	rootCmd.PersistentFlags().StringVarP(&outputsFile, "outputs", "o", "", "Write resolved outputs to a JSON file")
	rootCmd.PersistentFlags().BoolVar(&ciMode, "ci", false, "Force CI mode (non-interactive, exit immediately after workflow)")
}

func Execute() error {
	return rootCmd.Execute()
}
