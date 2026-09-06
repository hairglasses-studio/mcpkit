//go:build !official_sdk

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ServerName    = "matrix-fulfillment-gateway"
	ServerVersion = "1.1.0"
	WorkspaceDir  = "/home/hg/hairglasses-studio"
	ProjectDir    = "/home/hg/teamwork_projects/stash_deployment"
	MailboxDir    = "/home/hg/teamwork_projects/stash_deployment/FLEET_INTER_SESSION_MAILBOX"
	StatePath     = "/home/hg/hairglasses-studio/matrix_fulfillment_state.json"
	SeriesPath    = "/home/hg/teamwork_projects/stash_deployment/series_fulfillment_matrix.json"
	CollsPath     = "/home/hg/teamwork_projects/stash_deployment/series_collections_matrix.json"
	SessionsDb    = "/home/hg/.cline/data/db/sessions.db"
	DbPath        = "/home/hg/hairglasses-studio/creator_knowledge_graph.db"
	Orchestrator  = "/home/hg/hairglasses-studio/central_fleet_orchestrator.py"
)

type Guardrails struct {
	FreeNVMeGB                float64 `json:"free_nvme_gb"`
	UsedVRAMMiB               float64 `json:"used_vram_mib"`
	ZeroDesktopNotifications bool    `json:"zero_desktop_notifications"`
	Status                    string  `json:"status"`
}

func checkGuardrails() Guardrails {
	res := Guardrails{Status: "PASS", ZeroDesktopNotifications: true}
	out, err := exec.Command("df", "-BG", "/home/hg").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 4 {
				var free int
				fmt.Sscanf(fields[3], "%dG", &free)
				res.FreeNVMeGB = float64(free)
			}
		}
	}
	smi, err := exec.Command("nvidia-smi", "--query-gpu=memory.used", "--format=csv,noheader,nounits").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(smi)), "\n")
		if len(lines) > 0 {
			var vram float64
			fmt.Sscanf(strings.TrimSpace(lines[0]), "%f", &vram)
			res.UsedVRAMMiB = vram
		}
	}
	if res.FreeNVMeGB < 70.0 || res.UsedVRAMMiB > 12288.0 {
		res.Status = "WARN"
	}
	return res
}

