package worktree

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultPoolSize is the default number of pre-created worktrees per repo.
const DefaultPoolSize = 4

// DefaultStaleThreshold is the default age after which idle pool worktrees are
// considered stale and eligible for automatic cleanup.
const DefaultStaleThreshold = 24 * time.Hour

// protectedBranches lists branch names that pool worktrees must never merge into.
var protectedBranches = []string{"main", "master"}

// WorktreePool manages a pool of pre-created git worktrees per repository.
// Worktrees are acquired for session use and released back for reuse, avoiding
// the overhead of repeated `git worktree add` / `git worktree remove` cycles.
type WorktreePool struct {
	mu       sync.Mutex           // protects repos map
	repos    map[string]*repoPool // keyed by canonical repo path
	poolSize int                  // max idle worktrees per repo
	counter  atomic.Int64         // monotonic counter for unique naming
	closed   atomic.Bool
}

// repoPool holds the pool state for a single repository.
type repoPool struct {
	mu       sync.Mutex
	idle     []poolEntry // available worktrees ready for checkout
	repoRoot string      // canonical git toplevel path
}

// poolEntry represents a single pooled worktree.
type poolEntry struct {
	Path      string    // filesystem path to the worktree
	Branch    string    // branch name checked out in the worktree
	CreatedAt time.Time // when this entry was created or last released
}

// Stats exposes metrics about pool utilization.
type Stats struct {
	RepoPath  string `json:"repo_path"`
	IdleCount int    `json:"idle_count"`
	PoolSize  int    `json:"pool_size"`
}

// NewPool creates a pool with the given max idle size per repo.
// If size <= 0, DefaultPoolSize is used.
func NewPool(size int) *WorktreePool {
	if size <= 0 {
		size = DefaultPoolSize
	}
	return &WorktreePool{
		repos:    make(map[string]*repoPool),
		poolSize: size,
	}
}

// Acquire returns a clean worktree path for the given repo.
func (wp *WorktreePool) Acquire(ctx context.Context, repoPath string) (string, string, error) {
	if wp.closed.Load() {
		return "", "", fmt.Errorf("worktree pool is closed")
	}

	rp, err := wp.getOrCreateRepoPool(ctx, repoPath)
	if err != nil {
		return "", "", err
	}

	rp.mu.Lock()
	if len(rp.idle) > 0 {
		entry := rp.idle[len(rp.idle)-1]
		rp.idle = rp.idle[:len(rp.idle)-1]
		rp.mu.Unlock()

		if err := wp.resetWorktree(ctx, rp.repoRoot, entry.Path); err != nil {
			slog.Warn("failed to reset pooled worktree, creating fresh one",
				"path", entry.Path, "error", err)
			wp.destroyWorktree(rp.repoRoot, entry.Path)
			return wp.createFreshWorktree(ctx, rp.repoRoot)
		}

		slog.Debug("acquired pooled worktree", "path", entry.Path, "repo", rp.repoRoot)
		return entry.Path, entry.Branch, nil
	}
	rp.mu.Unlock()

	return wp.createFreshWorktree(ctx, rp.repoRoot)
}

