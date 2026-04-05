package configutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadJSON reads a JSON file and unmarshals it into a value of type T.
func LoadJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("configutil: read %s: %w", filepath.Base(path), err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, fmt.Errorf("configutil: parse %s: %w", filepath.Base(path), err)
	}
	return v, nil
}

// SaveJSON atomically writes a value as indented JSON to the given path.
// It writes to a temporary file in the same directory, then renames it
// to the target path. This prevents partial reads if another process is
// watching the file.
func SaveJSON[T any](path string, v T) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("configutil: marshal: %w", err)
	}
	// Append newline for POSIX compliance
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("configutil: create directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".configutil-*.tmp")
	if err != nil {
		return fmt.Errorf("configutil: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("configutil: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("configutil: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("configutil: rename file: %w", err)
	}
	return nil
}

// LoadJSONWithDefault reads a JSON config file. If the file does not exist,
// it returns the provided default value without error.
func LoadJSONWithDefault[T any](path string, defaultVal T) (T, error) {
	v, err := LoadJSON[T](path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultVal, nil
		}
		return defaultVal, err
	}
	return v, nil
}
