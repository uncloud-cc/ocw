package cli

import (
	"fmt"
	"sort"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

// listAvailableJobs prints available jobs from workflow files
func listAvailableJobs(files []string) error {
	fmt.Printf("Available jobs:\n\n")

	jobsByFile := make(map[string][]string)

	for _, file := range files {
		ocw, err := schema.ParseFile(file)
		if err != nil {
			continue
		}

		for _, name := range ocw.GetJobNames() {
			job := ocw.GetJob(name)
			displayName := name
			if job.Name != "" {
				displayName = fmt.Sprintf("%s (%s)", name, job.Name)
			}
			jobsByFile[file] = append(jobsByFile[file], displayName)
		}
	}

	if len(jobsByFile) == 0 {
		fmt.Printf("  No jobs found in workflow files.\n")
		return fmt.Errorf("no jobs available")
	}

	// Sort files for consistent output
	sortedFiles := make([]string, 0, len(jobsByFile))
	for f := range jobsByFile {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Strings(sortedFiles)

	for _, file := range sortedFiles {
		jobs := jobsByFile[file]
		fmt.Printf("  %s:\n", file)
		for _, job := range jobs {
			fmt.Printf("    - %s\n", job)
		}
	}

	fmt.Printf("\nUsage: ocw <job-name>\n")
	return fmt.Errorf("specify a job name to run")
}

// listAvailableJobsWithError lists available jobs and returns an error for missing job
func listAvailableJobsWithError(files []string, missingJob string) error {
	fmt.Printf("Job %q not found.\n\n", missingJob)
	listAvailableJobs(files)
	return fmt.Errorf("job %q not found", missingJob)
}

// printWorkflowSummary displays workflow information after validation
func printWorkflowSummary(ocw *schema.OCW) {
	fmt.Printf("Workflow Summary:\n")
	fmt.Printf("  Name: %s\n", ocw.Name)
	if ocw.ID != "" {
		fmt.Printf("  ID: %s\n", ocw.ID)
	}
	if ocw.Description != "" {
		fmt.Printf("  Description: %s\n", ocw.Description)
	}
	fmt.Printf("  Schema Version: %s\n", ocw.SchemaVersion)

	if ocw.HasDirectFlow() {
		fmt.Printf("  Flow Type: %s\n", ocw.GetFlowType())
		steps := ocw.GetSteps()
		if steps != nil {
			fmt.Printf("  Top-level Steps: %d\n", len(steps))
		}
	}

	if ocw.HasJobs() {
		fmt.Printf("  Jobs: %d\n", len(ocw.Jobs))
		for name, job := range ocw.Jobs {
			if job.Name != "" {
				fmt.Printf("    - %s (%s)\n", name, job.Name)
			} else {
				fmt.Printf("    - %s\n", name)
			}
		}
	}

	if len(ocw.Env) > 0 {
		fmt.Printf("  Environment Variables: %d\n", len(ocw.Env))
	}

	if len(ocw.Secrets) > 0 {
		fmt.Printf("  Secrets: %d\n", len(ocw.Secrets))
	}

	if len(ocw.Outputs) > 0 {
		fmt.Printf("  Outputs: %d\n", len(ocw.Outputs))
	}
}
