package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uncloud-cc/ocw/pkg/ocw"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate [workflow-file]",
	Short: "Validate a workflow file",
	Long: `Validate an OCW workflow file without running it.

This checks that the workflow file is syntactically correct and
follows the OCW schema.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var file string
		if len(args) > 0 {
			file = args[0]
		} else {
			// Use the global file flag or auto-discover
			file = workflowFile
			if file == "" {
				file = viper.GetString("file")
			}
			if file == "" {
				discovered, err := discoverWorkflowFile()
				if err != nil {
					return err
				}
				file = discovered
			}
		}

		// Get absolute path for better error messages
		absFile, err := filepath.Abs(file)
		if err != nil {
			absFile = file
		}

		// Parse the workflow file
		ocwSchema, err := ocw.ParseFile(file)
		if err != nil {
			return fmt.Errorf("failed to parse workflow file %s: %w", absFile, err)
		}

		// Validate the workflow
		if err := ocwSchema.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Printf("✓ Workflow file is valid: %s\n", absFile)
		return nil
	},
}
