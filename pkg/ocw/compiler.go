package ocw

import (
	"context"
	"fmt"
	"strings"

	flow "github.com/Azure/go-workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// CompileJob turns an OCW job into a *flow.Workflow.
func CompileJob(job *schema.Job, exec Runtime, state *State) (*flow.Workflow, error) {
	w := new(flow.Workflow)
	flowType := GetJobFlowType(job)
	switch flowType {
	case "sequence":
		steps := GetJobSteps(job)
		compiled, err := compileSteps(steps, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Pipe(compiled...))
	case "parallel":
		steps := GetJobSteps(job)
		compiled, err := compileSteps(steps, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Steps(compiled...))
	case "step":
		steps := GetJobSteps(job)
		s, err := compileStep(steps[0], exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Step(s))
	case "switch":
		swStep, err := compileJobSwitch(job, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Step(swStep))
	default:
		return nil, fmt.Errorf("unsupported job flow type: %q", flowType)
	}
	return w, nil
}

// CompileOCW compiles an OCW schema's top-level flow into a workflow.
func CompileOCW(ocw *schema.OCW, exec Runtime, state *State) (*flow.Workflow, error) {
	w := new(flow.Workflow)
	flowType := GetFlowType(ocw)
	switch flowType {
	case "sequence":
		steps := GetSteps(ocw)
		compiled, err := compileSteps(steps, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Pipe(compiled...))
	case "parallel":
		steps := GetSteps(ocw)
		compiled, err := compileSteps(steps, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Steps(compiled...))
	case "switch":
		swStep, err := compileOCWSwitch(ocw, exec, state)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Step(swStep))
	default:
		return nil, fmt.Errorf("unsupported flow type: %q", flowType)
	}
	return w, nil
}

// ocwStep adapts an OCW leaf step into a go-workflow Steper.
type ocwStep struct {
	name string
	do   func(ctx context.Context) error
}

func (s *ocwStep) String() string { return s.name }
func (s *ocwStep) Do(ctx context.Context) error {
	return s.do(ctx)
}

// evalStep evaluates an expression and stores the result for switch branching.
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

// switchStep compiles an OCW switch into a go-workflow Switch branch.
type switchStep struct {
	flow.SubWorkflow
	evalExpr string
	cases    map[string]flow.Steper
	def      flow.Steper
	state    *State
}

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

// parallelStep runs nested steps in parallel.
type parallelStep struct {
	flow.SubWorkflow
	steps []flow.Steper
}

func (p *parallelStep) BuildStep() {
	p.Add(flow.Steps(p.steps...))
}

// sequenceStep runs nested steps in sequence.
type sequenceStep struct {
	flow.SubWorkflow
	steps []flow.Steper
}

func (s *sequenceStep) BuildStep() {
	s.Add(flow.Pipe(s.steps...))
}

func compileSteps(steps []schema.Step, exec Runtime, state *State) ([]flow.Steper, error) {
	var out []flow.Steper
	for _, s := range steps {
		compiled, err := compileStep(s, exec, state)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func compileStep(s schema.Step, exec Runtime, state *State) (flow.Steper, error) {
	switch {
	case s.RunStep != nil:
		return compileRunStep(s.RunStep, exec, state)
	case s.BuildStep != nil:
		return compileBuildStep(s.BuildStep, exec, state)
	case s.ParallelStep != nil:
		return compileParallelStep(s.ParallelStep, exec, state)
	case s.SequenceStep != nil:
		return compileSequenceStep(s.SequenceStep, exec, state)
	case s.SwitchStep != nil:
		return compileSwitchStep(s.SwitchStep, exec, state)
	default:
		return nil, fmt.Errorf("unsupported step type")
	}
}

func compileRunStep(s *schema.RunStep, exec Runtime, state *State) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Image
	}
	return &ocwStep{
		name: name,
		do: func(ctx context.Context) error {
			interpolatedSchema, err := interpolateRunStepTemplates(s, state)
			if err != nil {
				return fmt.Errorf("Error while interpolating template value for step %s: %v", name, err)
			}

			outputs, err := exec.Run(ctx, interpolatedSchema)
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

func compileBuildStep(s *schema.BuildStep, exec Runtime, state *State) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Build.Image
	}
	return &ocwStep{
		name: name,
		do: func(ctx context.Context) error {
			fmt.Printf("\nTriggering build step with step state: %v\n\n", s)

			interpolatedSchema, err := interpolateBuildStepTemplates(s, state)
			if err != nil {
				return fmt.Errorf("Error while interpolating template value for step %s: %v", name, err)
			}

			outputs, err := exec.Build(ctx, interpolatedSchema)
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

func compileParallelStep(s *schema.ParallelStep, exec Runtime, state *State) (flow.Steper, error) {
	compiled, err := compileSteps(s.Parallel, exec, state)
	if err != nil {
		return nil, err
	}
	return &parallelStep{steps: compiled}, nil
}

func compileSequenceStep(s *schema.SequenceStep, exec Runtime, state *State) (flow.Steper, error) {
	compiled, err := compileSteps(s.Sequence, exec, state)
	if err != nil {
		return nil, err
	}
	return &sequenceStep{steps: compiled}, nil
}

func compileSwitchStep(s *schema.SwitchStep, exec Runtime, state *State) (flow.Steper, error) {
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range s.Case {
		compiled, err := compileStep(caseStepSchema, exec, state)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if s.Default != nil {
		compiled, err := compileStep(*s.Default, exec, state)
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

func compileJobSwitch(job *schema.Job, exec Runtime, state *State) (flow.Steper, error) {
	if job.Switch == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range job.Case {
		compiled, err := compileStep(caseStepSchema, exec, state)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if job.Default != nil {
		compiled, err := compileStep(*job.Default, exec, state)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: *job.Switch,
		cases:    cases,
		def:      def,
		state:    state,
	}, nil
}

func compileOCWSwitch(ocw *schema.OCW, exec Runtime, state *State) (flow.Steper, error) {
	if ocw.Switch == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range ocw.Case {
		compiled, err := compileStep(caseStepSchema, exec, state)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if ocw.Default != nil {
		compiled, err := compileStep(*ocw.Default, exec, state)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: *ocw.Switch,
		cases:    cases,
		def:      def,
		state:    state,
	}, nil
}
