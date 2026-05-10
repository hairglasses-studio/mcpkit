package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hairglasses-studio/mcpkit/discovery"
)

func main() {
	os.Exit(runPublishCLI(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func runPublishCLI(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("mcpkit-publish", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cardPath := fs.String("card", "", "path to .well-known/mcp.json or ServerMetadata JSON")
	registryURL := fs.String("registry-url", discovery.DefaultRegistryURL, "MCP Registry API base URL")
	token := fs.String("token", "", "registry bearer token; defaults to MCP_REGISTRY_TOKEN")
	modeRaw := fs.String("mode", string(discovery.PublishModeRegister), "publish mode: register or update")
	serverID := fs.String("server-id", "", "server id for update mode; defaults to metadata id")
	validateOnly := fs.Bool("validate-only", false, "validate metadata without calling the registry")
	timeout := fs.Duration("timeout", 30*time.Second, "publish request timeout")
	jsonOutput := fs.Bool("json", true, "write workflow result as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *cardPath == "" {
		fmt.Fprintln(stderr, "mcpkit-publish: -card is required")
		return 2
	}

	meta, err := readServerMetadata(*cardPath)
	if err != nil {
		fmt.Fprintf(stderr, "mcpkit-publish: %v\n", err)
		return 1
	}

	mode := discovery.PublishMode(*modeRaw)
	tokenValue := *token
	if tokenValue == "" {
		tokenValue = getenv("MCP_REGISTRY_TOKEN")
	}
	if tokenValue == "" && !*validateOnly {
		fmt.Fprintln(stderr, "mcpkit-publish: -token or MCP_REGISTRY_TOKEN is required unless -validate-only is set")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := discovery.RunPublishWorkflow(ctx, discovery.PublishWorkflowConfig{
		PublisherConfig: discovery.PublisherConfig{
			BaseURL: *registryURL,
			Token:   tokenValue,
		},
		Metadata:     meta,
		Mode:         mode,
		ServerID:     *serverID,
		ValidateOnly: *validateOnly,
	})
	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	} else if result != nil {
		fmt.Fprintf(stdout, "%s: %s\n", result.Mode, result.Validation.Summary())
	}
	if err != nil {
		fmt.Fprintf(stderr, "mcpkit-publish: %v\n", err)
		return 1
	}
	return 0
}

func readServerMetadata(path string) (discovery.ServerMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return discovery.ServerMetadata{}, fmt.Errorf("open card: %w", err)
	}
	defer f.Close()

	var meta discovery.ServerMetadata
	dec := json.NewDecoder(f)
	if err := dec.Decode(&meta); err != nil {
		return discovery.ServerMetadata{}, fmt.Errorf("decode card: %w", err)
	}
	return meta, nil
}
