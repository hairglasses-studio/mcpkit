package rdcycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileArtifactStore implements ArtifactStore by persisting each artifact as a JSON file.
type FileArtifactStore struct {
	mu  sync.RWMutex
	dir string
}

// NewFileArtifactStore creates a FileArtifactStore that writes to the given directory.
// The directory is created if it does not exist.
func NewFileArtifactStore(dir string) (*FileArtifactStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("filestore: create dir: %w", err)
	}
	return &FileArtifactStore{dir: dir}, nil
}

// Save writes an artifact to <dir>/<id>.json.
func (s *FileArtifactStore) Save(artifact Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("filestore: marshal: %w", err)
	}

	path := filepath.Join(s.dir, sanitizeID(artifact.ID)+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("filestore: write: %w", err)
	}
	return nil
}

// Get reads an artifact by ID from disk.
func (s *FileArtifactStore) Get(id string) (Artifact, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, sanitizeID(id)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, false
	}

	var a Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return Artifact{}, false
	}
	return a, true
}

// List returns all artifacts matching the given type. If artifactType is empty,
// all artifacts are returned.
func (s *FileArtifactStore) List(artifactType string) []Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}

	var result []Artifact
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var a Artifact
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		if artifactType == "" || a.Type == artifactType {
			result = append(result, a)
		}
	}
	return result
}

// Dir returns the directory where artifacts are stored.
func (s *FileArtifactStore) Dir() string {
	return s.dir
}

// sanitizeID replaces path separators and other problematic characters
// in artifact IDs so they can be used as filenames.
func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(id)
}
