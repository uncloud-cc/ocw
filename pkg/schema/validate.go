package schema

import (
	"errors"
	"fmt"
	"regexp"
)

// idPattern validates that IDs start with a letter or underscore and contain only
// letters, underscores, and numbers.
var idPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidationError represents a validation error with a path to the invalid field
type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	msg := fmt.Sprintf("%d validation errors:\n", len(e))
	for _, err := range e {
		msg += "  - " + err.Error() + "\n"
	}
	return msg
}

// ToError returns nil if there are no errors, otherwise returns the ValidationErrors
func (e ValidationErrors) ToError() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// validator helps collect validation errors with path context
type validator struct {
	path   string
	errors ValidationErrors
}

func newValidator() *validator {
	return &validator{}
}

func (v *validator) withPath(path string) *validator {
	newPath := path
	if v.path != "" {
		newPath = v.path + "." + path
	}
	return &validator{path: newPath, errors: v.errors}
}

func (v *validator) withIndex(index int) *validator {
	newPath := fmt.Sprintf("%s[%d]", v.path, index)
	return &validator{path: newPath, errors: v.errors}
}

func (v *validator) addError(msg string) {
	v.errors = append(v.errors, ValidationError{Path: v.path, Message: msg})
}

func (v *validator) addErrorf(format string, args ...any) {
	v.addError(fmt.Sprintf(format, args...))
}

func (v *validator) merge(other *validator) {
	v.errors = append(v.errors, other.errors...)
}

// Validate validates the OCW schema and returns any validation errors
func (o *OCW) Validate() error {
	v := newValidator()

	// Required fields
	if o.SchemaVersion == "" {
		v.withPath("schemaVersion").addError("is required")
	}
	if o.Name == "" {
		v.withPath("name").addError("is required")
	}

	// ID validation
	if o.ID != "" {
		v.validateID("id", o.ID)
	}

	// Validate inputs
	if o.Inputs != nil {
		v.validateInputs(o.Inputs)
	}

	// Check flow types at top level
	flowTypes := 0
	if len(o.Parallel) > 0 {
		flowTypes++
	}
	if len(o.Sequence) > 0 {
		flowTypes++
	}
	if o.Switch != nil {
		flowTypes++
	}

	// Must have either jobs or direct flow control (or both)
	hasJobs := len(o.Jobs) > 0
	hasDirectFlow := flowTypes > 0

	if !hasJobs && !hasDirectFlow {
		v.addError("must have either jobs or flow control (parallel, sequence, or switch)")
	}

	if flowTypes > 1 {
		v.addError("must have only one of: parallel, sequence, or switch (found multiple)")
	}

	// Validate jobs
	if hasJobs {
		for jobID, job := range o.Jobs {
			jv := v.withPath("jobs").withPath(jobID)

			// Validate job ID format
			if !idPattern.MatchString(jobID) {
				jv.addErrorf("invalid job id %q: must start with letter or underscore and contain only letters, numbers, and underscores", jobID)
			}

			jv.validateJob(&job)
			v.merge(jv)
		}
	}

	// Validate direct flow control steps
	if len(o.Parallel) > 0 {
		for i, step := range o.Parallel {
			sv := v.withPath("parallel").withIndex(i)
			sv.validateStep(&step)
			v.merge(sv)
		}
	}
	if len(o.Sequence) > 0 {
		for i, step := range o.Sequence {
			sv := v.withPath("sequence").withIndex(i)
			sv.validateStep(&step)
			v.merge(sv)
		}
	}
	if o.Switch != nil {
		v.validateSwitchFlow(o.Switch, o.Case, o.Default)
	}

	return v.errors.ToError()
}

func (v *validator) validateJob(job *Job) {
	// Check flow types
	flowTypes := 0
	if len(job.Parallel) > 0 {
		flowTypes++
	}
	if len(job.Sequence) > 0 {
		flowTypes++
	}
	if job.Switch != nil {
		flowTypes++
	}
	if job.Step != nil {
		flowTypes++
	}

	if flowTypes == 0 {
		v.addError("must have one of: parallel, sequence, switch, or a single step")
	} else if flowTypes > 1 {
		v.addError("must have only one of: parallel, sequence, switch, or a single step (found multiple)")
	}

	// Validate steps within the job
	if len(job.Parallel) > 0 {
		for i, step := range job.Parallel {
			sv := v.withPath("parallel").withIndex(i)
			sv.validateStep(&step)
			v.merge(sv)
		}
	}
	if len(job.Sequence) > 0 {
		for i, step := range job.Sequence {
			sv := v.withPath("sequence").withIndex(i)
			sv.validateStep(&step)
			v.merge(sv)
		}
	}
	if job.Switch != nil {
		v.validateSwitchFlow(job.Switch, job.Case, job.Default)
	}
	if job.Step != nil {
		v.validateStep(job.Step)
	}
}

