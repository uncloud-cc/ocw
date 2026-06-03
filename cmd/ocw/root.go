package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

var (
	envFiles    []string
	inputsFile  string
	outputsFile string
	verbose     bool
	jsonMode    bool
	showSecrets bool
)

var rootCmd = &cobra.Command{
	Use:   "ocw [file.yaml] [job-name]",
	Short: "ocw is a container-native CI/CD workflow engine that actually runs locally",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listAllJobs()
		}

		var filePath string
		var jobName string

		if len(args) == 1 {
			arg := args[0]
			if strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml") {
				filePath = arg
			} else {
				jobName = arg
				discovered, err := findJobFile(jobName)
				if err != nil {
					return err
				}
				filePath = discovered
			}
		} else {
			filePath = args[0]
			jobName = args[1]
		}

		parsed, err := ocw.ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		if err := parsed.Validate(); err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		workflowDir, err := filepath.Abs(filepath.Dir(filePath))
		if err != nil {
			return fmt.Errorf("resolve workflow directory: %w", err)
		}

		// Auto-load .env from workflow directory and CWD (non-overriding)
		loadedSet := make(map[string]struct{})
		var loadedFiles []string
		for _, dotenvPath := range []string{
			filepath.Join(workflowDir, ".env"),
			".env",
		} {
			if _, err := os.Stat(dotenvPath); err == nil {
				_ = godotenv.Overload(dotenvPath)
				absPath, _ := filepath.Abs(dotenvPath)
				if _, ok := loadedSet[absPath]; !ok {
					loadedSet[absPath] = struct{}{}
					loadedFiles = append(loadedFiles, dotenvPath)
				}
			}
		}

		// Load explicit --env-file(s) (overriding)
		for _, ef := range envFiles {
			if err := godotenv.Overload(ef); err != nil {
				return fmt.Errorf("load env file %q: %w", ef, err)
			}
			absPath, _ := filepath.Abs(ef)
			if _, ok := loadedSet[absPath]; !ok {
				loadedSet[absPath] = struct{}{}
				loadedFiles = append(loadedFiles, ef)
			}
		}

		state, err := ocw.NewState(&parsed.Inputs, inputsFile)
		if err != nil {
			return fmt.Errorf("inputs: %w", err)
		}
		state.Meta["name"] = parsed.Name
		state.Steps = make(map[string]map[string]string)

		if inputsFile != "" {
			loadedFiles = append(loadedFiles, inputsFile)
		}

		// Collect secret values for masking
		var secretValues []string
		for _, v := range state.Secrets {
			secretValues = append(secretValues, v)
		}
		jsonMode = jsonMode || verbose
		printer := ocw.NewPrinter(jsonMode, showSecrets, secretValues)

		exec, err := ocw.NewDockerRuntime(parsed.Volumes, workflowDir, printer)
		if err != nil {
			return fmt.Errorf("runtime: %w", err)
		}
		defer exec.Close()

		var workflow *flow.Workflow
		var job *schema.Job
		displayName := parsed.Name
		if jobName != "" {
			job = ocw.GetJob(parsed, jobName)
			if job == nil {
				return fmt.Errorf("job %q not found in %s", jobName, filePath)
			}
			state.Meta["job"] = jobName
			if job.Name != "" {
				displayName = job.Name
			} else {
				displayName = jobName
			}
			workflow, err = ocw.CompileJob(job, exec, state, printer)
		} else {
			if ocw.HasDirectFlow(parsed) {
				workflow, err = ocw.CompileOCW(parsed, exec, state, printer)
			} else if ocw.HasJobs(parsed) {
				return listJobsInFile(filePath, parsed)
			} else {
				return fmt.Errorf("no workflow flow or jobs found in %s", filePath)
			}
		}
		if err != nil {
			return fmt.Errorf("compile: %w", err)
		}

		printer.Info("workflow_start", map[string]any{
			"name": displayName,
			"file": filePath,
			"job":  jobName,
		})
		printer.PrintJobStart(displayName, workflowDir, loadedFiles)
		start := time.Now()
		runErr := workflow.Do(cmd.Context())
		duration := time.Since(start)

		// Determine which raw outputs to resolve
		var rawOutputs map[string]string
		if jobName != "" {
			rawOutputs = job.Outputs
		} else {
			rawOutputs = parsed.Outputs
		}

		printer.PrintCompletionBanner(displayName, duration, runErr == nil)

		// Resolve and print outputs (only on success)
		if runErr == nil && len(rawOutputs) > 0 {
			resolved, err := ocw.ResolveOutputs(rawOutputs, state)
			if err != nil {
				return fmt.Errorf("resolve outputs: %w", err)
			}
			printer.PrintOutputs("Outputs", resolved)

			if outputsFile != "" {
				data, err := json.MarshalIndent(resolved, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal outputs: %w", err)
				}
				if err := os.WriteFile(outputsFile, data, 0644); err != nil {
					return fmt.Errorf("write outputs file: %w", err)
				}
			}
		}

		if runErr != nil {
			printer.Error("workflow_complete", map[string]any{
				"name":        displayName,
				"duration_ms": duration.Milliseconds(),
				"success":     false,
				"error":       runErr.Error(),
			})
		} else {
			printer.Info("workflow_complete", map[string]any{
				"name":        displayName,
				"duration_ms": duration.Milliseconds(),
				"success":     true,
			})
		}

		if runErr != nil {
			return fmt.Errorf("run: %w", runErr)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringSliceVarP(&envFiles, "env-file", "e", nil, ".env file(s) to load")
	rootCmd.PersistentFlags().StringVarP(&inputsFile, "inputs", "i", "", "JSON file with input overrides")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "debug", "v", false, "Emit NDJSON events to stdout (includes debug info)")
	rootCmd.PersistentFlags().BoolVar(&jsonMode, "json", false, "Emit pure NDJSON protocol to stdout (machine-readable)")
	rootCmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in output")
	rootCmd.PersistentFlags().StringVarP(&outputsFile, "outputs", "o", "", "Write resolved outputs to a JSON file")
}

