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
