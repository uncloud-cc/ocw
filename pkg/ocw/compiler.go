package ocw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	flow "github.com/Azure/go-workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// TODO: Add tests for compiler.go!

// Compile turns an OCW schema (or a specific job within it) into a *flow.Workflow.
// If jobName is empty, it compiles the schema's top-level flow.
// If jobName is non-empty, it compiles the named job from the schema.

// TODO: Test this, using the traversal available in go-workflow: https://deepwiki.com/Azure/go-workflow/4.1-step-wrapping-and-introspection
func Compile(ocw *schema.OCW, jobName string, exec Runtime, state *State, bus *EventBus, baseDir string) (*flow.Workflow, error) {
	w := new(flow.Workflow)

	// Extract flow fields from either the schema or a named job.
	var parallel, sequence []schema.Step
	var switchExpr string
	var cases map[string]schema.Step
	var defaultStep *schema.Step

	if jobName != "" {
		job := GetJob(ocw, jobName)
		if job == nil {
			return nil, fmt.Errorf("job %q not found", jobName)
		}
		parallel = job.Parallel
		sequence = job.Sequence
		switchExpr = job.Switch
		cases = job.Case
		defaultStep = job.Default

		// A single-step job is treated as a one-element sequence.
		if job.Step != nil {
			sequence = []schema.Step{*job.Step}
		}
	} else {
		parallel = ocw.Parallel
		sequence = ocw.Sequence
		switchExpr = ocw.Switch
		cases = ocw.Case
		defaultStep = ocw.Default
	}

	// Determine flow type from the extracted fields.
	var flowType string
	switch {
	case len(parallel) > 0:
		flowType = "parallel"
	case len(sequence) > 0:
		flowType = "sequence"
	case switchExpr != "":
		flowType = "switch"
	}

	// Compile the unified flow.
	switch flowType {
	case "sequence":
		compiled, err := compileSteps(sequence, exec, state, bus, false, baseDir)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Pipe(compiled...))
	case "parallel":
		compiled, err := compileSteps(parallel, exec, state, bus, true, baseDir)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Steps(compiled...))
	case "switch":
		swStep, err := compileSwitchStep(&schema.SwitchStep{
			Switch:  switchExpr,
			Case:    cases,
			Default: defaultStep,
		}, exec, state, bus, baseDir)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Step(swStep))
	default:
		return nil, fmt.Errorf("unsupported flow type: %q", flowType)
	}

	return w, nil
}

func compileSteps(steps []schema.Step, exec Runtime, state *State, bus *EventBus, usePrefix bool, baseDir string) ([]flow.Steper, error) {
	var out []flow.Steper
	for _, s := range steps {
		compiled, err := compileStep(s, exec, state, bus, usePrefix, baseDir)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func compileStep(s schema.Step, exec Runtime, state *State, bus *EventBus, usePrefix bool, baseDir string) (flow.Steper, error) {
	switch {
	case s.RunStep != nil:
		return compileRunStep(s.RunStep, exec, state, bus, usePrefix)
	case s.BuildStep != nil:
		return compileBuildStep(s.BuildStep, exec, state, bus, usePrefix)
	case s.ParallelStep != nil:
		return compileParallelStep(s.ParallelStep, exec, state, bus, baseDir)
	case s.SequenceStep != nil:
		return compileSequenceStep(s.SequenceStep, exec, state, bus, baseDir)
	case s.SwitchStep != nil:
		return compileSwitchStep(s.SwitchStep, exec, state, bus, baseDir)
	case s.WorkflowStep != nil:
		return compileWorkflowStep(s.WorkflowStep, exec, state, bus, baseDir)
	default:
		return nil, fmt.Errorf("unsupported step type")
	}
}

func compileRunStep(s *schema.RunStep, exec Runtime, state *State, bus *EventBus, usePrefix bool) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Image
	}
	prefix := ""
	if usePrefix {
		if s.ID != "" {
			prefix = string(s.ID)
		} else if s.Name != "" {
			prefix = s.Name
		} else {
			prefix = s.Image
		}
	}
	stepType := "run"
	if s.Background {
		stepType = "service"
	}
	return &leafStep{
		name:     name,
		stepType: stepType,
		bus:      bus,
		do: func(ctx context.Context) error {
			interpolatedSchema, err := interpolateRunStepTemplates(s, state)
			if err != nil {
				return fmt.Errorf("Error while interpolating template value for step %s: %v", name, err)
			}

			if s.Background {
				_, err := exec.StartService(ctx, interpolatedSchema, prefix, bus)
				if err != nil {
					return fmt.Errorf("Error while starting service %s: %v", name, err)
				}
				return nil
			}

			outputs, err := exec.Run(ctx, interpolatedSchema, prefix, bus)
			if err != nil {
				return fmt.Errorf("Error while running container %s: %v", name, err)
			}

			if s.ID != "" && len(outputs) > 0 {
				state.SetStepOutputs(string(s.ID), outputs)
			}
			return nil
		},
	}, nil
}

