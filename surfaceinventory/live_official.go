//go:build official_sdk

package surfaceinventory

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectAndListNames spawns argv[0] (with the remaining elements as its
// arguments) as an MCP server subprocess, connects to it as a client over
// stdio via mcp.CommandTransport, and enumerates every live-registered
// surface name of the given kind using ClientSession's auto-paginating
// iterators (Tools/Resources/Prompts — iter.Seq2[*T, error]), which
// transparently follow list-result cursors so callers never see a
// truncated page.
func connectAndListNames(ctx context.Context, argv []string, kind string) ([]string, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "surface-inventory-live",
		Version: "0.0.0",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", argv[0], err)
	}
	defer session.Close()

	var names []string
	switch kind {
	case KindMCPTool:
		for t, err := range session.Tools(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list tools: %w", err)
			}
			names = append(names, t.Name)
		}
	case KindMCPResource:
		for r, err := range session.Resources(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list resources: %w", err)
			}
			names = append(names, r.URI)
		}
	case KindMCPPrompt:
		for p, err := range session.Prompts(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list prompts: %w", err)
			}
			names = append(names, p.Name)
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
	return names, nil
}
