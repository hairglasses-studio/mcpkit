package session

import (
	"context"
	"fmt"
)

// MigrationFunc is a function that migrates data from an old session into a
// new session. Implementations should copy relevant keys and return any error.
type MigrationFunc func(ctx context.Context, oldSess Session, newSess Session) error

// Migrator helps stateful servers transition existing sessions to the new
// session store. It reads sessions from a source store and re-creates them
// in a destination store, applying optional migration functions.
type Migrator struct {
	src   SessionStore
	dst   SessionStore
	funcs []MigrationFunc
}

// NewMigrator creates a Migrator that copies sessions from src to dst.
func NewMigrator(src, dst SessionStore, fns ...MigrationFunc) *Migrator {
	return &Migrator{src: src, dst: dst, funcs: fns}
}

// MigrateID migrates a single session identified by id from the source store
// to the destination store. Returns the new session and any error.
func (m *Migrator) MigrateID(ctx context.Context, id string) (Session, error) {
	old, ok, err := m.src.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("session migrate: get source session %q: %w", id, err)
	}
	if !ok {
		return nil, fmt.Errorf("session migrate: source session %q not found or expired", id)
	}

	newSess, err := m.dst.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("session migrate: create destination session: %w", err)
	}

	// Apply each migration function in order.
	for i, fn := range m.funcs {
		if err := fn(ctx, old, newSess); err != nil {
			// Best-effort cleanup of the partially-migrated session.
			_ = m.dst.Delete(ctx, newSess.ID())
			return nil, fmt.Errorf("session migrate: migration func [%d]: %w", i, err)
		}
	}

	// Delete old session from source store.
	if err := m.src.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("session migrate: delete source session %q: %w", id, err)
	}

	return newSess, nil
}

// CopyKeys is a MigrationFunc that copies all listed keys from old to new.
// Missing keys are silently skipped.
func CopyKeys(keys ...string) MigrationFunc {
	return func(ctx context.Context, old Session, newSess Session) error {
		for _, k := range keys {
			if v, ok := old.Get(k); ok {
				newSess.Set(k, v)
			}
		}
		return nil
	}
}
