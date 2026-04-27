// Package cli provides the command-line interface for OCW using cobra and viper.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Global flags
	cfgFile      string
	workflowFile string
	envFile      string
	validateOnly bool
	showSecrets  bool
	force        bool
	verbose      bool
	jsonOutput   bool
	showVersion  bool

	version string
)

// Execute runs the root command and returns any error
func Execute(v string) error {
	version = v
	return rootCmd.Execute()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ocw [job-name|file.yaml]",
	Short: "Open Container Workflow - container-native CI/CD workflows that run locally",
	Long: `ocw (Open Container Workflow) is a container-native CI/CD workflow tool
that runs workflows locally using Docker or Podman.

Run a workflow directly:
  ocw workflow.yaml       Run the workflow file directly
  ocw -f ci.yaml          Run direct flow from a specific file

Run a workflow job:
  ocw dev                 Run the 'dev' job from all .yaml files in current directory
  ocw build               Run the 'build' job
  ocw -f workflow.yaml build  Run 'build' job from specific file

For more information, visit: https://ocw.dev`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Printf("ocw version %s\n", version)
			return nil
		}

		var jobArg string
		var fileArg string
		if len(args) > 0 {
			arg := args[0]
			// Check if the argument is a YAML file
			if strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml") {
				fileArg = arg
			} else {
				jobArg = arg
			}
		}

		return runCommand(cmd.Context(), fileArg, jobArg)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.ocw.yaml)")
	rootCmd.PersistentFlags().StringVarP(&workflowFile, "file", "f", "", "Workflow file to use (default: auto-discover)")
	rootCmd.PersistentFlags().StringVarP(&envFile, "env", "e", "", "Environment file to load (default: .env)")
	rootCmd.PersistentFlags().BoolVar(&validateOnly, "validate", false, "Only validate the workflow file, don't run it")
	rootCmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in output (unmask secrets)")
	rootCmd.PersistentFlags().BoolVar(&force, "force", false, "Force remove existing containers with the same name")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging of internal steps")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "V", false, "Show version")

	// Add subcommands
	rootCmd.AddCommand(validateCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".ocw" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".ocw")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("OCW")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// runCommand executes the main workflow logic
func runCommand(ctx context.Context, fileArg, jobArg string) error {
	config := &RunConfig{
		WorkflowFile: workflowFile,
		FileArg:      fileArg,
		EnvFile:      envFile,
		JobName:      jobArg,
		ValidateOnly: validateOnly,
		ShowSecrets:  showSecrets,
		Force:        force,
		Verbose:      verbose,
		JSONOutput:   jsonOutput,
	}

	return RunWorkflow(ctx, config)
}