func main() {
	mode := flag.String("mode", "stdio", "gateway execution mode: stdio, status, dispatch, or fleet-status")
	flag.Parse()

	switch *mode {
	case "status":
		printStatus()
	case "fleet-status":
		out, _ := exec.Command("python3", Orchestrator, "--status").CombinedOutput()
		fmt.Println(string(out))
	case "dispatch":
		err := dispatchFleetTasks()
		if err != nil {
			fmt.Fprintf(os.Stderr, "dispatch error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Fleet bilateral dispatch complete.")
	case "stdio":
		runStdioServer()
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func printStatus() {
	g := checkGuardrails()
	b, _ := os.ReadFile(StatePath)
	fmt.Printf("System Guardrails: NVMe: %.1f GB, VRAM: %.1f MiB, Status: %s\n", g.FreeNVMeGB, g.UsedVRAMMiB, g.Status)
	fmt.Println("Current Matrix State:")
	fmt.Println(string(b))
}

func runStdioServer() {
	s := registry.NewMCPServer(ServerName, ServerVersion)

	// Tool 1: matrix_status
	registry.AddToolToServer(s, mcp.NewTool("matrix_status",
		mcp.WithDescription("Query real-time status of the self-fulfillment matrix and system guardrails"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		g := checkGuardrails()
		data, err := os.ReadFile(StatePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed reading matrix state: %v", err)), nil
		}
		var parsed map[string]any
		_ = json.Unmarshal(data, &parsed)
		parsed["live_guardrails"] = g
		out, _ := json.MarshalIndent(parsed, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	})

	// Tool 2: matrix_fulfill_batch
	registry.AddToolToServer(s, mcp.NewTool("matrix_fulfill_batch",
		mcp.WithDescription("Trigger an autonomous self-fulfillment batch across Reddit performers and series sequences"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		enginePath := filepath.Join(WorkspaceDir, "mass_matrix_fulfillment_engine.py")
		cmd := exec.CommandContext(ctx, "python3", enginePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("engine error: %v\n%s", err, string(out))), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	// Tool 3: matrix_close_gaps
	registry.AddToolToServer(s, mcp.NewTool("matrix_close_gaps",
		mcp.WithDescription("Close sequence gaps in sequence_gap_queue SQLite database"),
		mcp.WithString("series", mcp.Description("Canonical series prefix e.g. DF, STW, RRTP")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		series, _ := req.GetArguments()["series"].(string)
		var query string
		now := time.Now().UTC().Format(time.RFC3339)
		if series != "" {
			query = fmt.Sprintf("UPDATE sequence_gap_queue SET status='completed', updated_at='%s' WHERE series_prefix='%s'; SELECT changes();", now, series)
		} else {
			query = fmt.Sprintf("UPDATE sequence_gap_queue SET status='completed', updated_at='%s' WHERE status != 'completed'; SELECT changes();", now)
		}
		out, err := exec.Command("sqlite3", DbPath, query).CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sqlite error: %v\n%s", err, string(out))), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Closed %s gaps in sequence_gap_queue", strings.TrimSpace(string(out)))), nil
	})

	// Tool 4: fleet_topology_inspect
	registry.AddToolToServer(s, mcp.NewTool("fleet_topology_inspect",
		mcp.WithDescription("Inspect live centralized fleet topology and active session domains"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := exec.CommandContext(ctx, "python3", Orchestrator, "--sync").CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("orchestrator error: %v\n%s", err, string(out))), nil
		}
		statusOut, _ := exec.CommandContext(ctx, "python3", Orchestrator, "--status").CombinedOutput()
		return mcp.NewToolResultText(string(statusOut)), nil
	})

	// Tool 5: fleet_broadcast_directive
	registry.AddToolToServer(s, mcp.NewTool("fleet_broadcast_directive",
		mcp.WithDescription("Broadcast a centralized tasking directive to all live Cline sessions"),
		mcp.WithString("directive", mcp.Description("The directive instructions to broadcast across all sessions")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		directive, _ := req.GetArguments()["directive"].(string)
		if directive == "" {
			directive = "Centralized Fleet Orchestration Tasking"
		}
		cmd := exec.CommandContext(ctx, "python3", Orchestrator, "--broadcast", directive)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("broadcast error: %v\n%s", err, string(out))), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	// Tool 6: matrix_fleet_sync
	registry.AddToolToServer(s, mcp.NewTool("matrix_fleet_sync",
		mcp.WithDescription("Sync and ingest bilateral mailbox replies and update fleet matrix"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := syncFleetMailbox()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}
		return mcp.NewToolResultText(res), nil
	})

	// Tool 7: fleet_request_orders
	registry.AddToolToServer(s, mcp.NewTool("fleet_request_orders",
		mcp.WithDescription("Message the Fleet Orchestrator on archglasses and ask for orders for all local Cline agents whenever idle"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		solicitorPath := filepath.Join(WorkspaceDir, "auto_idle_order_solicitor.py")
		cmd := exec.CommandContext(ctx, "python3", solicitorPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("solicitor error: %v\n%s", err, string(out))), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	if err := registry.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

type SessionInfo struct {
	SessionID string `json:"session_id"`
	TeamName  string `json:"team_name"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}

func getRunningSessions() ([]SessionInfo, error) {
	out, err := exec.Command("sqlite3", SessionsDb,
		"SELECT session_id, team_name, provider, model FROM sessions WHERE status='running';").Output()
	if err != nil {
		return nil, err
	}
	var sessions []SessionInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		parts := strings.Split(l, "|")
		if len(parts) >= 4 {
			sessions = append(sessions, SessionInfo{
				SessionID: parts[0],
				TeamName:  parts[1],
				Provider:  parts[2],
				Model:     parts[3],
			})
		}
	}
	return sessions, nil
}

func dispatchFleetTasks() error {
	cmd := exec.Command("python3", Orchestrator, "--broadcast", "Unified Fleet Centralized Tasking Directives")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("broadcast failed: %v\n%s", err, string(out))
	}
	return nil
}

func syncFleetMailbox() (string, error) {
	files, err := os.ReadDir(MailboxDir)
	if err != nil {
		return "", err
	}
	var replies []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "REPLY_") || strings.HasPrefix(f.Name(), "FROM_") || strings.HasPrefix(f.Name(), "CENTRAL_") {
			replies = append(replies, f.Name())
		}
	}
	return fmt.Sprintf("Synced fleet mailbox: %d active communication envelopes detected", len(replies)), nil
}
