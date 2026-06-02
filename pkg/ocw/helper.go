package ocw

import "fmt"

// ResolveOutputs interpolates a raw outputs map (from schema) against the
// current workflow state. It returns a resolved map ready for display.
func ResolveOutputs(outputs map[string]string, state *State) (map[string]string, error) {
	if len(outputs) == 0 {
		return nil, nil
	}
	resolved := make(map[string]string, len(outputs))
	for key, template := range outputs {
		value, err := state.InterpolateTemplate(template)
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", key, err)
		}
		resolved[key] = value
	}
	return resolved, nil
}
