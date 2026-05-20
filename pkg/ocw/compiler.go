package ocw

import (
	"context"
	"fmt"
	"os"
	"strings"

	flow "github.com/Azure/go-workflow"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// CompileJob turns an OCW job into a *flow.Workflow.
func CompileJob(job *schema.Job, exec Runtime) (*flow.Workflow, error) {
	w := new(flow.Workflow)
	flowType := GetJobFlowType(job)
	switch flowType {
	case "sequence":
		steps := GetJobSteps(job)
		compiled, err := compileSteps(steps, exec)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Pipe(compiled...))
	case "parallel":
		steps := GetJobSteps(job)
		compiled, err := compileSteps(steps, exec)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Steps(compiled...))
	case "step":
		steps := GetJobSteps(job)
		s, err := compileStep(steps[0], exec)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Step(s))
	case "switch":
		swStep, err := compileJobSwitch(job, exec)
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
func CompileOCW(ocw *schema.OCW, exec Runtime) (*flow.Workflow, error) {
	w := new(flow.Workflow)
	flowType := GetFlowType(ocw)
	switch flowType {
	case "sequence":
		steps := GetSteps(ocw)
		compiled, err := compileSteps(steps, exec)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Pipe(compiled...))
	case "parallel":
		steps := GetSteps(ocw)
		compiled, err := compileSteps(steps, exec)
		if err != nil {
			return nil, err
		}
		w.Add(flow.Steps(compiled...))
	case "switch":
		swStep, err := compileOCWSwitch(ocw, exec)
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
}

func (e *evalStep) String() string { return "eval: " + e.expr }
func (e *evalStep) Do(_ context.Context) error {
	e.result = e.eval()
	return nil
}
func (e *evalStep) Result() string { return e.result }

func (e *evalStep) eval() string {
	expr := strings.TrimSpace(e.expr)
	if strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}") {
		inner := strings.TrimSpace(expr[2 : len(expr)-2])
		if strings.HasPrefix(inner, "env.") {
			key := strings.TrimPrefix(inner, "env.")
			if val := os.Getenv(key); val != "" {
				return val
			}
		}
	}
	return expr
}

// switchStep compiles an OCW switch into a go-workflow Switch branch.
type switchStep struct {
	flow.SubWorkflow
	evalExpr string
	cases    map[string]flow.Steper
	def      flow.Steper
}

func (s *switchStep) BuildStep() {
	eval := &evalStep{expr: s.evalExpr}
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

func compileSteps(steps []schema.Step, exec Runtime) ([]flow.Steper, error) {
	var out []flow.Steper
	for _, s := range steps {
		compiled, err := compileStep(s, exec)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func compileStep(s schema.Step, exec Runtime) (flow.Steper, error) {
	switch {
	case s.RunStep != nil:
		return compileRunStep(s.RunStep, exec)
	case s.BuildStep != nil:
		return compileBuildStep(s.BuildStep, exec)
	case s.ParallelStep != nil:
		return compileParallelStep(s.ParallelStep, exec)
	case s.SequenceStep != nil:
		return compileSequenceStep(s.SequenceStep, exec)
	case s.SwitchStep != nil:
		return compileSwitchStep(s.SwitchStep, exec)
	default:
		return nil, fmt.Errorf("unsupported step type")
	}
}

func compileRunStep(s *schema.RunStep, exec Runtime) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Image
	}
	return &ocwStep{
		name: name,
		do: func(ctx context.Context) error {
			fmt.Printf("\nTriggering run step with step state: %v\n\n", s)
			// TODO: Make sure we interpolate all template values here before we run this thing!
			return exec.Run(ctx, s, nil)
		},
	}, nil
}

func compileBuildStep(s *schema.BuildStep, exec Runtime) (flow.Steper, error) {
	name := s.Name
	if name == "" {
		name = s.Build.Image
	}
	return &ocwStep{
		name: name,
		do: func(ctx context.Context) error {
			fmt.Printf("\nTriggering build step with step state: %v\n\n", s)
			return exec.Build(ctx, s, nil)
		},
	}, nil
}

func compileParallelStep(s *schema.ParallelStep, exec Runtime) (flow.Steper, error) {
	compiled, err := compileSteps(s.Parallel, exec)
	if err != nil {
		return nil, err
	}
	return &parallelStep{steps: compiled}, nil
}

func compileSequenceStep(s *schema.SequenceStep, exec Runtime) (flow.Steper, error) {
	compiled, err := compileSteps(s.Sequence, exec)
	if err != nil {
		return nil, err
	}
	return &sequenceStep{steps: compiled}, nil
}

func compileSwitchStep(s *schema.SwitchStep, exec Runtime) (flow.Steper, error) {
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range s.Case {
		compiled, err := compileStep(caseStepSchema, exec)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if s.Default != nil {
		compiled, err := compileStep(*s.Default, exec)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: s.Switch,
		cases:    cases,
		def:      def,
	}, nil
}

func compileJobSwitch(job *schema.Job, exec Runtime) (flow.Steper, error) {
	if job.Switch == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range job.Case {
		compiled, err := compileStep(caseStepSchema, exec)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if job.Default != nil {
		compiled, err := compileStep(*job.Default, exec)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: *job.Switch,
		cases:    cases,
		def:      def,
	}, nil
}

func compileOCWSwitch(ocw *schema.OCW, exec Runtime) (flow.Steper, error) {
	if ocw.Switch == nil {
		return nil, fmt.Errorf("switch expression is nil")
	}
	cases := make(map[string]flow.Steper)
	for caseValue, caseStepSchema := range ocw.Case {
		compiled, err := compileStep(caseStepSchema, exec)
		if err != nil {
			return nil, err
		}
		cases[caseValue] = compiled
	}

	var def flow.Steper
	if ocw.Default != nil {
		compiled, err := compileStep(*ocw.Default, exec)
		if err != nil {
			return nil, err
		}
		def = compiled
	}

	return &switchStep{
		evalExpr: *ocw.Switch,
		cases:    cases,
		def:      def,
	}, nil
}