func compileBuildStep(s *schema.BuildStep, exec Runtime, state *State, bus *EventBus, usePrefix bool) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Build.Image
	}
	prefix := ""
	if usePrefix {
		if s.ID != "" {
			prefix = string(s.ID)
		} else if s.Name != "" {
			prefix = s.Name
		} else {
			prefix = s.Build.Image
		}
	}
	return &leafStep{
		name:     name,
		stepType: "build",
		bus:      bus,
		do: func(ctx context.Context) error {
			interpolatedSchema, err := interpolateBuildStepTemplates(s, state)
			if err != nil {
				return fmt.Errorf("Error while interpolating template value for step %s: %v", name, err)
			}

			outputs, err := exec.Build(ctx, interpolatedSchema, prefix, bus)
			if err != nil {
				return fmt.Errorf("Error while trying to run build step %s: %v", name, err)
			}

			if s.ID != "" && len(outputs) > 0 {
				state.SetStepOutputs(string(s.ID), outputs)
			}
			return nil
		},
	}, nil
}

func compileParallelStep(s *schema.ParallelStep, exec Runtime, state *State, bus *EventBus, baseDir string) (flow.Steper, error) {
	compiled, err := compileSteps(s.Parallel, exec, state, bus, true, baseDir)
	if err != nil {
		return nil, err
	}
	return &parallelStep{
		steps: compiled,
		bus:   bus,
		names: stepNames(compiled),
	}, nil
}

func compileSequenceStep(s *schema.SequenceStep, exec Runtime, state *State, bus *EventBus, baseDir string) (flow.Steper, error) {
	compiled, err := compileSteps(s.Sequence, exec, state, bus, false, baseDir)
	if err != nil {
		return nil, err
	}
	return &sequenceStep{
		steps: compiled,
		bus:   bus,
		names: stepNames(compiled),
	}, nil
}

