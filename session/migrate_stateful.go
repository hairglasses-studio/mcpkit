package session

import "context"

// MigrateStatefulSessions is a convenience helper for servers that maintain
// session state in an arbitrary map (e.g. an in-process map[string]map[string]any).
// It creates a new session in store for each entry in legacy, copies all
// key/value pairs, and returns a mapping of old IDs → new session IDs.
//
// If legacy is nil or empty, an empty map is returned without error.
func MigrateStatefulSessions(
	ctx context.Context,
	store SessionStore,
	legacy map[string]map[string]any,
) (map[string]string, error) {
	result := make(map[string]string, len(legacy))
	for oldID, data := range legacy {
		sess, err := store.Create(ctx)
		if err != nil {
			return result, err
		}
		for k, v := range data {
			sess.Set(k, v)
		}
		result[oldID] = sess.ID()
	}
	return result, nil
}