// Release returns a worktree to the pool for reuse.
func (wp *WorktreePool) Release(ctx context.Context, repoPath, wtPath string) {
	if wp.closed.Load() {
		return
	}
	if wtPath == "" {
		return
	}

	rp, err := wp.getOrCreateRepoPool(ctx, repoPath)
	if err != nil {
		slog.Warn("release: cannot resolve repo pool, destroying worktree",
			"repo", repoPath, "path", wtPath, "error", err)
		wp.destroyWorktree(repoPath, wtPath)
		return
	}

	if err := wp.cleanWorktree(ctx, rp.repoRoot, wtPath); err != nil {
		slog.Warn("release: failed to clean worktree, destroying instead",
			"path", wtPath, "error", err)
		wp.destroyWorktree(rp.repoRoot, wtPath)
		return
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	if len(rp.idle) >= wp.poolSize {
		wp.destroyWorktree(rp.repoRoot, wtPath)
		return
	}

	branch := wp.currentBranch(wtPath)
	rp.idle = append(rp.idle, poolEntry{Path: wtPath, Branch: branch, CreatedAt: time.Now()})
	slog.Debug("released worktree to pool", "path", wtPath, "idle", len(rp.idle), "repo", rp.repoRoot)
}

// Warm pre-creates worktrees for the given repo path.
func (wp *WorktreePool) Warm(ctx context.Context, repoPath string, count int) error {
	if wp.closed.Load() {
		return fmt.Errorf("worktree pool is closed")
	}

	rp, err := wp.getOrCreateRepoPool(ctx, repoPath)
	if err != nil {
		return err
	}

	rp.mu.Lock()
	existing := len(rp.idle)
	rp.mu.Unlock()

	need := count - existing
	if need <= 0 {
		return nil
	}
	if existing+need > wp.poolSize {
		need = wp.poolSize - existing
	}
	if need <= 0 {
		return nil
	}

	var created int
	for i := 0; i < need; i++ {
		if ctx.Err() != nil {
			break
		}
		path, branch, err := wp.createFreshWorktree(ctx, rp.repoRoot)
		if err != nil {
			slog.Warn("warm: failed to create worktree", "repo", rp.repoRoot, "error", err)
			continue
		}

		rp.mu.Lock()
		if len(rp.idle) < wp.poolSize {
			rp.idle = append(rp.idle, poolEntry{Path: path, Branch: branch, CreatedAt: time.Now()})
			created++
		} else {
			wp.destroyWorktree(rp.repoRoot, path)
		}
		rp.mu.Unlock()
	}

	slog.Info("warmed worktree pool", "repo", rp.repoRoot, "created", created, "total_idle", existing+created)
	return nil
}

// CleanupStale removes idle pool worktrees older than the threshold.
func (wp *WorktreePool) CleanupStale(olderThan time.Duration) int {
	if wp.closed.Load() {
		return 0
	}

	wp.mu.Lock()
	reposCopy := make(map[string]*repoPool, len(wp.repos))
	maps.Copy(reposCopy, wp.repos)
	wp.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var totalCleaned int

	for _, rp := range reposCopy {
		rp.mu.Lock()
		var kept []poolEntry
		var stale []poolEntry
		for _, e := range rp.idle {
			if e.CreatedAt.Before(cutoff) {
				stale = append(stale, e)
			} else {
				kept = append(kept, e)
			}
		}
		rp.idle = kept
		rp.mu.Unlock()

		for _, e := range stale {
			wp.destroyWorktree(rp.repoRoot, e.Path)
			totalCleaned++
		}
	}

	return totalCleaned
}

// Close destroys all pooled worktrees and marks the pool as closed.
func (wp *WorktreePool) Close() {
	if !wp.closed.CompareAndSwap(false, true) {
		return
	}

	wp.mu.Lock()
	repos := wp.repos
	wp.repos = make(map[string]*repoPool)
	wp.mu.Unlock()

	for _, rp := range repos {
		rp.mu.Lock()
		entries := rp.idle
		rp.idle = nil
		rp.mu.Unlock()

		for _, e := range entries {
			wp.destroyWorktree(rp.repoRoot, e.Path)
		}
	}
}

func (wp *WorktreePool) getOrCreateRepoPool(ctx context.Context, repoPath string) (*repoPool, error) {
	root, err := resolveRepoRoot(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	wp.mu.Lock()
	defer wp.mu.Unlock()

	if rp, ok := wp.repos[root]; ok {
		return rp, nil
	}

	rp := &repoPool{
		repoRoot: root,
		idle:     make([]poolEntry, 0, wp.poolSize),
	}
	wp.repos[root] = rp
	return rp, nil
}

func resolveRepoRoot(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (wp *WorktreePool) createFreshWorktree(ctx context.Context, repoRoot string) (string, string, error) {
	seq := wp.counter.Add(1)
	name := fmt.Sprintf("pool-%d-%d", time.Now().UnixMilli(), seq)
	wtPath := filepath.Join(repoRoot, ".ralph", "worktrees", "pool", name)

	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", "", fmt.Errorf("create pool dir: %w", err)
	}

	branch := fmt.Sprintf("ralph/pool/%s", name)
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "add", "-B", branch, wtPath, "HEAD")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(output)))
	}

	_ = setupAlternates(repoRoot, wtPath)
	_ = installMergePreventionHook(wtPath)

	return wtPath, branch, nil
}

func setupAlternates(repoRoot, wtPath string) error {
	parentObjects := filepath.Join(repoRoot, ".git", "objects")
	wtGitDir, err := resolveWorktreeGitDir(wtPath)
	if err != nil {
		return err
	}

	altFile := filepath.Join(wtGitDir, "objects", "info", "alternates")
	absParentObjects, _ := filepath.Abs(parentObjects)

	if err := os.MkdirAll(filepath.Dir(altFile), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(altFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, absParentObjects)
	return err
}

func resolveWorktreeGitDir(wtPath string) (string, error) {
	gitPath := filepath.Join(wtPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return gitPath, nil
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(wtPath, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

const mergePreventionHookScript = `#!/bin/sh
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
case "$BRANCH" in
  main|master)
    echo "ERROR: merge into protected branch '$BRANCH' is blocked in pool worktrees." >&2
    exit 1
    ;;
esac
exit 0
`

func installMergePreventionHook(wtPath string) error {
	wtGitDir, err := resolveWorktreeGitDir(wtPath)
	if err != nil {
		return err
	}
	hookPath := filepath.Join(wtGitDir, "hooks", "pre-merge-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(hookPath, []byte(mergePreventionHookScript), 0755)
}

func (wp *WorktreePool) resetWorktree(ctx context.Context, repoRoot, wtPath string) error {
	if _, err := os.Stat(wtPath); err != nil {
		return err
	}
	exec.CommandContext(ctx, "git", "-C", wtPath, "reset", "--hard", "HEAD").Run()
	exec.CommandContext(ctx, "git", "-C", wtPath, "clean", "-fd").Run()
	return nil
}

func (wp *WorktreePool) cleanWorktree(ctx context.Context, repoRoot, wtPath string) error {
	if _, err := os.Stat(wtPath); err != nil {
		return err
	}
	exec.CommandContext(ctx, "git", "-C", wtPath, "reset", "--hard", "HEAD").Run()
	exec.CommandContext(ctx, "git", "-C", wtPath, "clean", "-fd").Run()
	return nil
}

func (wp *WorktreePool) destroyWorktree(repoRoot, wtPath string) {
	if wtPath == "" {
		return
	}
	_ = os.RemoveAll(wtPath)
	_ = exec.Command("git", "-C", repoRoot, "worktree", "prune").Run()
}

func (wp *WorktreePool) currentBranch(wtPath string) string {
	out, err := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