func compileSwitchStep(s *schema.SwitchStep, exec Runtime, state *State, bus *EventBus, baseDir string) (flow.Steper, error) {
	if s.Switch == "" {
		return nil, fmt.Errorf("switch expression is empty")
	}
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range s.Case {
		compiled, err := compileStep(caseStepSchema, exec, state, bus, false, baseDir)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if s.Default != nil {
		compiled, err := compileStep(*s.Default, exec, state, bus, false, baseDir)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: s.Switch,
		cases:    cases,
		def:      def,
		state:    state,
	}, nil
}

func compileWorkflowStep(s *schema.WorkflowStep, exec Runtime, state *State, bus *EventBus, baseDir string) (flow.Steper, error) {
	// Use the workflow source (Uses) as the display name so the header
	// shows where the workflow was imported from.
	name := s.Workflow.Uses
	if name == "" {
		name = s.Name
	}

	// Resolve workflow file path
	var workflowPath string
	if isLocalWorkflowRef(s.Workflow.Uses) {
		workflowPath = filepath.Join(baseDir, s.Workflow.Uses)
	} else {
		ref, err := ParseWorkflowRef(s.Workflow.Uses)
		if err != nil {
			return nil, fmt.Errorf("parse workflow ref %q: %w", s.Workflow.Uses, err)
		}
		fetcher, err := NewFetcher()
		if err != nil {
			return nil, fmt.Errorf("create fetcher: %w", err)
		}
		workflowPath, err = fetcher.Fetch(ref)
		if err != nil {
			return nil, fmt.Errorf("fetch workflow %q: %w", s.Workflow.Uses, err)
		}
	}

	// Parse the referenced workflow
	parsed, err := ParseFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("parse workflow %s: %w", workflowPath, err)
	}
	if err := parsed.Validate(); err != nil {
		return nil, fmt.Errorf("validate workflow %s: %w", workflowPath, err)
	}

	// Create child state starting empty — child workflows do NOT inherit
	// the parent's process environment unless explicitly configured.
	childState := &State{
		Meta:    state.Meta,
		Steps:   state.Steps,
		Inputs:  make(map[string]string),
		Secrets: make(map[string]string),
		Env:     make(map[string]string),
	}

	// Apply the imported workflow's own declared defaults.
	if parsed.Inputs != nil {
		for key, decl := range parsed.Inputs {
			if decl.Value != "" {
				childState.Env[key] = decl.Value
				if decl.IsSecret {
					childState.Secrets[key] = decl.Value
				} else {
					childState.Inputs[key] = decl.Value
				}
			}
		}
	}

	// Apply inheritance rules from the parent workflow.
	inherit := s.Workflow.Inherit
	if inherit == nil {
		// Default: inherit nothing
		inherit = &schema.InheritConfig{Env: schema.InheritNone, Secrets: schema.InheritNone}
	}
	if inherit.Env == schema.InheritAll {
		for k, v := range state.Inputs {
			if _, exists := childState.Inputs[k]; !exists {
				childState.Inputs[k] = v
			}
		}
		for k, v := range state.Env {
			if _, exists := childState.Env[k]; !exists {
				childState.Env[k] = v
			}
		}
	}
	if inherit.Secrets == schema.InheritAll {
		for k, v := range state.Secrets {
			if _, exists := childState.Secrets[k]; !exists {
				childState.Secrets[k] = v
			}
		}
	}

	// Apply explicit inputs overrides from the workflow step config.
	for key, val := range s.Workflow.Inputs {
		interpolated, err := state.InterpolateTemplate(val.Value)
		if err != nil {
			return nil, fmt.Errorf("interpolate inputs %q: %w", key, err)
		}
		childState.Env[key] = interpolated
		if val.IsSecret {
			childState.Secrets[key] = interpolated
		} else {
			childState.Inputs[key] = interpolated
		}
	}

	// Apply explicit secrets overrides from the workflow step config.
	for key, val := range s.Workflow.Secrets {
		if val.Plain != "" {
			childState.Secrets[key] = val.Plain
			childState.Env[key] = val.Plain
		} else if val.Secure != nil {
			childState.Secrets[key] = val.Secure.Secure
			childState.Env[key] = val.Secure.Secure
		}
	}

	// Compile the nested workflow using its own directory as base
	nestedDir := filepath.Dir(workflowPath)
	nestedWorkflow, err := Compile(parsed, "", exec, childState, bus, nestedDir)
	if err != nil {
		return nil, fmt.Errorf("compile workflow %s: %w", workflowPath, err)
	}

	return &workflowStep{
		name:     name,
		workflow: nestedWorkflow,
		bus:      bus,
	}, nil
}

func isLocalWorkflowRef(from string) bool {
	return strings.HasPrefix(from, ".") ||
		strings.HasPrefix(from, "/") ||
		strings.HasPrefix(from, "\\")
}

// ---------------------------------------------------------------------------
// Leaf steps (Run & Build)
// ---------------------------------------------------------------------------
type leafStep struct {
	name     string
	stepType string
	do       func(ctx context.Context) error
	bus      *EventBus
}

func (s *leafStep) String() string { return s.name }
func (s *leafStep) Do(ctx context.Context) error {
	s.bus.Event(&StepStart{
		Name:     s.name,
		StepType: s.stepType,
	})
	err := s.do(ctx)
	s.bus.Event(&StepComplete{
		Name:    s.name,
		Success: err == nil,
	})
	return err
}

