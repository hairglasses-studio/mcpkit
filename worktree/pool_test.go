package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewPoolDefaults(t *testing.T) {
	t.Parallel()

	pool := NewPool(0)
	if pool.poolSize != DefaultPoolSize {
		t.Fatalf("pool size = %d, want %d", pool.poolSize, DefaultPoolSize)
	}
	if pool.repos == nil {
		t.Fatal("repos map was not initialized")
	}

	custom := NewPool(2)
	if custom.poolSize != 2 {
		t.Fatalf("custom pool size = %d, want 2", custom.poolSize)
	}
}

func TestAcquireReleaseReusesCleanWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(1)
	t.Cleanup(pool.Close)

	wtPath, branch, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if wtPath == "" || branch == "" {
		t.Fatalf("Acquire() returned path=%q branch=%q", wtPath, branch)
	}
	if !strings.HasPrefix(branch, "ralph/pool/") {
		t.Fatalf("branch = %q, want ralph/pool/*", branch)
	}

	if got := mustGit(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"); got != branch {
		t.Fatalf("worktree branch = %q, want %q", got, branch)
	}
	assertWorktreeWiring(t, repo, wtPath)

	if err := os.WriteFile(filepath.Join(wtPath, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("dirty tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "scratch.txt"), []byte("remove me\n"), 0644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	pool.Release(ctx, repo, wtPath)
	if !pathExists(wtPath) {
		t.Fatal("released worktree should remain available in the idle pool")
	}

	reusedPath, reusedBranch, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() reused error = %v", err)
	}
	if reusedPath != wtPath {
		t.Fatalf("Acquire() reused path = %q, want %q", reusedPath, wtPath)
	}
	if reusedBranch != branch {
		t.Fatalf("Acquire() reused branch = %q, want %q", reusedBranch, branch)
	}
	if pathExists(filepath.Join(reusedPath, "scratch.txt")) {
		t.Fatal("untracked file survived pooled reset")
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(reusedPath, "README.md"))); got != "seed" {
		t.Fatalf("tracked file content = %q, want seed", got)
	}

	pool.Release(ctx, repo, reusedPath)
	pool.Close()
	if pathExists(reusedPath) {
		t.Fatal("Close() should remove idle worktrees")
	}
	pool.Close()
	if _, _, err := pool.Acquire(ctx, repo); err == nil {
		t.Fatal("Acquire() on closed pool succeeded")
	}
}

func TestWarmCleanupStaleAndCapacity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(2)
	t.Cleanup(pool.Close)

	if err := pool.Warm(ctx, repo, 3); err != nil {
		t.Fatalf("Warm() error = %v", err)
	}
	rp, err := pool.getOrCreateRepoPool(ctx, repo)
	if err != nil {
		t.Fatalf("repo pool: %v", err)
	}

	rp.mu.Lock()
	if got := len(rp.idle); got != 2 {
		rp.mu.Unlock()
		t.Fatalf("idle count after warm = %d, want 2", got)
	}
	stalePath := rp.idle[0].Path
	keptPath := rp.idle[1].Path
	rp.idle[0].CreatedAt = time.Now().Add(-2 * time.Hour)
	rp.idle[1].CreatedAt = time.Now()
	rp.mu.Unlock()

	if cleaned := pool.CleanupStale(time.Hour); cleaned != 1 {
		t.Fatalf("CleanupStale() = %d, want 1", cleaned)
	}
	if pathExists(stalePath) {
		t.Fatalf("stale worktree still exists: %s", stalePath)
	}
	if !pathExists(keptPath) {
		t.Fatalf("fresh worktree was removed: %s", keptPath)
	}

	rp.mu.Lock()
	remaining := len(rp.idle)
	rp.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("idle count after cleanup = %d, want 1", remaining)
	}

	if err := pool.Warm(ctx, repo, 1); err != nil {
		t.Fatalf("Warm() with enough idle error = %v", err)
	}
	rp.mu.Lock()
	afterNoopWarm := len(rp.idle)
	rp.mu.Unlock()
	if afterNoopWarm != 1 {
		t.Fatalf("idle count after no-op warm = %d, want 1", afterNoopWarm)
	}

	pool.Close()
	if cleaned := pool.CleanupStale(time.Nanosecond); cleaned != 0 {
		t.Fatalf("CleanupStale() on closed pool = %d, want 0", cleaned)
	}
	if err := pool.Warm(ctx, repo, 1); err == nil {
		t.Fatal("Warm() on closed pool succeeded")
	}
}

func TestAcquireFreshWhenIdleWorktreeMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(1)
	t.Cleanup(pool.Close)

	if err := pool.Warm(ctx, repo, 1); err != nil {
		t.Fatalf("Warm() error = %v", err)
	}
	rp, err := pool.getOrCreateRepoPool(ctx, repo)
	if err != nil {
		t.Fatalf("repo pool: %v", err)
	}

	rp.mu.Lock()
	missingPath := rp.idle[0].Path
	missingBranch := rp.idle[0].Branch
	rp.mu.Unlock()
	if err := os.RemoveAll(missingPath); err != nil {
		t.Fatalf("remove idle worktree: %v", err)
	}

	freshPath, freshBranch, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() after missing idle error = %v", err)
	}
	if freshPath == missingPath {
		t.Fatalf("Acquire() reused missing path %q", missingPath)
	}
	if freshBranch == missingBranch {
		t.Fatalf("Acquire() reused branch for missing worktree %q", missingBranch)
	}
	if !pathExists(freshPath) {
		t.Fatalf("fresh worktree does not exist: %s", freshPath)
	}
	pool.Release(ctx, repo, freshPath)
}

func TestReleaseInvalidRepoDestroysWorktree(t *testing.T) {
	t.Parallel()

	wtPath := filepath.Join(t.TempDir(), "orphan")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}

	pool := NewPool(1)
	pool.Release(context.Background(), filepath.Join(t.TempDir(), "not-a-repo"), wtPath)
	if pathExists(wtPath) {
		t.Fatal("Release() should destroy worktree when repo cannot be resolved")
	}
}

func TestResolveRepoRootAndWorktreeGitDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	nested := filepath.Join(repo, "nested", "path")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	root, err := resolveRepoRoot(ctx, nested)
	if err != nil {
		t.Fatalf("resolveRepoRoot() error = %v", err)
	}
	if root != repo {
		t.Fatalf("resolveRepoRoot() = %q, want %q", root, repo)
	}

	wtPath, _, err := NewPool(1).Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	gitDir, err := resolveWorktreeGitDir(wtPath)
	if err != nil {
		t.Fatalf("resolveWorktreeGitDir() error = %v", err)
	}
	if !strings.Contains(gitDir, ".git/worktrees/") {
		t.Fatalf("git dir = %q, want linked worktree git dir", gitDir)
	}

	inline := filepath.Join(t.TempDir(), "inline")
	if err := os.MkdirAll(filepath.Join(inline, ".git"), 0755); err != nil {
		t.Fatalf("mkdir inline git dir: %v", err)
	}
	if got, err := resolveWorktreeGitDir(inline); err != nil || got != filepath.Join(inline, ".git") {
		t.Fatalf("resolveWorktreeGitDir(inline) = %q, %v", got, err)
	}
}

func TestPreWarmCreatesWorktrees(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	created, err := pool.PreWarm(ctx, repo, 3)
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}
	if created != 3 {
		t.Fatalf("PreWarm() created = %d, want 3", created)
	}

	// Verify worktrees exist on disk.
	rp, err := pool.getOrCreateRepoPool(ctx, repo)
	if err != nil {
		t.Fatalf("repo pool: %v", err)
	}
	rp.mu.Lock()
	idle := len(rp.idle)
	var paths []string
	for _, e := range rp.idle {
		paths = append(paths, e.Path)
	}
	rp.mu.Unlock()

	if idle != 3 {
		t.Fatalf("idle count = %d, want 3", idle)
	}
	for _, p := range paths {
		if !pathExists(p) {
			t.Fatalf("pre-warmed worktree does not exist: %s", p)
		}
	}
}

