package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	homeOnce sync.Once
	homeDir  string
)

// HomeDir returns the current user's home directory. The result is cached
// after the first call. Returns an empty string if the home directory
// cannot be determined.
func HomeDir() string {
	homeOnce.Do(func() {
		homeDir, _ = os.UserHomeDir()
	})
	return homeDir
}

// ExpandHome replaces a leading "~" or "~/" in path with the user's home
// directory. If path does not start with "~", it is returned unchanged.
func ExpandHome(path string) string {
	if path == "~" {
		return HomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}

// ConfigDir returns the XDG-compliant config directory for the given
// application name: $XDG_CONFIG_HOME/<app> or ~/.config/<app>.
func ConfigDir(app string) string {
	return filepath.Join(ResolveEnv("XDG_CONFIG_HOME", filepath.Join(HomeDir(), ".config")), app)
}

// DataDir returns the XDG-compliant data directory for the given
// application name: $XDG_DATA_HOME/<app> or ~/.local/share/<app>.
func DataDir(app string) string {
	return filepath.Join(ResolveEnv("XDG_DATA_HOME", filepath.Join(HomeDir(), ".local", "share")), app)
}

// CacheDir returns the XDG-compliant cache directory for the given
// application name: $XDG_CACHE_HOME/<app> or ~/.cache/<app>.
func CacheDir(app string) string {
	return filepath.Join(ResolveEnv("XDG_CACHE_HOME", filepath.Join(HomeDir(), ".cache")), app)
}

// ResolveEnv returns the value of the environment variable named by key.
// If the variable is empty or unset, fallback is returned.
func ResolveEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IsSubPath reports whether target is contained within (or equal to) the
// base directory. Both paths are cleaned before comparison. Symbolic links
// are not resolved.
func IsSubPath(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."))
}
