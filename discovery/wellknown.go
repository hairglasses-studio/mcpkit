//go:build !official_sdk

package discovery

import (
	"encoding/json"
	"net/http"
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