func TestPreWarmRespectsPoolSize(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(2)
	t.Cleanup(pool.Close)

	created, err := pool.PreWarm(ctx, repo, 5)
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}
	if created != 2 {
		t.Fatalf("PreWarm() created = %d, want 2 (pool max)", created)
	}

	// Second PreWarm should be a no-op.
	created2, err := pool.PreWarm(ctx, repo, 5)
	if err != nil {
		t.Fatalf("PreWarm() second call error = %v", err)
	}
	if created2 != 0 {
		t.Fatalf("PreWarm() second call created = %d, want 0", created2)
	}
}

func TestPreWarmOnClosedPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(2)
	pool.Close()

	if _, err := pool.PreWarm(ctx, repo, 1); err == nil {
		t.Fatal("PreWarm() on closed pool succeeded")
	}
}

func TestAcquireReturnsPreWarmedWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	created, err := pool.PreWarm(ctx, repo, 2)
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}
	if created != 2 {
		t.Fatalf("PreWarm() created = %d, want 2", created)
	}

	// Acquire should return a pre-warmed worktree (LIFO).
	rp, _ := pool.getOrCreateRepoPool(ctx, repo)
	rp.mu.Lock()
	expectedPath := rp.idle[len(rp.idle)-1].Path
	rp.mu.Unlock()

	wtPath, _, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if wtPath != expectedPath {
		t.Fatalf("Acquire() path = %q, want pre-warmed %q", wtPath, expectedPath)
	}

	// Pool should have 1 idle left.
	if got := pool.Size(ctx, repo); got != 1 {
		t.Fatalf("Size() = %d after one acquire, want 1", got)
	}
}

func TestAcquireOnDemandWhenPoolEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	// Don't pre-warm — acquire should create on-demand.
	if got := pool.Size(ctx, repo); got != 0 {
		t.Fatalf("Size() = %d before any warm, want 0", got)
	}

	wtPath, branch, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if wtPath == "" || branch == "" {
		t.Fatalf("Acquire() returned empty path=%q branch=%q", wtPath, branch)
	}
	if !pathExists(wtPath) {
		t.Fatalf("on-demand worktree does not exist: %s", wtPath)
	}
}

func TestReleaseReturnsToPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	wtPath, _, err := pool.Acquire(ctx, repo)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if got := pool.Size(ctx, repo); got != 0 {
		t.Fatalf("Size() = %d after acquire, want 0", got)
	}

	pool.Release(ctx, repo, wtPath)

	if got := pool.Size(ctx, repo); got != 1 {
		t.Fatalf("Size() = %d after release, want 1", got)
	}
}

func TestConcurrentAcquireRelease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(8)
	t.Cleanup(pool.Close)

	// Pre-warm 4 worktrees.
	created, err := pool.PreWarm(ctx, repo, 4)
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}
	if created != 4 {
		t.Fatalf("PreWarm() created = %d, want 4", created)
	}

	// Launch 8 goroutines that acquire and release.
	const goroutines = 8
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			wt, _, err := pool.Acquire(ctx, repo)
			if err != nil {
				errCh <- fmt.Errorf("Acquire: %w", err)
				return
			}
			// Simulate brief work.
			time.Sleep(5 * time.Millisecond)
			pool.Release(ctx, repo, wt)
			errCh <- nil
		}()
	}

	for i := 0; i < goroutines; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent goroutine failed: %v", err)
		}
	}

	// All should have been returned; pool should have idle worktrees.
	idle := pool.Size(ctx, repo)
	if idle == 0 {
		t.Fatal("Size() = 0 after all releases, expected some idle worktrees")
	}
}

