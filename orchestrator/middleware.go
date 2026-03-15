package orchestrator

// StageMiddleware wraps a stage function, analogous to registry.Middleware
// for tools. The stageName parameter identifies which stage is being wrapped.
type StageMiddleware func(stageName string, next StageFunc) StageFunc

// WrapStage applies middleware to a single stage function. Middleware is
// applied in order: the first middleware is the outermost wrapper.
func WrapStage(stage StageFunc, name string, mw ...StageMiddleware) StageFunc {
	wrapped := stage
	// Apply in reverse so first middleware is outermost.
	for i := len(mw) - 1; i >= 0; i-- {
		wrapped = mw[i](name, wrapped)
	}
	return wrapped
}

// WrapStages applies middleware to multiple stage functions. names[i]
// corresponds to stages[i]. If len(names) < len(stages), unnamed stages
// get "stage-N". Returns a new slice; the input is not mutated.
func WrapStages(stages []StageFunc, names []string, mw ...StageMiddleware) []StageFunc {
	if len(mw) == 0 {
		// Return a copy even with no middleware to maintain immutability.
		out := make([]StageFunc, len(stages))
		copy(out, stages)
		return out
	}
	out := make([]StageFunc, len(stages))
	for i, stage := range stages {
		name := stageName(names, i)
		out[i] = WrapStage(stage, name, mw...)
	}
	return out
}

func stageName(names []string, i int) string {
	if i < len(names) && names[i] != "" {
		return names[i]
	}
	return "stage-" + itoa(i)
}

// itoa converts a non-negative int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
