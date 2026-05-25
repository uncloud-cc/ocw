package main

import (
	"fmt"
	"path/filepath"
	"strings"

	flow "github.com/Azure/go-workflow"
	"github.com/spf13/cobra"
	"github.com/uncloud-cc/ocw/internal"
	"github.com/uncloud-cc/ocw/pkg/ocw"
	"github.com/uncloud-cc/ocw/pkg/schema"
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

		schema, err := ocw.ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		if err := schema.Validate(); err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		exec := &internal.DummyRuntime{}

		envMap := make(map[string]string, len(schema.Env))
		for k, v := range schema.Env {
			envMap[k] = v.Value
		}

		state := &ocw.State{
			Meta:  map[string]string{"name": schema.Name, "id": schema.ID},
			Env:   envMap,
			Steps: make(map[string]map[string]string),
		}

		var workflow *flow.Workflow
		if jobName != "" {
			job := ocw.GetJob(schema, jobName)
			if job == nil {
				return fmt.Errorf("job %q not found in %s", jobName, filePath)
			}
			workflow, err = ocw.CompileJob(job, exec, state)
		} else {
			if ocw.HasDirectFlow(schema) {
				workflow, err = ocw.CompileOCW(schema, exec, state)
			} else if ocw.HasJobs(schema) {
				return listJobsInFile(filePath, schema)
			} else {
				return fmt.Errorf("no workflow flow or jobs found in %s", filePath)
			}
		}
		if err != nil {
			return fmt.Errorf("compile: %w", err)
		}

		if err := workflow.Do(cmd.Context()); err != nil {
			return fmt.Errorf("run: %w", err)
		}

		return nil
	},
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
