package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hairglasses-studio/mcpkit/secrets"
)

func writeTempEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}
	return path
}

func TestFileProvider_Get(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "API_KEY=supersecret\n")

	p := NewFileProvider(WithFiles(path))
	s, err := p.Get(context.Background(), "API_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "supersecret" {
		t.Errorf("expected supersecret, got %q", s.Value)
	}
}

func TestFileProvider_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "EXISTING=yes\n")

	p := NewFileProvider(WithFiles(path))
	_, err := p.Get(context.Background(), "MISSING_KEY")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != secrets.ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestFileProvider_Get_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	content := `DOUBLE_QUOTED="double quoted value"
SINGLE_QUOTED='single quoted value'
`
	path := writeTempEnvFile(t, dir, ".env", content)
	p := NewFileProvider(WithFiles(path))

	s1, err := p.Get(context.Background(), "DOUBLE_QUOTED")
	if err != nil {
		t.Fatalf("unexpected error for DOUBLE_QUOTED: %v", err)
	}
	if s1.Value != "double quoted value" {
		t.Errorf("expected 'double quoted value', got %q", s1.Value)
	}

	s2, err := p.Get(context.Background(), "SINGLE_QUOTED")
	if err != nil {
		t.Fatalf("unexpected error for SINGLE_QUOTED: %v", err)
	}
	if s2.Value != "single quoted value" {
		t.Errorf("expected 'single quoted value', got %q", s2.Value)
	}
}

func TestFileProvider_Get_EscapeSequences(t *testing.T) {
	dir := t.TempDir()
	content := "MULTILINE=line1\\nline2\nTABBED=col1\\tcol2\n"
	path := writeTempEnvFile(t, dir, ".env", content)
	p := NewFileProvider(WithFiles(path))

	sn, err := p.Get(context.Background(), "MULTILINE")
	if err != nil {
		t.Fatalf("unexpected error for MULTILINE: %v", err)
	}
	if sn.Value != "line1\nline2" {
		t.Errorf("expected newline escape expanded, got %q", sn.Value)
	}

	st, err := p.Get(context.Background(), "TABBED")
	if err != nil {
		t.Fatalf("unexpected error for TABBED: %v", err)
	}
	if st.Value != "col1\tcol2" {
		t.Errorf("expected tab escape expanded, got %q", st.Value)
	}
}

func TestFileProvider_List(t *testing.T) {
	dir := t.TempDir()
	content := "KEY_A=1\nKEY_B=2\nKEY_C=3\n"
	path := writeTempEnvFile(t, dir, ".env", content)
	p := NewFileProvider(WithFiles(path))

	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}
	for _, want := range []string{"KEY_A", "KEY_B", "KEY_C"} {
		if !found[want] {
			t.Errorf("expected key %q in List, got %v", want, keys)
		}
	}
}

func TestFileProvider_Exists(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "PRESENT=yes\n")
	p := NewFileProvider(WithFiles(path))

	exists, err := p.Exists(context.Background(), "PRESENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected Exists=true for key in file")
	}

	notExists, err := p.Exists(context.Background(), "ABSENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notExists {
		t.Error("expected Exists=false for key not in file")
	}
}

func TestFileProvider_Comments(t *testing.T) {
	dir := t.TempDir()
	content := "# this is a comment\nREAL_KEY=value\n# another comment\n"
	path := writeTempEnvFile(t, dir, ".env", content)
	p := NewFileProvider(WithFiles(path))

	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range keys {
		if k == "" || k[0] == '#' {
			t.Errorf("comment line leaked into keys: %q", k)
		}
	}

	s, err := p.Get(context.Background(), "REAL_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "value" {
		t.Errorf("expected value, got %q", s.Value)
	}
}

