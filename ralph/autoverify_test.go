//go:build !official_sdk

package ralph

import "testing"

func TestDetectPackage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"session/session.go", "./session/..."},
		{"session/store_test.go", "./session/..."},
		{"main.go", "./..."},
		{"", ""},
		{"README.md", ""},
		{"auth/jwt/validator.go", "./auth/jwt/..."},
	}
	for _, tt := range tests {
		got := detectPackage(tt.path)
		if got != tt.want {
			t.Errorf("detectPackage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
