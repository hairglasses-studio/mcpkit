package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type stepResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code,omitempty"`
}

type result struct {
	Repo    string       `json:"repo"`
	Steps   []stepResult `json:"steps"`
	Success bool         `json:"success"`
}

func main() {
	repo := flag.String("repo", ".", "repository path")
	jsonOut := flag.Bool("json", false, "emit JSON output")
	buildOnly := flag.Bool("build-only", false, "run build stage only")
	testOnly := flag.Bool("test-only", false, "run test stage only")
	flag.Parse()

	res, err := runPipeline(*repo, *buildOnly, *testOnly)
	if err != nil {
		if *jsonOut {
			res.Success = false
			out, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(out))
		}
		fmt.Fprintln(os.Stderr, "hg-pipeline:", err)
		os.Exit(1)
	}
	if *jsonOut {
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(out))
		return
	}
	for _, s := range res.Steps {
		fmt.Printf("%s: %s\n", s.Name, s.Status)
	}
}

func runPipeline(repo string, buildOnly, testOnly bool) (result, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return result{}, err
	}
	detected := detectLanguage(abs)
	if detected == "unknown" {
		return result{Repo: filepath.Base(abs)}, fmt.Errorf("no buildable project detected at %s", abs)
	}
	res := result{Repo: filepath.Base(abs), Success: true}
	if !testOnly {
		if err := runStep(abs, detected, "build"); err != nil {
			res.Steps = append(res.Steps, stepResult{Name: "build", Status: "fail", ExitCode: exitCode(err)})
			res.Success = false
			return res, err
		}
		res.Steps = append(res.Steps, stepResult{Name: "build", Status: "pass"})
	}
	if !buildOnly {
		if detected == "go" {
			if err := runStep(abs, detected, "vet"); err != nil {
				res.Steps = append(res.Steps, stepResult{Name: "vet", Status: "fail", ExitCode: exitCode(err)})
				res.Success = false
				return res, err
			}
			res.Steps = append(res.Steps, stepResult{Name: "vet", Status: "pass"})
		}
		if err := runStep(abs, detected, "test"); err != nil {
			res.Steps = append(res.Steps, stepResult{Name: "test", Status: "fail", ExitCode: exitCode(err)})
			res.Success = false
			return res, err
		}
		res.Steps = append(res.Steps, stepResult{Name: "test", Status: "pass"})
	}
	return res, nil
}

func detectLanguage(repo string) string {
	if exists(filepath.Join(repo, "go.mod")) {
		return "go"
	}
	if exists(filepath.Join(repo, "package.json")) {
		return "node"
	}
	if exists(filepath.Join(repo, "pyproject.toml")) || exists(filepath.Join(repo, "requirements.txt")) || exists(filepath.Join(repo, "setup.py")) {
		return "python"
	}
	return "unknown"
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runStep(repo, lang, step string) error {
	var cmd *exec.Cmd
	switch lang {
	case "go":
		switch step {
		case "build":
			cmd = exec.Command("go", "build", "-buildvcs=false", "./...")
		case "vet":
			cmd = exec.Command("go", "vet", "./...")
		case "test":
			cmd = exec.Command("go", "test", "-buildvcs=false", "./...", "-count=1")
		}
	case "node":
		switch step {
		case "build":
			cmd = exec.Command("npm", "run", "build")
		case "test":
			cmd = exec.Command("npm", "test")
		}
	case "python":
		switch step {
		case "build":
			cmd = exec.Command("python3", "-m", "compileall", ".")
		case "test":
			cmd = exec.Command("python3", "-m", "pytest")
		}
	}
	if cmd == nil {
		return fmt.Errorf("unsupported step %s for language %s", step, lang)
	}
	cmd.Dir = repo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func exitCode(err error) int {
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return 1
}
