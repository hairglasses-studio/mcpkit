// Package executil provides context-aware shell command execution with
// timeout support. It wraps os/exec with consistent error formatting,
// separate or combined stdout/stderr capture, and convenience timeout
// helpers. All functions accept a context for cancellation and deadline
// propagation.
package executil