// Creates a new copy of schema.RunStep with {{ template }} values interpolated
// This is a separate step so that we can later implement step-by-step debugging,
// middleware plugins etc.
// TODO: Document exactly which fields are interpolated in the docs!
func interpolateRunStepTemplates(s *schema.RunStep, state *State) (*schema.RunStep, error) {
	stepBase, err := interpolateStepbase(&s.StepBase, state)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	image, err := state.InterpolateTemplate(s.Image)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	cmd, err := state.InterpolateTemplate(s.Cmd)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	args := []string{}
	for _, v := range s.Args {
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}

		args = append(args, interpolatedValue)
	}

	entryPoint, err := state.InterpolateTemplate(s.Entrypoint)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	workdir, err := state.InterpolateTemplate(s.Workdir)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	// Background is not interpolated for now

	var healthCheck *schema.HealthCheck
	if s.HealthCheck != nil {
		healthCheckCmd, err := state.InterpolateTemplate(s.HealthCheck.Cmd)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		healthCheck = &schema.HealthCheck{
			Cmd: healthCheckCmd,

			// Not interpolated for now
			Interval:    s.HealthCheck.Interval,
			Timeout:     s.HealthCheck.Timeout,
			Retries:     s.HealthCheck.Retries,
			StartPeriod: s.HealthCheck.StartPeriod,
		}
	}

	// Watch is not interpolated for now
	// CPUs is not interpolated for now
	// Memory is not interpolated for now
	// GPUs is not interpolated for now

	platform, err := state.InterpolateTemplate(s.Platform)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	pullPolicy, err := state.InterpolateTemplate(string(s.Pull))
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	// Quiet is not interpolated for now
	// TTY is not interpolated for now
	// Volumes is not interpolated for now

	return &schema.RunStep{
		StepBase:    stepBase,
		Image:       image,
		Cmd:         cmd,
		Args:        args,
		Entrypoint:  entryPoint,
		Workdir:     workdir,
		HealthCheck: healthCheck,
		Platform:    platform,
		Pull:        schema.PullPolicy(pullPolicy),

		// Not interpolated (for now)
		Background: s.Background,
		Expose:     s.Expose,
		Watch:      s.Watch,
		CPUs:       s.CPUs,
		Memory:     s.Memory,
		GPUs:       s.GPUs,
		Quiet:      s.Quiet,
		TTY:        s.TTY,
		Volumes:    s.Volumes,
	}, nil
}

