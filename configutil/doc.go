// Package configutil provides generic JSON config file loading with atomic
// writes. LoadJSON uses Go generics for type-safe deserialization. SaveJSON
// uses the temp-file-and-rename pattern to prevent partial reads on
// concurrent access.
package configutil
