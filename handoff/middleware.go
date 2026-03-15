package handoff

// DelegateMiddleware wraps a delegate function, analogous to
// registry.Middleware for tools. The agentName parameter identifies
// which agent the delegation targets.
type DelegateMiddleware func(agentName string, next DelegateFunc) DelegateFunc

// WrapDelegate applies middleware to a delegate function. Middleware is
// applied in order: the first middleware is the outermost wrapper.
func WrapDelegate(fn DelegateFunc, name string, mw ...DelegateMiddleware) DelegateFunc {
	wrapped := fn
	for i := len(mw) - 1; i >= 0; i-- {
		wrapped = mw[i](name, wrapped)
	}
	return wrapped
}

// WithMiddleware returns a new Config with the middleware applied to its
// Delegate function. The original Config is not modified.
// If the Config has no Delegate function, middleware is still recorded
// but will have no effect until a Delegate is set.
func (c Config) WithMiddleware(mw ...DelegateMiddleware) Config {
	newCfg := c
	if c.Delegate != nil && len(mw) > 0 {
		newCfg.Delegate = WrapDelegate(c.Delegate, "", mw...)
	}
	return newCfg
}