// Creates a new copy of schema.BuildStep with {{ template }} values interpolated
// TODO: Document exactly which fields are interpolated in the docs!
func interpolateBuildStepTemplates(s *schema.BuildStep, state *State) (*schema.BuildStep, error) {
	stepBase, err := interpolateStepbase(&s.StepBase, state)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	image, err := state.InterpolateTemplate(s.Build.Image)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	contextPath, err := state.InterpolateTemplate(s.Build.Context)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	dockerfile, err := state.InterpolateTemplate(s.Build.Dockerfile)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	target, err := state.InterpolateTemplate(s.Build.Target)
	if err != nil {
		return nil, fmt.Errorf("Interpolation error: %v", err)
	}

	buildArgs := map[string]string{}
	for k, v := range s.Build.BuildArgs {
		interpolatedKey, err := state.InterpolateTemplate(k)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		buildArgs[interpolatedKey] = interpolatedValue
	}

	var platform *schema.StringOrStringSlice
	if s.Build.Platform != nil {
		var ps schema.StringOrStringSlice
		if s.Build.Platform.Single != nil {
			p, err := state.InterpolateTemplate(*s.Build.Platform.Single)
			if err != nil {
				return nil, fmt.Errorf("Interpolation error: %v", err)
			}
			ps.Single = &p
		}
		if len(s.Build.Platform.Multiple) > 0 {
			p := []string{}
			for _, v := range s.Build.Platform.Multiple {
				interpolatedValue, err := state.InterpolateTemplate(v)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				p = append(p, interpolatedValue)
			}
			ps.Multiple = p
		}
		platform = &ps
	}

	cacheFrom := []string{}
	for _, v := range s.Build.CacheFrom {
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		cacheFrom = append(cacheFrom, interpolatedValue)
	}

	cacheTo := []string{}
	for _, v := range s.Build.CacheTo {
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		cacheTo = append(cacheTo, interpolatedValue)
	}

	tags := []string{}
	for _, v := range s.Build.Tags {
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		tags = append(tags, interpolatedValue)
	}

	var buildSecrets *schema.BuildSecrets
	if s.Build.BuildSecrets != nil {
		var bs schema.BuildSecrets
		if s.Build.BuildSecrets.Map != nil {
			m := map[string]string{}
			for k, v := range s.Build.BuildSecrets.Map {
				interpolatedKey, err := state.InterpolateTemplate(k)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				interpolatedValue, err := state.InterpolateTemplate(v)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				m[interpolatedKey] = interpolatedValue
			}
			bs.Map = m
		}
		if len(s.Build.BuildSecrets.Array) > 0 {
			a := []schema.BuildSecretConfig{}
			for _, v := range s.Build.BuildSecrets.Array {
				id, err := state.InterpolateTemplate(v.ID)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				src, err := state.InterpolateTemplate(v.Src)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				env, err := state.InterpolateTemplate(v.Env)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				a = append(a, schema.BuildSecretConfig{ID: id, Src: src, Env: env})
			}
			bs.Array = a
		}
		buildSecrets = &bs
	}

	labels := map[string]string{}
	for k, v := range s.Build.Labels {
		interpolatedKey, err := state.InterpolateTemplate(k)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		labels[interpolatedKey] = interpolatedValue
	}

	var annotation *schema.StringMapOrSlice
	if s.Build.Annotation != nil {
		var ann schema.StringMapOrSlice
		if s.Build.Annotation.Map != nil {
			m := map[string]string{}
			for k, v := range s.Build.Annotation.Map {
				interpolatedKey, err := state.InterpolateTemplate(k)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				interpolatedValue, err := state.InterpolateTemplate(v)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				m[interpolatedKey] = interpolatedValue
			}
			ann.Map = m
		}
		if len(s.Build.Annotation.Slice) > 0 {
			sl := []string{}
			for _, v := range s.Build.Annotation.Slice {
				interpolatedValue, err := state.InterpolateTemplate(v)
				if err != nil {
					return nil, fmt.Errorf("Interpolation error: %v", err)
				}
				sl = append(sl, interpolatedValue)
			}
			ann.Slice = sl
		}
		annotation = &ann
	}

	buildContext := map[string]string{}
	for k, v := range s.Build.BuildContext {
		interpolatedKey, err := state.InterpolateTemplate(k)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return nil, fmt.Errorf("Interpolation error: %v", err)
		}
		buildContext[interpolatedKey] = interpolatedValue
	}

	return &schema.BuildStep{
		StepBase: stepBase,
		Build: schema.BuildConfig{
			Image:        image,
			Context:      contextPath,
			Dockerfile:   dockerfile,
			Target:       target,
			BuildArgs:    buildArgs,
			Platform:     platform,
			CacheFrom:    cacheFrom,
			CacheTo:      cacheTo,
			Tags:         tags,
			BuildSecrets: buildSecrets,
			Labels:       labels,
			Annotation:   annotation,
			BuildContext: buildContext,

			// Not interpolated (for now)
			NoCache:       s.Build.NoCache,
			NoCacheFilter: s.Build.NoCacheFilter,
			Output:        s.Build.Output,
			Push:          s.Build.Push,
			Load:          s.Build.Load,
			Pull:          s.Build.Pull,
			ShmSize:       s.Build.ShmSize,
			Ulimit:        s.Build.Ulimit,
			Progress:      s.Build.Progress,
			Quiet:         s.Build.Quiet,
			Provenance:    s.Build.Provenance,
			SBOM:          s.Build.SBOM,
			Attest:        s.Build.Attest,
			MetadataFile:  s.Build.MetadataFile,
			IIDFile:       s.Build.IIDFile,
		},
		Volumes: s.Volumes,
	}, nil
}

