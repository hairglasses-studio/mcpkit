// Command openapi demonstrates an MCP server powered by the mcpkit OpenAPI bridge.
//
// It starts an in-process mock REST API with an OpenAPI v3 specification,
// converts the API operations into MCP tools using openapi.NewBridgeFromSpec,
// and exposes them via an MCP server.
//
// Usage:
//
//	go run ./examples/openapi
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/hairglasses-studio/mcpkit/bridge/openapi"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// Pet represents a sample resource in the mock API.
type Pet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// openAPISpecJSON is a complete OpenAPI v3 spec for the mock pet store API.
const openAPISpecJSON = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Pet Store API",
    "version": "1.0.0",
    "description": "Mock API for testing mcpkit OpenAPI bridge"
  },
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
        "summary": "List all pets in the store",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "description": "Maximum number of pets to return",
            "required": false,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "A list of pets"
          }
        }
      },
      "post": {
        "operationId": "createPet",
        "summary": "Create a new pet",
        "requestBody": {
          "description": "Pet object to create",
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Pet created"
          }
        }
      }
    },
    "/pets/{id}": {
      "get": {
        "operationId": "getPetById",
        "summary": "Get a pet by ID",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "description": "ID of the pet to retrieve",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Pet detail"
          },
          "404": {
            "description": "Pet not found"
          }
        }
      }
    }
  }
}`

func main() {
	// 1. Start in-process mock REST API server
	petsDB := map[string]Pet{
		"1": {ID: "1", Name: "Fido", Kind: "dog"},
		"2": {ID: "2", Name: "Whiskers", Kind: "cat"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(petsDB)
		case http.MethodPost:
			var p Pet
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if p.ID == "" {
				p.ID = fmt.Sprintf("%d", len(petsDB)+1)
			}
			petsDB[p.ID] = p
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(p)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/pets/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := r.URL.Path[len("/pets/"):]
		if p, ok := petsDB[id]; ok {
			json.NewEncoder(w).Encode(p)
			return
		}
		http.Error(w, `{"error":"Pet not found"}`, http.StatusNotFound)
	})

	mockServer := httptest.NewServer(mux)
	defer mockServer.Close()

	// 2. Parse OpenAPI v3 specification
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData([]byte(openAPISpecJSON))
	if err != nil {
		log.Fatalf("Failed to parse OpenAPI spec: %v", err)
	}

	// 3. Create Tool Registry & OpenAPI Bridge
	reg := registry.NewToolRegistry()
	bridge, err := openapi.NewBridgeFromSpec(spec, reg, openapi.BridgeConfig{
		BaseURL:   mockServer.URL,
		NameStyle: "operationId",
		Timeout:   5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create OpenAPI bridge: %v", err)
	}

	if err := bridge.RegisterTools(); err != nil {
		log.Fatalf("Failed to register OpenAPI tools: %v", err)
	}

	log.Printf("OpenAPI bridge initialized with %d tools from spec", bridge.ToolCount())

	// 4. Attach to MCP server and serve over stdio
	s := registry.NewMCPServer("openapi-example", "1.0.0")
	reg.RegisterWithServer(s)

	ctx := context.Background()
	_ = ctx

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