func (v *validator) validateID(field string, id string) {
	if !idPattern.MatchString(id) {
		v.withPath(field).addErrorf("invalid id %q: must start with letter or underscore and contain only letters, numbers, and underscores", id)
	}
}

func (v *validator) validateInputs(inputs Inputs) {
	for name, input := range inputs {
		iv := v.withPath("inputs").withPath(name)
		iv.validateInput(&input)
		v.merge(iv)
	}
}

func (v *validator) validateInput(input *Input) {
	switch {
	case input.StringInput != nil:
		// String inputs are valid by default
		if input.StringInput.MinLength != nil && input.StringInput.MaxLength != nil {
			if *input.StringInput.MinLength > *input.StringInput.MaxLength {
				v.addError("minLength cannot be greater than maxLength")
			}
		}
		if input.StringInput.Pattern != "" {
			if _, err := regexp.Compile(input.StringInput.Pattern); err != nil {
				v.addErrorf("invalid pattern %q: %v", input.StringInput.Pattern, err)
			}
		}
	case input.NumberInput != nil:
		if input.NumberInput.Min != nil && input.NumberInput.Max != nil {
			if *input.NumberInput.Min > *input.NumberInput.Max {
				v.addError("min cannot be greater than max")
			}
		}
	case input.BooleanInput != nil:
		// Boolean inputs are always valid
	case input.ChoiceInput != nil:
		if len(input.ChoiceInput.Options) == 0 {
			v.addError("choice input must have at least one option")
		}
		if input.ChoiceInput.Default != "" {
			found := false
			for _, opt := range input.ChoiceInput.Options {
				if opt == input.ChoiceInput.Default {
					found = true
					break
				}
			}
			if !found {
				v.addErrorf("default value %q is not in options", input.ChoiceInput.Default)
			}
		}
	default:
		v.addError("input type not specified")
	}
}

func (v *validator) validateStep(step *Step) {
	switch {
	case step.RunStep != nil:
		v.validateRunStep(step.RunStep)
	case step.BuildStep != nil:
		v.validateBuildStep(step.BuildStep)
	case step.ParallelStep != nil:
		v.validateParallelStep(step.ParallelStep)
	case step.SequenceStep != nil:
		v.validateSequenceStep(step.SequenceStep)
	case step.WorkflowStep != nil:
		v.validateWorkflowStep(step.WorkflowStep)
	case step.SwitchStep != nil:
		v.validateSwitchStep(step.SwitchStep)
	default:
		v.addError("step type not recognized (must have image, build, parallel, sequence, workflow, or switch)")
	}
}

func (v *validator) validateRunStep(step *RunStep) {
	// Name is required for run steps
	if step.Name == "" {
		v.withPath("name").addError("is required")
	}

	// ID validation
	if step.ID != "" {
		v.validateID("id", step.ID)
	}

	// Image is required
	if step.Image == "" {
		v.withPath("image").addError("is required")
	}

	// Expose requires background
	if step.Expose != nil && !step.Background {
		v.withPath("expose").addError("requires background: true (exposed services must run in background)")
	}

	// Validate expose ports
	if step.Expose != nil {
		for i, port := range step.Expose.Ports {
			pv := v.withPath("expose").withIndex(i)
			if port.ContainerPort < 1 || port.ContainerPort > 65535 {
				pv.withPath("containerPort").addErrorf("must be between 1 and 65535, got %d", port.ContainerPort)
			}
			if port.HostPort != 0 && (port.HostPort < 1 || port.HostPort > 65535) {
				pv.withPath("hostPort").addErrorf("must be between 1 and 65535, got %d", port.HostPort)
			}
			// Validate protocol
			if port.Protocol != "" && port.Protocol != "http" && port.Protocol != "https" && port.Protocol != "tcp" && port.Protocol != "udp" {
				pv.withPath("protocol").addErrorf("must be one of: http, https, tcp, udp (got %q)", port.Protocol)
			}
		}
	}
}