func TestFileProvider_Reload(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "TOKEN=original\n")
	p := NewFileProvider(WithFiles(path))

	s1, err := p.Get(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("initial Get failed: %v", err)
	}
	if s1.Value != "original" {
		t.Errorf("expected original, got %q", s1.Value)
	}

	// Overwrite the file
	if err := os.WriteFile(path, []byte("TOKEN=updated\n"), 0600); err != nil {
		t.Fatalf("failed to update file: %v", err)
	}

	if err := p.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	s2, err := p.Get(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("Get after Reload failed: %v", err)
	}
	if s2.Value != "updated" {
		t.Errorf("expected updated after Reload, got %q", s2.Value)
	}
}

func TestFileProvider_Health_Available(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "KEY=val\n")
	p := NewFileProvider(WithFiles(path))

	h := p.Health(context.Background())
	if !h.Available {
		t.Error("expected available=true when file exists")
	}
	if h.Name != "file" {
		t.Errorf("expected name=file, got %q", h.Name)
	}
}

func TestFileProvider_Health_Unavailable(t *testing.T) {
	// Provide only nonexistent files
	p := NewFileProvider(WithFiles("/tmp/definitely_not_a_real_file_mcpkit_xyz.env"))

	h := p.Health(context.Background())
	if h.Available {
		t.Error("expected available=false when no files exist")
	}
}

func TestFileProvider_Priority(t *testing.T) {
	p := NewFileProvider()
	if p.Priority() != 200 {
		t.Errorf("expected default priority 200, got %d", p.Priority())
	}

	p2 := NewFileProvider(WithFilePriority(50))
	if p2.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", p2.Priority())
	}
}

func TestFileProvider_Close(t *testing.T) {
	p := NewFileProvider()
	if err := p.Close(); err != nil {
		t.Errorf("expected nil error from Close, got %v", err)
	}
}

func TestFileProvider_IsAvailable_True(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "KEY=val\n")
	p := NewFileProvider(WithFiles(path))
	if !p.IsAvailable() {
		t.Error("expected IsAvailable=true when file exists")
	}
}

func TestFileProvider_IsAvailable_False(t *testing.T) {
	p := NewFileProvider(WithFiles("/tmp/mcpkit_definitely_nonexistent_xyzabc.env"))
	if p.IsAvailable() {
		t.Error("expected IsAvailable=false when no files exist")
	}
}

func TestFileProvider_loadFiles_SkipsNonExistentWithoutError(t *testing.T) {
	// Non-existent files should not produce an error (IsNotExist is swallowed).
	p := NewFileProvider(WithFiles("/tmp/mcpkit_nonexistent_load_test.env"))
	err := p.loadFiles()
	if err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
}

func TestFileProvider_loadFile_SkipsLinesWithoutEquals(t *testing.T) {
	// A line with no '=' should be silently ignored.
	dir := t.TempDir()
	content := "NOEQUALSSIGN\nVALID=yes\n"
	path := writeTempEnvFile(t, dir, ".env", content)
	p := NewFileProvider(WithFiles(path))

	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range keys {
		if k == "NOEQUALSSIGN" {
			t.Errorf("line without '=' should have been skipped, but key %q appeared", k)
		}
	}

	s, err := p.Get(context.Background(), "VALID")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "yes" {
		t.Errorf("expected yes, got %q", s.Value)
	}
}

func TestFileProvider_Get_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := writeTempEnvFile(t, dir, ".env", "UPPER_KEY=casevalue\n")
	p := NewFileProvider(WithFiles(path))

	// lowercase lookup of an uppercase key should find it via ToUpper fallback.
	s, err := p.Get(context.Background(), "upper_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "casevalue" {
		t.Errorf("expected casevalue, got %q", s.Value)
	}
}

func TestFileProvider_DefaultFilesUsedWhenNoneProvided(t *testing.T) {
	// When no WithFiles option is given, NewFileProvider should populate a default
	// list (.env, .env.local, .env.secrets).  We just verify it doesn't panic and
	// returns without error (files simply won't exist in the test environment).
	p := NewFileProvider()
	if len(p.files) == 0 {
		t.Error("expected default files to be set when none provided")
	}
	// loadFiles with default files that don't exist should not return an error.
	err := p.loadFiles()
	if err != nil {
		t.Errorf("unexpected error from loadFiles with default nonexistent files: %v", err)
	}
}