func Execute() error {
	return rootCmd.Execute()
}

func listAllJobs() error {
	files := findWorkflowFiles()
	if len(files) == 0 {
		return fmt.Errorf("no workflow files found in current directory")
	}

	type jobInfo struct {
		file string
		name string
	}
	var allJobs []jobInfo
	var parsedFiles int

	for _, file := range files {
		schema, err := ocw.ParseFile(file)
		if err != nil {
			continue
		}
		if !ocw.HasJobs(schema) && !ocw.HasDirectFlow(schema) {
			continue
		}
		parsedFiles++
		if ocw.HasJobs(schema) {
			for _, name := range ocw.GetJobNames(schema) {
				allJobs = append(allJobs, jobInfo{file: file, name: name})
			}
		}
	}

	if parsedFiles == 0 {
		return fmt.Errorf("no workflow files found in current directory")
	}
	if len(allJobs) == 0 {
		return fmt.Errorf("no jobs found in any workflow file")
	}

	fmt.Println("Available jobs:")
	fmt.Println()

	currentFile := ""
	for _, job := range allJobs {
		if job.file != currentFile {
			currentFile = job.file
			fmt.Printf("  %s:\n", job.file)
		}
		fmt.Printf("    - %s\n", job.name)
	}

	fmt.Println()
	fmt.Println("Run a job with:\n  ocw <job name>\n  ocw <file.yaml> <job name>")

	return nil
}

func listJobsInFile(filePath string, schema *schema.OCW) error {
	if !ocw.HasJobs(schema) {
		return fmt.Errorf("no jobs found in %s", filePath)
	}
	fmt.Printf("Available jobs in %s:\n\n", filePath)
	for _, name := range ocw.GetJobNames(schema) {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()
	fmt.Printf("Run a job with:\n  ocw <job name>\n  ocw %s <job name>\n\n", filePath)
	return nil
}

func findJobFile(jobName string) (string, error) {
	files := findWorkflowFiles()
	for _, file := range files {
		schema, err := ocw.ParseFile(file)
		if err != nil {
			continue
		}
		if ocw.HasJobs(schema) {
			if job := ocw.GetJob(schema, jobName); job != nil {
				return file, nil
			}
		}
	}
	return "", fmt.Errorf("job %q not found in any workflow file", jobName)
}

func findWorkflowFiles() []string {
	files, _ := filepath.Glob("*.yaml")
	ymlFiles, _ := filepath.Glob("*.yml")
	all := append(files, ymlFiles...)
	var result []string
	for _, f := range all {
		if !strings.HasPrefix(filepath.Base(f), ".") {
			result = append(result, f)
		}
	}
	return result
}
