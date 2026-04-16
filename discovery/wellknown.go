//go:build !official_sdk

package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ServerCard wraps ServerMetadata with generation timestamp for the
// .well-known/mcp.json endpoint.
type ServerCard struct {
	ServerMetadata
	GeneratedAt time.Time `json:"generated_at"`
}

// ServerCardHandler returns an http.Handler that serves a server card at
// .well-known/mcp.json. It calls MetadataFromConfig(cfg) on each request
// so the card reflects the current registry state.
func ServerCardHandler(cfg MetadataConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		meta := MetadataFromConfig(cfg)
		card := ServerCard{
			ServerMetadata: meta,
			GeneratedAt:    time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")

		if r.Method == http.MethodHead {
			return
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(card)
	})
}

// StaticServerCardHandler returns an http.Handler that serves a pre-built
// ServerMetadata as a server card. The metadata is fixed at creation time
// (GeneratedAt is set once). Use this when the registry is not expected to
// change at runtime.
func StaticServerCardHandler(meta ServerMetadata) http.Handler {
	card := ServerCard{
		ServerMetadata: meta,
		GeneratedAt:    time.Now().UTC(),
	}

	data, _ := json.MarshalIndent(card, "", "  ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")

		if r.Method == http.MethodHead {
			return
		}

		w.Write(data)
		w.Write([]byte("\n"))
	})
}

// WriteFile writes a ServerCard to dest atomically using a sibling temp file
// followed by an OS rename. This ensures that concurrent readers never see a
// partially-written file.
//
// dest is typically ".well-known/mcp.json". Parent directories are created if
// they do not exist. The file is written with mode 0o644.
//
// Example usage in a server binary:
//
//	card := discovery.ServerCard{
//	    ServerMetadata: discovery.MetadataFromConfig(cfg),
//	    GeneratedAt:    time.Now().UTC(),
//	}
//	if err := discovery.WriteFile(".well-known/mcp.json", card); err != nil {
//	    log.Fatal(err)
//	}
func WriteFile(dest string, card ServerCard) error {
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("discovery: marshal server card: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("discovery: create parent dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".mcp-card-*.json.tmp")
	if err != nil {
		return fmt.Errorf("discovery: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("discovery: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("discovery: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("discovery: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("discovery: rename temp file to %s: %w", dest, err)
	}
	return nil
}

// ContractWriteFlag is the conventional CLI flag name used by mcpkit servers
// to trigger atomic .well-known/mcp.json generation at startup.
//
// Add this flag to your server's flag set:
//
//	contractWrite := flag.String(discovery.ContractWriteFlag, "", "write .well-known/mcp.json to this path and exit")
//	flag.Parse()
//	if err := discovery.HandleContractWrite(*contractWrite, cfg); err != nil {
//	    log.Fatal(err)
//	}
const ContractWriteFlag = "contract-write"

// HandleContractWrite checks whether dest is non-empty and, if so, generates a
// ServerCard from cfg, writes it atomically to dest, and returns
// ErrContractWritten. Callers should exit cleanly when ErrContractWritten is
// returned. If dest is empty HandleContractWrite is a no-op and returns nil.
//
// This implements the --contract-write flag convention: pass the flag value
// directly to this function. The server generates its manifest and exits
// without starting any transport, making it suitable for CI pipelines and
// repo generation scripts that collect .well-known/mcp.json from binaries.
//
// Example:
//
//	flag.Parse()
//	if err := discovery.HandleContractWrite(*contractWrite, cfg); err != nil {
//	    if errors.Is(err, discovery.ErrContractWritten) {
//	        os.Exit(0)
//	    }
//	    log.Fatal(err)
//	}
func HandleContractWrite(dest string, cfg MetadataConfig) error {
	if dest == "" {
		return nil
	}
	card := ServerCard{
		ServerMetadata: MetadataFromConfig(cfg),
		GeneratedAt:    time.Now().UTC(),
	}
	if err := WriteFile(dest, card); err != nil {
		return err
	}
	return ErrContractWritten
}

// ErrContractWritten is returned by HandleContractWrite when the contract file
// was successfully written. Callers should treat this as a clean exit signal.
var ErrContractWritten = fmt.Errorf("discovery: contract written")
