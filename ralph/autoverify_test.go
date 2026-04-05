//go:build !official_sdk

package ralph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestRunAutoVerify_BuildOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)

	results := runAutoVerify(context.Background(), root, "./...", AutoVerifyBuild)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !strings.Contains(results[0], "AUTO-VERIFY BUILD OK") {
		t.Errorf("expected BUILD OK, got: %v", results)
	}
}

func TestRunAutoVerify_VetLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)

	results := runAutoVerify(context.Background(), root, "./...", AutoVerifyVet)
	found := false
	for _, r := range results {
		if strings.Contains(r, "AUTO-VERIFY VET OK") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected VET OK in results: %v", results)
	}
}

func TestRunAutoVerify_FullLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestNoop(t *testing.T) {}\n"), 0o644)

	results := runAutoVerify(context.Background(), root, "./...", AutoVerifyFull)
	foundTest := false
	for _, r := range results {
		if strings.Contains(r, "AUTO-VERIFY TEST OK") {
			foundTest = true
		}
	}
	if !foundTest {
		t.Errorf("expected TEST OK in results: %v", results)
	}
}

func TestRunAutoVerify_BuildFail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() { invalid syntax }\n"), 0o644)

	results := runAutoVerify(context.Background(), root, "./...", AutoVerifyFull)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !strings.Contains(results[0], "AUTO-VERIFY BUILD FAIL") {
		t.Errorf("expected BUILD FAIL, got: %v", results)
	}
	// Should stop after build failure — no vet or test results.
	if len(results) > 1 {
		t.Errorf("expected only build failure result, got %d results", len(results))
	}
}

func TestRunGoBuild_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)

	result := runGoBuild(context.Background(), root, "./...")
	if result != "" {
		t.Errorf("expected empty string for successful build, got: %s", result)
	}
}

func TestRunGoBuild_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() { broken }\n"), 0o644)

	result := runGoBuild(context.Background(), root, "./...")
	if result == "" {
		t.Error("expected non-empty error output for broken build")
	}
}

func TestRunGoVet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)

	result := runGoVet(context.Background(), root, "./...")
	if result != "" {
		t.Errorf("expected empty string for successful vet, got: %s", result)
	}
}

func TestRunGoTest_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0o644)

	result := runGoTest(context.Background(), root, "./...")
	if result != "" {
		t.Errorf("expected empty string for passing tests, got: %s", result)
	}
}

func TestRunGoTest_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external command test in short mode")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)

	result := runGoTest(context.Background(), root, "./...")
	if result == "" {
		t.Error("expected non-empty error output for failing tests")
	}
}
