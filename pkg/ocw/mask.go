package ocw

import "strings"

// MaskSecrets replaces secret values with [secret] in text.
func MaskSecrets(text string, secrets []string) string {
	result := text
	for _, s := range secrets {
		if s != "" {
			result = strings.ReplaceAll(result, s, "[secret]")
		}
	}
	return result
}

// MaskStringMap masks secret values in a map[string]string.
func MaskStringMap(m map[string]string, secrets []string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = MaskSecrets(v, secrets)
	}
	return out
}

// MaskAny recursively masks secret values inside maps, slices, and strings.
func MaskAny(v any, secrets []string) any {
	switch x := v.(type) {
	case string:
		return MaskSecrets(x, secrets)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = MaskAny(v, secrets)
		}
		return out
	case map[string]string:
		return MaskStringMap(x, secrets)
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = MaskAny(v, secrets)
		}
		return out
	case []string:
		out := make([]string, len(x))
		for i, v := range x {
			out[i] = MaskSecrets(v, secrets)
		}
		return out
	default:
		return v
	}
}

// MaskMap masks secret values in a map[string]any.
func MaskMap(m map[string]any, secrets []string) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = MaskAny(v, secrets)
	}
	return out
}

// MaskEvent masks secrets in an Event before it is emitted or rendered.
func MaskEvent(ev Event, secrets []string, showSecrets bool) Event {
	if showSecrets {
		return ev
	}

	switch e := ev.(type) {
	case *ContainerOutput:
		e.Line = MaskSecrets(e.Line, secrets)
	case *LogDebug:
		e.Message = MaskSecrets(e.Message, secrets)
		e.Fields = MaskMap(e.Fields, secrets)
	case *LogInfo:
		e.Message = MaskSecrets(e.Message, secrets)
		e.Fields = MaskMap(e.Fields, secrets)
	case *LogWarn:
		e.Message = MaskSecrets(e.Message, secrets)
		e.Fields = MaskMap(e.Fields, secrets)
	case *LogError:
		e.Message = MaskSecrets(e.Message, secrets)
		e.Fields = MaskMap(e.Fields, secrets)
	case *WorkflowOutputs:
		e.Outputs = MaskStringMap(e.Outputs, secrets)
	case *StepStart:
		e.Extra = MaskStringMap(e.Extra, secrets)
	}
	return ev
}
