package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeDir(t *testing.T) {
	h := HomeDir()
	if h == "" {
		t.Fatal("HomeDir returned empty string")
	}
	// Second call should return the same cached value.
	if HomeDir() != h {
		t.Fatal("HomeDir returned different value on second call")
	}
}

func TestExpandHome_Tilde(t *testing.T) {
	got := ExpandHome("~/foo")
	want := filepath.Join(HomeDir(), "foo")
	if got != want {
		t.Fatalf("ExpandHome(\"~/foo\") = %q, want %q", got, want)
	}
}

func TestExpandHome_TildeOnly(t *testing.T) {
	got := ExpandHome("~")
	if got != HomeDir() {
		t.Fatalf("ExpandHome(\"~\") = %q, want %q", got, HomeDir())
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	for _, p := range []string{"/abs/path", "rel/path", ""} {
		if got := ExpandHome(p); got != p {
			t.Errorf("ExpandHome(%q) = %q, want unchanged", p, got)
		}
	}
}

func TestConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got := ConfigDir("myapp")
	if got != "/custom/config/myapp" {
		t.Fatalf("ConfigDir = %q, want /custom/config/myapp", got)
	}
}

func TestConfigDir_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got := ConfigDir("myapp")
	want := filepath.Join(HomeDir(), ".config", "myapp")
	if got != want {
		t.Fatalf("ConfigDir = %q, want %q", got, want)
	}
}

func TestDataDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	got := DataDir("myapp")
	if got != "/custom/data/myapp" {
		t.Fatalf("DataDir = %q, want /custom/data/myapp", got)
	}
}

func TestDataDir_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	got := DataDir("myapp")
	want := filepath.Join(HomeDir(), ".local", "share", "myapp")
	if got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}
}

func TestCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	got := CacheDir("myapp")
	if got != "/custom/cache/myapp" {
		t.Fatalf("CacheDir = %q, want /custom/cache/myapp", got)
	}
}

func TestCacheDir_Default(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	got := CacheDir("myapp")
	want := filepath.Join(HomeDir(), ".cache", "myapp")
	if got != want {
		t.Fatalf("CacheDir = %q, want %q", got, want)
	}
}

func TestResolveEnv_Set(t *testing.T) {
	t.Setenv("TEST_PATHUTIL_VAR", "custom_value")
	got := ResolveEnv("TEST_PATHUTIL_VAR", "fallback")
	if got != "custom_value" {
		t.Fatalf("ResolveEnv = %q, want custom_value", got)
	}
}

func TestResolveEnv_Unset(t *testing.T) {
	os.Unsetenv("TEST_PATHUTIL_UNSET")
	got := ResolveEnv("TEST_PATHUTIL_UNSET", "fallback")
	if got != "fallback" {
		t.Fatalf("ResolveEnv = %q, want fallback", got)
	}
}

func TestIsSubPath_Within(t *testing.T) {
	if !IsSubPath("/home/user", "/home/user/docs") {
		t.Fatal("expected /home/user/docs to be subpath of /home/user")
	}
}

func TestIsSubPath_Equal(t *testing.T) {
	if !IsSubPath("/home/user", "/home/user") {
		t.Fatal("expected /home/user to be subpath of itself")
	}
}

func TestIsSubPath_Outside(t *testing.T) {
	if IsSubPath("/home/user", "/etc/passwd") {
		t.Fatal("expected /etc/passwd NOT to be subpath of /home/user")
	}
}

func TestIsSubPath_DotDot(t *testing.T) {
	if IsSubPath("/home/user", "/home/user/../other") {
		t.Fatal("expected traversal NOT to be subpath")
	}
}

func TestIsSubPath_Relative(t *testing.T) {
	if !IsSubPath("base", "base/sub/deep") {
		t.Fatal("expected base/sub/deep to be subpath of base")
	}
	if IsSubPath("base", "other/dir") {
		t.Fatal("expected other/dir NOT to be subpath of base")
	}
}

func TestExpandHome_TildeInMiddle(t *testing.T) {
	// Tilde not at start should not be expanded.
	input := "/path/~/file"
	if got := ExpandHome(input); got != input {
		t.Fatalf("ExpandHome(%q) = %q, want unchanged", input, got)
	}
}

func TestConfigDir_ContainsApp(t *testing.T) {
	got := ConfigDir("testapp")
	if !strings.HasSuffix(got, "/testapp") {
		t.Fatalf("ConfigDir should end with /testapp, got %q", got)
	}
}
