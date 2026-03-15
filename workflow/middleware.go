package workflow

// NodeMiddleware wraps a node function, analogous to registry.Middleware
// for tools. The nodeName parameter identifies which node is being wrapped.
type NodeMiddleware func(nodeName string, next NodeFunc) NodeFunc

// WrapNodeFunc applies middleware to a single node function. Middleware is
// applied in order: the first middleware is the outermost wrapper.
func WrapNodeFunc(fn NodeFunc, name string, mw ...NodeMiddleware) NodeFunc {
	wrapped := fn
	for i := len(mw) - 1; i >= 0; i-- {
		wrapped = mw[i](name, wrapped)
	}
	return wrapped
}
