//go:build !official_sdk

package ralph

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// detectPackage returns a Go build target from a file path.
// For example, "session/session.go" becomes "./session/...".
// Returns empty string for non-Go files.
func detectPackage(filePath string) string {
	if !strings.HasSuffix(filePath, ".go") {
		return ""
	}
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return "./..."
	}
	return "./" + filepath.ToSlash(dir) + "/..."
}

// runGoBuild runs `go build` for the given package and returns the output.
// Returns empty string on success, or the compiler error output on failure.
func runGoBuild(ctx context.Context, root, pkg string) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", pkg)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			return fmt.Sprintf("go build %s: %v", pkg, err)
		}
		return fmt.Sprintf("go build %s:\n%s", pkg, output)
	}
	return ""
}

// AutoVerifyLevel controls which checks run after write_file calls.
type AutoVerifyLevel string

const (
	// AutoVerifyBuild runs only "go build" (default for backward compat).
	AutoVerifyBuild AutoVerifyLevel = "build"
	// AutoVerifyVet runs "go build" + "go vet".
	AutoVerifyVet AutoVerifyLevel = "vet"
	// AutoVerifyFull runs "go build" + "go vet" + "go test -short".
	AutoVerifyFull AutoVerifyLevel = "full"
)

// runGoVet runs `go vet` for the given package and returns the output.
// Returns empty string on success, or the vet warning output on failure.
func runGoVet(ctx context.Context, root, pkg string) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "vet", pkg)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			return fmt.Sprintf("go vet %s: %v", pkg, err)
		}
		return fmt.Sprintf("go vet %s:\n%s", pkg, output)
	}
	return ""
}

// runGoTest runs `go test -short -count=1` for the given package and returns the output.
// Returns empty string on success (all tests pass), or the test failure output on failure.
func runGoTest(ctx context.Context, root, pkg string) string {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-short", "-count=1", pkg)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(out.String())
		if output == "" {
			return fmt.Sprintf("go test %s: %v", pkg, err)
		}
		// Truncate long test output.
		if len(output) > 2000 {
			output = output[:2000] + "\n... (truncated)"
		}
		return fmt.Sprintf("go test %s:\n%s", pkg, output)
	}
	return ""
}

// runAutoVerify runs verification checks at the given level for a package.
// Returns a slice of result strings (empty slice = all passed).
func runAutoVerify(ctx context.Context, root, pkg string, level AutoVerifyLevel) []string {
	var results []string

	// Always run build.
	if buildOut := runGoBuild(ctx, root, pkg); buildOut != "" {
		results = append(results, "AUTO-VERIFY BUILD FAIL: "+buildOut)
		return results // Don't continue if build fails.
	}
	results = append(results, "AUTO-VERIFY BUILD OK: "+pkg+" compiles")

	if level == AutoVerifyBuild || level == "" {
		return results
	}

	// Run vet.
	if vetOut := runGoVet(ctx, root, pkg); vetOut != "" {
		results = append(results, "AUTO-VERIFY VET FAIL: "+vetOut)
		return results // Don't continue if vet fails.
	}
	results = append(results, "AUTO-VERIFY VET OK: "+pkg)

	if level == AutoVerifyVet {
		return results
	}

	// Run tests (full level).
	if testOut := runGoTest(ctx, root, pkg); testOut != "" {
		results = append(results, "AUTO-VERIFY TEST FAIL: "+testOut)
	} else {
		results = append(results, "AUTO-VERIFY TEST OK: "+pkg)
	}

	return results
}
