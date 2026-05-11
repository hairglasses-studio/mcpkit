package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type repoEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Language     string `json:"language"`
	Lifecycle    string `json:"lifecycle"`
	GoWorkMember bool   `json:"go_work_member"`
}

type manifest struct {
	Repos []repoEntry `json:"repos"`
}

type options struct {
	root                 string
	manifestPath         string
	versionFile          string
	dryRun               bool
	tidy                 bool
	includeCompatibility bool
	includeDeprecated    bool
	repoFilter           map[string]struct{}
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "hg-sync:", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	home, _ := os.UserHomeDir()
	defaultRoot := filepath.Join(home, "hairglasses-studio")

	var repos string
	var opts options
	flag.StringVar(&opts.root, "root", defaultRoot, "workspace root")
	flag.StringVar(&opts.manifestPath, "manifest", "", "manifest path (default <root>/workspace/manifest.json)")
	flag.StringVar(&opts.versionFile, "version-file", "", "go version file (default <root>/make/go-version)")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "show what would change")
	flag.BoolVar(&opts.tidy, "tidy", false, "run go mod tidy after update")
	flag.BoolVar(&opts.includeCompatibility, "include-compatibility", false, "include compatibility lifecycle repos")
	flag.BoolVar(&opts.includeDeprecated, "include-deprecated", false, "include deprecated lifecycle repos")
	flag.StringVar(&repos, "repos", "", "comma-separated repo names")
	flag.Parse()

	opts.repoFilter = map[string]struct{}{}
	for _, p := range strings.Split(repos, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			opts.repoFilter[p] = struct{}{}
		}
	}
	if opts.manifestPath == "" {
		opts.manifestPath = filepath.Join(opts.root, "workspace", "manifest.json")
	}
	if opts.versionFile == "" {
		opts.versionFile = filepath.Join(opts.root, "make", "go-version")
	}
	return opts
}

func run(opts options) error {
	targetVersionRaw, err := os.ReadFile(opts.versionFile)
	if err != nil {
		return fmt.Errorf("read version file: %w", err)
	}
	targetVersion := strings.TrimSpace(string(targetVersionRaw))
	if targetVersion == "" {
		return errors.New("empty go version in version file")
	}

	mf, err := loadManifest(opts.manifestPath)
	if err != nil {
		return err
	}
	repos := selectRepos(mf.Repos, opts)
	changed := 0
	for _, repo := range repos {
		repoPath := repo.Path
		if !filepath.IsAbs(repoPath) {
			repoPath = filepath.Join(opts.root, repoPath)
		}
		goModPath := filepath.Join(repoPath, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			continue
		}
		before, after, err := rewriteGoVersion(goModPath, targetVersion)
		if err != nil {
			return fmt.Errorf("%s: %w", repo.Name, err)
		}
		if before == after {
			fmt.Printf("%-24s %s (current)\n", repo.Name, before)
			continue
		}
		if opts.dryRun {
			fmt.Printf("%-24s %s -> %s (dry-run)\n", repo.Name, before, after)
			changed++
			continue
		}
		fmt.Printf("%-24s %s -> %s\n", repo.Name, before, after)
		changed++
		if opts.tidy {
			cmd := exec.Command("go", "mod", "tidy")
			cmd.Dir = repoPath
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: go mod tidy: %w", repo.Name, err)
			}
		}
	}
	if opts.dryRun {
		fmt.Printf("repos to update: %d\n", changed)
	} else {
		fmt.Printf("repos updated: %d\n", changed)
	}
	return nil
}

func loadManifest(path string) (manifest, error) {
	var mf manifest
	raw, err := os.ReadFile(path)
	if err != nil {
		return mf, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(raw, &mf); err != nil {
		return mf, fmt.Errorf("parse manifest: %w", err)
	}
	return mf, nil
}

func selectRepos(repos []repoEntry, opts options) []repoEntry {
	out := make([]repoEntry, 0, len(repos))
	for _, r := range repos {
		if len(opts.repoFilter) > 0 {
			if _, ok := opts.repoFilter[r.Name]; !ok {
				continue
			}
		}
		if !opts.includeCompatibility && strings.EqualFold(r.Lifecycle, "compatibility") {
			continue
		}
		if !opts.includeDeprecated && strings.EqualFold(r.Lifecycle, "deprecated") {
			continue
		}
		if isGoRepo(r) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b repoEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func isGoRepo(r repoEntry) bool {
	if r.GoWorkMember {
		return true
	}
	return strings.Contains(strings.ToLower(r.Language), "go")
}

func rewriteGoVersion(path, target string) (string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read go.mod: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	before := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "go ") {
			parts := strings.Fields(trim)
			if len(parts) >= 2 {
				before = parts[1]
			}
			lines[i] = "go " + target
			break
		}
	}
	if before == "" {
		return "", "", errors.New("missing go directive")
	}
	if before == target {
		return before, target, nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return before, "", fmt.Errorf("write go.mod: %w", err)
	}
	return before, target, nil
}
