//go:build !official_sdk

package ralph

import (
	"testing"
)

func TestAutoVerifyLevel_Constants(t *testing.T) {
	t.Parallel()
	// Verify the constants exist and have expected values.
	if AutoVerifyBuild != "build" {
		t.Errorf("AutoVerifyBuild = %q, want 'build'", AutoVerifyBuild)
	}
	if AutoVerifyVet != "vet" {
		t.Errorf("AutoVerifyVet = %q, want 'vet'", AutoVerifyVet)
	}
	if AutoVerifyFull != "full" {
		t.Errorf("AutoVerifyFull = %q, want 'full'", AutoVerifyFull)
	}
}

func TestDetectPackage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"session/session.go", "./session/..."},
		{"session/store_test.go", "./session/..."},
		{"ralph/loop.go", "./ralph/..."},
		{"main.go", "./..."},
		{"README.md", ""},
		{"", ""},
		{"deep/nested/pkg/file.go", "./deep/nested/pkg/..."},
		{"auth/jwt/validator.go", "./auth/jwt/..."},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectPackage(tt.path)
			if got != tt.want {
				t.Errorf("detectPackage(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		output string
		want   float64
	}{
		{"ok  \tgithub.com/foo/bar\t0.5s\tcoverage: 85.2% of statements", 85.2},
		{"coverage: 100.0% of statements", 100.0},
		{"coverage: 0.0% of statements", 0.0},
		{"no test files", 0},
		{"FAIL\tgithub.com/foo/bar", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			got := parseCoverage(tt.output)
			if got != tt.want {
				t.Errorf("parseCoverage(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}
