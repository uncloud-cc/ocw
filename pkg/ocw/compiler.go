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
			// Interpolate the things!
			cmdParsed, err := state.InterpolateTemplate(s.Cmd)
			if err != nil {
				fmt.Printf("*** Interpolation error: %v", err)
			} else {
				fmt.Printf("Interpolated cmd:\n%s\n\n", cmdParsed)
			}

			outputs, err := exec.Run(ctx, s)
			if err != nil {
				return err
			}

			if s.ID != "" && len(outputs) > 0 {
				state.SetStepOutputs(string(s.ID), outputs)
			}
			return nil
		},
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

			outputs, err := exec.Build(ctx, s)
			if err != nil {
				return err
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
