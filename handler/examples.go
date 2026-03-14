package handler

// ToolExample represents a concrete usage example for a tool definition.
// Including 1-5 realistic examples in tool definitions improves LLM accuracy
// from 72% to 90% on complex parameter handling (Anthropic research).
//
// Examples are included in the tool description, not the JSON Schema.
type ToolExample struct {
	// Description explains what this example demonstrates.
	Description string

	// Input maps parameter names to example values.
	Input map[string]any

	// Output is the expected result summary (optional).
	Output string
}

// FormatExamples formats tool examples into a string suitable for appending
// to a tool description. The format is designed to be easily parsed by LLMs.
func FormatExamples(examples []ToolExample) string {
	if len(examples) == 0 {
		return ""
	}

	var s string
	s += "\n\nExamples:"
	for i, ex := range examples {
		s += "\n"
		if ex.Description != "" {
			s += "\n  " + ex.Description + ":"
		}
		s += "\n  Input: {"
		first := true
		for k, v := range ex.Input {
			if !first {
				s += ", "
			}
			s += formatKV(k, v)
			first = false
		}
		s += "}"
		if ex.Output != "" {
			s += "\n  Output: " + ex.Output
		}
		if i < len(examples)-1 {
			s += ""
		}
	}
	return s
}

func formatKV(key string, value any) string {
	switch v := value.(type) {
	case string:
		return `"` + key + `": "` + v + `"`
	case bool:
		if v {
			return `"` + key + `": true`
		}
		return `"` + key + `": false`
	case int:
		return `"` + key + `": ` + intToStr(v)
	case float64:
		return `"` + key + `": ` + floatToStr(v)
	default:
		return `"` + key + `": "` + formatAny(v) + `"`
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func floatToStr(f float64) string {
	// Simple float formatting — sufficient for example display
	n := int(f)
	frac := f - float64(n)
	s := intToStr(n)
	if frac != 0 {
		s += "."
		frac *= 100
		if frac < 0 {
			frac = -frac
		}
		fi := int(frac + 0.5)
		if fi < 10 {
			s += "0"
		}
		s += intToStr(fi)
	}
	return s
}

func formatAny(v any) string {
	if v == nil {
		return "null"
	}
	return "..."
}
