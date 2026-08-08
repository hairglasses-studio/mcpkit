// Package fleetinventory is the fleet-wide inventory platform: a single
// pruned filesystem walk per repo feeds provider-parity metrics, code-surface
// extraction (via surfaceinventory), and workspace drift detection, exposed
// as Go APIs and MCP tools.
//
// It replaces the failure modes of the bash agent-parity audit (dozens of
// full-tree finds per repo, hours of wall clock, silent stalls) with one
// bounded, parallel, deterministic scan: repos are walked once, walks run
// concurrently with a per-repo timeout, and every error is recorded in the
// report instead of aborting the sweep.
package fleetinventory

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// prunedDirs are directory basenames never descended into (mirrors the bash
// audit's prune list plus the surfaceinventory skips).
var prunedDirs = map[string]bool{
	".git": true, "node_modules": true, "worktrees": true, "vendor": true,
	".venv": true, "venv": true, "venv_test": true, "__pycache__": true,
	".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true,
	"htmlcov": true, "_salvage": true, "build": true, "dist": true,
	"testdata": true, "site-packages": true,
}

// FileIndex is the product of one repo walk: every regular file's
// repo-relative path (forward-slash), sorted.
type FileIndex struct {
	Repo  string
	Dir   string
	Paths []string
	// WalkErrors records unreadable directories; never fatal.
	WalkErrors []string
	// Truncated is set when MaxFiles stopped the walk early.
	Truncated bool
}

// WalkOptions bound a repo walk.
type WalkOptions struct {
	// MaxFiles stops the walk after this many files (0 = ScanDefaults.MaxFiles).
	MaxFiles int
	// Timeout aborts the walk after this duration (0 = ScanDefaults.RepoTimeout).
	Timeout time.Duration
}

// ScanDefaults bound scans when the caller passes zero values.
var ScanDefaults = struct {
	MaxFiles    int
	RepoTimeout time.Duration
	Parallelism int
}{
	MaxFiles:    500_000,
	RepoTimeout: 2 * time.Minute,
	Parallelism: 8,
}

// WalkRepo builds the FileIndex for one repo directory.
func WalkRepo(ctx context.Context, dir, name string, opts WalkOptions) FileIndex {
	maxFiles := opts.MaxFiles
	if maxFiles <= 0 {
		maxFiles = ScanDefaults.MaxFiles
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = ScanDefaults.RepoTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	idx := FileIndex{Repo: name, Dir: dir}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			idx.Truncated = true
			return filepath.SkipAll
		}
		if err != nil {
			idx.WalkErrors = append(idx.WalkErrors, err.Error())
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != dir && (prunedDirs[base] || isHiddenPrunable(base)) {
				return filepath.SkipDir
			}
			// Nested git checkouts (submodule gitlinks or embedded repos)
			// are inventoried under their own fleet entry.
			if path != dir {
				if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		idx.Paths = append(idx.Paths, filepath.ToSlash(rel))
		if len(idx.Paths) >= maxFiles {
			idx.Truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		idx.WalkErrors = append(idx.WalkErrors, err.Error())
	}
	sort.Strings(idx.Paths)
	return idx
}

// isHiddenPrunable prunes hidden dirs EXCEPT the provider-config dirs the
// parity metrics need to see (.agents, .claude, .codex, .gemini, .github,
// .codex-plugin, .well-known). Notably PRUNED: .ralph and .ouro — loop state
// dirs that leak rehomed Go module caches and other machine state into the
// index (jobb's .ralph/go-mod-cache-* inflated its apparent MCP surface 24x).
// HasRalph is detected by a direct stat in CollectParity instead.
func isHiddenPrunable(base string) bool {
	if !strings.HasPrefix(base, ".") {
		return false
	}
	switch base {
	case ".agents", ".claude", ".codex", ".codex-plugin", ".gemini", ".github", ".well-known":
		return false
	}
	return true
}

// Match returns the paths whose slash-relative form matches pred.
func (idx *FileIndex) Match(pred func(path string) bool) []string {
	var out []string
	for _, p := range idx.Paths {
		if pred(p) {
			out = append(out, p)
		}
	}
	return out
}

// CountBase counts files whose basename equals base.
func (idx *FileIndex) CountBase(base string) int {
	n := 0
	for _, p := range idx.Paths {
		if path := p; strings.HasSuffix(path, "/"+base) || path == base {
			n++
		}
	}
	return n
}

// CountSuffix counts files whose path ends with suffix (slash-aware).
func (idx *FileIndex) CountSuffix(suffix string) int {
	n := 0
	for _, p := range idx.Paths {
		if strings.HasSuffix(p, suffix) {
			n++
		}
	}
	return n
}

// Has reports whether the exact repo-relative path exists in the index.
func (idx *FileIndex) Has(path string) bool {
	i := sort.SearchStrings(idx.Paths, path)
	return i < len(idx.Paths) && idx.Paths[i] == path
}

// walkAll walks every repo concurrently with the platform's bounded pool.
func walkAll(ctx context.Context, root string, repos []string, opts WalkOptions) []FileIndex {
	sem := make(chan struct{}, ScanDefaults.Parallelism)
	out := make([]FileIndex, len(repos))
	var wg sync.WaitGroup
	for i, name := range repos {
		dir := filepath.Join(root, name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			out[i] = FileIndex{Repo: name, Dir: dir, WalkErrors: []string{"missing repo directory"}}
			continue
		}
		wg.Add(1)
		go func(i int, dir, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = WalkRepo(ctx, dir, name, opts)
		}(i, dir, name)
	}
	wg.Wait()
	return out
}
