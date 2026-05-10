package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
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
	tokenURL := fs.String("token-url", "", "OAuth2 token URL; defaults to MCP_REGISTRY_TOKEN_URL")
	clientID := fs.String("client-id", "", "OAuth2 client id; defaults to MCP_REGISTRY_CLIENT_ID")
	clientSecret := fs.String("client-secret", "", "OAuth2 client secret; defaults to MCP_REGISTRY_CLIENT_SECRET")
	scopes := fs.String("scopes", "", "comma-separated OAuth2 scopes; defaults to MCP_REGISTRY_SCOPES")
	audience := fs.String("audience", "", "OAuth2 audience; defaults to MCP_REGISTRY_AUDIENCE")
	useBasicAuth := fs.Bool("client-basic-auth", false, "send OAuth2 client credentials with HTTP Basic auth")

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
	var tokenSource discovery.RegistryTokenSource
	if tokenValue == "" && !*validateOnly {
		oauthTokenURL := firstNonEmpty(*tokenURL, getenv("MCP_REGISTRY_TOKEN_URL"))
		oauthClientID := firstNonEmpty(*clientID, getenv("MCP_REGISTRY_CLIENT_ID"))
		oauthClientSecret := firstNonEmpty(*clientSecret, getenv("MCP_REGISTRY_CLIENT_SECRET"))
		oauthScopes := firstNonEmpty(*scopes, getenv("MCP_REGISTRY_SCOPES"))
		oauthAudience := firstNonEmpty(*audience, getenv("MCP_REGISTRY_AUDIENCE"))
		if oauthTokenURL != "" || oauthClientID != "" || oauthClientSecret != "" {
			source, err := discovery.NewClientCredentialsTokenSource(discovery.ClientCredentialsConfig{
				TokenURL:     oauthTokenURL,
				ClientID:     oauthClientID,
				ClientSecret: oauthClientSecret,
				Scopes:       splitCSV(oauthScopes),
				Audience:     oauthAudience,
				UseBasicAuth: *useBasicAuth,
			})
			if err != nil {
				fmt.Fprintf(stderr, "mcpkit-publish: %v\n", err)
				return 2
			}
			tokenSource = source
		}
	}
	if tokenValue == "" && tokenSource == nil && !*validateOnly {
		fmt.Fprintln(stderr, "mcpkit-publish: -token, MCP_REGISTRY_TOKEN, or OAuth2 client credentials are required unless -validate-only is set")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := discovery.RunPublishWorkflow(ctx, discovery.PublishWorkflowConfig{
		PublisherConfig: discovery.PublisherConfig{
			BaseURL:     *registryURL,
			Token:       tokenValue,
			TokenSource: tokenSource,
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
