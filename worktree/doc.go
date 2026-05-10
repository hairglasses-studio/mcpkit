// Package worktree manages a pool of pre-created git worktrees per
// repository so concurrent sessions can acquire and release isolated
// checkouts without paying the cost of `git worktree add` /
// `git worktree remove` on every turn. Pool worktrees never merge into
// main/master and get recycled after a configurable staleness window.
//
// Typical usage:
//
//	pool := worktree.New(repoDir, worktree.PoolConfig{
//		Size:           worktree.DefaultPoolSize,
//		StaleThreshold: worktree.DefaultStaleThreshold,
//	})
//	wt, release, err := pool.Acquire(ctx)
//	if err != nil { ... }
//	defer release()
//	// work inside wt.Path()
package worktree