func TestDrainRemovesAvailableWorktrees(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	created, err := pool.PreWarm(ctx, repo, 3)
	if err != nil {
		t.Fatalf("PreWarm() error = %v", err)
	}
	if created != 3 {
		t.Fatalf("PreWarm() created = %d, want 3", created)
	}

	// Collect paths for existence check after drain.
	rp, _ := pool.getOrCreateRepoPool(ctx, repo)
	rp.mu.Lock()
	var paths []string
	for _, e := range rp.idle {
		paths = append(paths, e.Path)
	}
	rp.mu.Unlock()

	pool.Drain(ctx, repo)

	if got := pool.Size(ctx, repo); got != 0 {
		t.Fatalf("Size() after Drain = %d, want 0", got)
	}

	// Verify worktrees are removed from disk.
	for _, p := range paths {
		if pathExists(p) {
			t.Fatalf("drained worktree still exists: %s", p)
		}
	}

	// Pool is still open — can PreWarm again.
	created2, err := pool.PreWarm(ctx, repo, 1)
	if err != nil {
		t.Fatalf("PreWarm() after Drain error = %v", err)
	}
	if created2 != 1 {
		t.Fatalf("PreWarm() after Drain created = %d, want 1", created2)
	}
}

func TestSizeReturnsCorrectCounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	// Before any warming.
	if got := pool.Size(ctx, repo); got != 0 {
		t.Fatalf("Size() empty pool = %d, want 0", got)
	}

	// After warming.
	pool.PreWarm(ctx, repo, 2)
	if got := pool.Size(ctx, repo); got != 2 {
		t.Fatalf("Size() after warm = %d, want 2", got)
	}

	// After acquire.
	pool.Acquire(ctx, repo)
	if got := pool.Size(ctx, repo); got != 1 {
		t.Fatalf("Size() after acquire = %d, want 1", got)
	}

	// Closed pool.
	pool.Close()
	if got := pool.Size(ctx, repo); got != 0 {
		t.Fatalf("Size() closed pool = %d, want 0", got)
	}
}

func TestAllStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := initGitRepo(t)
	pool := NewPool(4)
	t.Cleanup(pool.Close)

	pool.PreWarm(ctx, repo, 2)

	stats := pool.AllStats()
	if len(stats) != 1 {
		t.Fatalf("AllStats() len = %d, want 1", len(stats))
	}
	if stats[0].IdleCount != 2 {
		t.Fatalf("AllStats()[0].IdleCount = %d, want 2", stats[0].IdleCount)
	}
	if stats[0].PoolSize != 4 {
		t.Fatalf("AllStats()[0].PoolSize = %d, want 4", stats[0].PoolSize)
	}
}

func TestHelperErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := NewPool(1)
	if _, _, err := pool.Acquire(ctx, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Acquire() on missing repo succeeded")
	}

	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := resolveWorktreeGitDir(missing); err == nil {
		t.Fatal("resolveWorktreeGitDir() on missing path succeeded")
	}

	if branch := pool.currentBranch(missing); branch != "" {
		t.Fatalf("currentBranch() on missing path = %q, want empty", branch)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("seed\n"), 0644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "seed")

	return repo
}

func assertWorktreeWiring(t *testing.T, repo, wtPath string) {
	t.Helper()

	wtGitDir, err := resolveWorktreeGitDir(wtPath)
	if err != nil {
		t.Fatalf("resolveWorktreeGitDir() error = %v", err)
	}
	alternates := strings.TrimSpace(readFile(t, filepath.Join(wtGitDir, "objects", "info", "alternates")))
	wantObjects, err := filepath.Abs(filepath.Join(repo, ".git", "objects"))
	if err != nil {
		t.Fatalf("abs objects path: %v", err)
	}
	if !strings.Contains(alternates, wantObjects) {
		t.Fatalf("alternates = %q, want %q", alternates, wantObjects)
	}

	hook := filepath.Join(wtGitDir, "hooks", "pre-merge-commit")
	if info, err := os.Stat(hook); err != nil {
		t.Fatalf("pre-merge hook missing: %v", err)
	} else if info.Mode().Perm() != 0755 {
		t.Fatalf("pre-merge hook mode = %v, want 0755", info.Mode().Perm())
	}
	if body := readFile(t, hook); !strings.Contains(body, "protected branch") {
		t.Fatalf("pre-merge hook body missing guard: %q", body)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %s failed: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s failed: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
