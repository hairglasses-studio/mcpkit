package mcptest

import "testing"

// mockTB is a minimal testing.TB that captures failure signals.
type mockTB struct {
	testing.TB
	onError func()
}

func (m *mockTB) Helper()                         {}
func (m *mockTB) Log(args ...any)                 {}
func (m *mockTB) Logf(format string, args ...any) {}
func (m *mockTB) Error(args ...any) {
	m.onError()
}
func (m *mockTB) Errorf(format string, args ...any) {
	m.onError()
}
func (m *mockTB) Fatal(args ...any) {
	m.onError()
}
func (m *mockTB) Fatalf(format string, args ...any) {
	m.onError()
}