func interpolateStepbase(s *schema.StepBase, state *State) (schema.StepBase, error) {
	name, err := state.InterpolateTemplate(s.Name)
	if err != nil {
		return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
	}

	id, err := state.InterpolateTemplate(s.ID)
	if err != nil {
		return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
	}

	description, err := state.InterpolateTemplate(s.Description)
	if err != nil {
		return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
	}

	// Config is currently not interpolated

	env := map[string]string{}
	for k, v := range s.Env {
		interpolatedKey, err := state.InterpolateTemplate(k)
		if err != nil {
			return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
		}

		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
		}

		env[interpolatedKey] = interpolatedValue
	}

	var envFile *schema.StringOrStringSlice
	if s.EnvFile != nil {
		var efs schema.StringOrStringSlice
		if s.EnvFile.Single != nil {
			e, err := state.InterpolateTemplate(*s.EnvFile.Single)
			if err != nil {
				return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
			}
			efs.Single = &e
		}
		if len(s.EnvFile.Multiple) > 0 {
			e := []string{}
			for _, v := range s.EnvFile.Multiple {
				interpolatedValue, err := state.InterpolateTemplate(v)
				if err != nil {
					return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
				}
				e = append(e, interpolatedValue)
			}
			efs.Multiple = e
		}
		envFile = &efs
	}

	secrets := map[string]string{}
	for k, v := range s.Secrets {
		interpolatedKey, err := state.InterpolateTemplate(k)
		if err != nil {
			return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
		}

		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
		}

		secrets[interpolatedKey] = interpolatedValue
	}

	needs := []string{}
	for _, v := range s.Needs {
		interpolatedValue, err := state.InterpolateTemplate(v)
		if err != nil {
			return schema.StepBase{}, fmt.Errorf("Interpolation error: %v", err)
		}

		needs = append(needs, interpolatedValue)
	}

	return schema.StepBase{
		Name:        name,
		ID:          id,
		Description: description,
		Env:         env,
		EnvFile:     envFile,
		Secrets:     secrets,
		Needs:       needs,

		// Not interpolated (for now)
		Config: s.Config,
	}, nil
}

// ---------------------------------------------------------------------------
// Composite steps (Parallel, Sequence, Switch, Workflow)
// https://deepwiki.com/Azure/go-workflow/4.3-composite-steps-and-subworkflows
// ---------------------------------------------------------------------------
type parallelStep struct {
	flow.SubWorkflow
	steps []flow.Steper
	bus   *EventBus
	names []string
}

func (p *parallelStep) String() string {
	return fmt.Sprintf("parallel(%d)", len(p.steps))
}

func (p *parallelStep) BuildStep() {
	p.Add(flow.Steps(p.steps...))
}

type sequenceStep struct {
	flow.SubWorkflow
	steps []flow.Steper
	bus   *EventBus
	names []string
}

func (s *sequenceStep) String() string {
	return fmt.Sprintf("sequence(%d)", len(s.steps))
}

func (s *sequenceStep) BuildStep() {
	s.Add(flow.Pipe(s.steps...))
}

type switchStep struct {
	flow.SubWorkflow
	evalExpr string
	cases    map[string]flow.Steper
	def      flow.Steper
	state    *State
}

func (s *switchStep) String() string { return "switch" }

func (s *switchStep) BuildStep() {
	eval := &evalStep{expr: s.evalExpr, state: s.state}
	sw := flow.Switch(eval)
	for caseValue, caseStep := range s.cases {
		sw.Case(caseStep, func(_ context.Context, e *evalStep) (bool, error) {
			return e.Result() == caseValue, nil
		})
	}
	if s.def != nil {
		sw.Default(s.def)
	}
	s.Add(sw)
}

type evalStep struct {
	expr   string
	result string
	state  *State
}

func (e *evalStep) String() string { return "eval: " + e.expr }
func (e *evalStep) Do(_ context.Context) error {
	e.result = e.eval()
	return nil
}
func (e *evalStep) Result() string { return e.result }

func (e *evalStep) eval() string {
	expr := strings.TrimSpace(e.expr)

	resolved, err := e.state.InterpolateTemplate(expr)
	if err == nil && resolved != "" {
		return resolved
	}

	return expr
}

type workflowStep struct {
	name     string
	workflow *flow.Workflow
	bus      *EventBus
}

func (w *workflowStep) String() string { return w.name }

func (w *workflowStep) Do(ctx context.Context) error {
	return w.workflow.Do(ctx)
}

// stepNames extracts human-readable names from a slice of compiled steps.
func stepNames(steps []flow.Steper) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		if str, ok := s.(fmt.Stringer); ok {
			names[i] = str.String()
		} else {
			names[i] = fmt.Sprintf("step-%d", i+1)
		}
	}
	return names
}
