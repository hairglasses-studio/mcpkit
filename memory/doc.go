// Package memory provides an agent memory registry with pluggable storage
// backends. The default InMemoryStore supports namespaced key-value entries
// with TTL and tagging. A middleware is included for automatically attaching
// memory context to tool invocations via the registry chain.
package memory