func (v *validator) validateBuildStep(step *BuildStep) {
	// Name is required for build steps
	if step.Name == "" {
		v.withPath("name").addError("is required")
	}

	// ID validation
	if step.ID != "" {
		v.validateID("id", step.ID)
	}

	// Image is required in build config
	if step.Build.Image == "" {
		v.withPath("build.image").addError("is required")
	}
}

func (v *validator) validateParallelStep(step *ParallelStep) {
	// ID validation
	if step.ID != "" {
		v.validateID("id", step.ID)
	}

	if len(step.Parallel) == 0 {
		v.withPath("parallel").addError("must have at least one step")
	}

	for i, s := range step.Parallel {
		sv := v.withPath("parallel").withIndex(i)
		sv.validateStep(&s)
		v.merge(sv)
	}
}

func (v *validator) validateSequenceStep(step *SequenceStep) {
	// ID validation
	if step.ID != "" {
		v.validateID("id", step.ID)
	}

	if len(step.Sequence) == 0 {
		v.withPath("sequence").addError("must have at least one step")
	}

	for i, s := range step.Sequence {
		sv := v.withPath("sequence").withIndex(i)
		sv.validateStep(&s)
		v.merge(sv)
	}
}

func (v *validator) validateWorkflowStep(step *WorkflowStep) {
	if step.Workflow.From == "" {
		v.withPath("workflow.from").addError("is required")
	}

	// Validate inherit values if present
	if step.Workflow.Inherit != nil {
		if step.Workflow.Inherit.Secrets != "" &&
			step.Workflow.Inherit.Secrets != InheritNone &&
			step.Workflow.Inherit.Secrets != InheritAll {
			v.withPath("workflow.inherit.secrets").addErrorf("must be %q or %q", InheritNone, InheritAll)
		}
		if step.Workflow.Inherit.Env != "" &&
			step.Workflow.Inherit.Env != InheritNone &&
			step.Workflow.Inherit.Env != InheritAll {
			v.withPath("workflow.inherit.env").addErrorf("must be %q or %q", InheritNone, InheritAll)
		}
	}
}

func (v *validator) validateSwitchStep(step *SwitchStep) {
	// ID validation
	if step.ID != "" {
		v.validateID("id", step.ID)
	}

	if step.Switch == "" {
		v.withPath("switch").addError("is required")
	}

	if len(step.Case) == 0 {
		v.withPath("case").addError("must have at least one case")
	}

	for caseName, caseSteps := range step.Case {
		cv := v.withPath("case").withPath(caseName)
		cv.validateStepOrSteps(&caseSteps)
		v.merge(cv)
	}

	if step.Default != nil {
		dv := v.withPath("default")
		dv.validateStepOrSteps(step.Default)
		v.merge(dv)
	}
}

func (v *validator) validateSwitchFlow(switchExpr *string, cases map[string]StepOrSteps, defaultCase *StepOrSteps) {
	if switchExpr == nil || *switchExpr == "" {
		v.withPath("switch").addError("is required")
	}

	if len(cases) == 0 {
		v.withPath("case").addError("must have at least one case")
	}

	for caseName, caseSteps := range cases {
		cv := v.withPath("case").withPath(caseName)
		cv.validateStepOrSteps(&caseSteps)
		v.merge(cv)
	}

	if defaultCase != nil {
		dv := v.withPath("default")
		dv.validateStepOrSteps(defaultCase)
		v.merge(dv)
	}
}

func (v *validator) validateStepOrSteps(sos *StepOrSteps) {
	if sos.Single != nil {
		v.validateStep(sos.Single)
	} else if sos.Multiple != nil {
		for i, s := range sos.Multiple {
			sv := v.withIndex(i)
			sv.validateStep(&s)
			v.merge(sv)
		}
	} else {
		v.addError("must have at least one step")
	}
}

// ValidateAndParse parses YAML data and validates the result
func ValidateAndParse(data []byte) (*OCW, error) {
	ocw, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if err := ocw.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return ocw, nil
}

// ValidateAndParseFile parses a YAML file and validates the result
func ValidateAndParseFile(path string) (*OCW, error) {
	ocw, err := ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if err := ocw.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return ocw, nil
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var ve ValidationErrors
	return errors.As(err, &ve)
}

// GetValidationErrors extracts validation errors from an error
func GetValidationErrors(err error) ValidationErrors {
	var ve ValidationErrors
	if errors.As(err, &ve) {
		return ve
	}
	return nil
}
