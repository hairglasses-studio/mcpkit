package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	home, _ := os.UserHomeDir()
	defaultRepo := filepath.Join(home, "hairglasses-studio", "cr8-cli")

	repo := flag.String("repo", defaultRepo, "cr8 repository path")
	jsonOut := flag.Bool("json", false, "emit JSON output from hg-pipeline")
	buildOnly := flag.Bool("build-only", false, "run build stage only")
	testOnly := flag.Bool("test-only", false, "run test stage only")
	flag.Parse()

	args := []string{"run", "./cmd/hg-pipeline", "--repo", *repo}
	if *jsonOut {
		args = append(args, "--json")
	}
	if *buildOnly {
		args = append(args, "--build-only")
	}
	if *testOnly {
		args = append(args, "--test-only")
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cr8:", err)
		os.Exit(1)
	}
}

func repoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
