package ocw

import (
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// GetFlowType returns the flow control type used in this workflow (for direct execution)
func GetFlowType(ocw *schema.OCW) string {
	if len(ocw.Parallel) > 0 {
		return "parallel"
	}
	if len(ocw.Sequence) > 0 {
		return "sequence"
	}
	if ocw.Switch != "" {
		return "switch"
	}
	return ""
}

// HasDirectFlow returns true if the workflow has direct flow control (not just jobs)
func HasDirectFlow(ocw *schema.OCW) bool {
	return GetFlowType(ocw) != ""
}

// HasJobs returns true if the workflow has named jobs
func HasJobs(ocw *schema.OCW) bool {
	return len(ocw.Jobs) > 0
}

// GetJob returns a job by name, or nil if not found
func GetJob(ocw *schema.OCW, name string) *schema.Job {
	if ocw.Jobs == nil {
		return nil
	}
	job, ok := ocw.Jobs[name]
	if !ok {
		return nil
	}
	return &job
}

// GetJobNames returns a list of all job names
func GetJobNames(ocw *schema.OCW) []string {
	if ocw.Jobs == nil {
		return nil
	}
	names := make([]string, 0, len(ocw.Jobs))
	for name := range ocw.Jobs {
		names = append(names, name)
	}
	return names
}

// GetSteps returns all top-level steps regardless of flow type
func GetSteps(ocw *schema.OCW) []schema.Step {
	if len(ocw.Parallel) > 0 {
		return ocw.Parallel
	}
	if len(ocw.Sequence) > 0 {
		return ocw.Sequence
	}
	return nil
}

// GetJobFlowType returns the flow control type used in this job
func GetJobFlowType(job *schema.Job) string {
	if len(job.Parallel) > 0 {
		return "parallel"
	}
	if len(job.Sequence) > 0 {
		return "sequence"
	}
	if job.Switch != "" {
		return "switch"
	}
	if job.Step != nil {
		return "step"
	}
	return ""
}

// GetJobSteps returns all top-level steps regardless of flow type
func GetJobSteps(job *schema.Job) []schema.Step {
	if len(job.Parallel) > 0 {
		return job.Parallel
	}
	if len(job.Sequence) > 0 {
		return job.Sequence
	}
	if job.Step != nil {
		return []schema.Step{*job.Step}
	}
	return nil
}
