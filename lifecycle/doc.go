// Package lifecycle provides a production server lifecycle manager with OS
// signal handling, graceful drain, and LIFO shutdown hooks. It integrates
// with the health package to return 503 during drain and ensures in-flight
// requests complete before process exit.
package lifecycle
