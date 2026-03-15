//go:build !official_sdk

// Package bootstrap provides agent workspace initialization and capability reporting.
//
// It generates a ContextReport summarizing all registered tools, resources, prompts,
// and active extensions so agents can self-describe their capabilities at startup.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hairglasses-studio/mcpkit/extensions"
	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// ContextReport summarizes all capabilities of a running MCP server.
type ContextReport struct {
	ServerName  string            `json:"server_name"`
	Tools       []ToolSummary     `json:"tools"`
	Resources   []ResourceSummary `json:"resources,omitempty"`
	Prompts     []PromptSummary   `json:"prompts,omitempty"`
	Extensions  []string          `json:"extensions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// ToolSummary is a concise representation of a registered tool.
type ToolSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsWrite     bool     `json:"is_write,omitempty"`
}

// ResourceSummary is a concise representation of a registered resource.
type ResourceSummary struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PromptSummary is a concise representation of a registered prompt.
type PromptSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Arguments   []string `json:"arguments,omitempty"`
}

// Config holds all registries needed to generate a ContextReport.
type Config struct {
	ServerName string
	Tools      *registry.ToolRegistry
	Resources  *resources.ResourceRegistry
	Prompts    *prompts.PromptRegistry
	Extensions *extensions.ExtensionRegistry
	Metadata   map[string]string
}

// GenerateReport builds a ContextReport from the provided Config.
// Any nil registry is silently skipped; its section will be absent from the report.
func GenerateReport(config Config) *ContextReport {
	report := &ContextReport{
		ServerName:  config.ServerName,
		Metadata:    config.Metadata,
		GeneratedAt: time.Now(),
	}

	if config.Tools != nil {
		for _, td := range config.Tools.GetAllToolDefinitions() {
			report.Tools = append(report.Tools, ToolSummary{
				Name:        td.Tool.Name,
				Description: td.Tool.Description,
				Category:    td.Category,
				Tags:        td.Tags,
				IsWrite:     td.IsWrite,
			})
		}
	}

	if config.Resources != nil {
		for _, rd := range config.Resources.GetAllResourceDefinitions() {
			report.Resources = append(report.Resources, ResourceSummary{
				URI:         rd.Resource.URI,
				Name:        rd.Resource.Name,
				Description: rd.Resource.Description,
			})
		}
	}

	if config.Prompts != nil {
		for _, pd := range config.Prompts.GetAllPromptDefinitions() {
			ps := PromptSummary{
				Name:        pd.Prompt.Name,
				Description: pd.Prompt.Description,
			}
			for _, arg := range pd.Prompt.Arguments {
				ps.Arguments = append(ps.Arguments, arg.Name)
			}
			report.Prompts = append(report.Prompts, ps)
		}
	}

	if config.Extensions != nil {
		for _, ext := range config.Extensions.Active() {
			report.Extensions = append(report.Extensions, ext.Name)
		}
	}

	return report
}

// FormatText returns a human-readable text representation of the report.
func (r *ContextReport) FormatText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Server: %s\n", r.ServerName)
	fmt.Fprintf(&b, "Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339))

	if len(r.Tools) > 0 {
		fmt.Fprintf(&b, "Tools (%d):\n", len(r.Tools))
		for _, t := range r.Tools {
			writeFlag := ""
			if t.IsWrite {
				writeFlag = " [write]"
			}
			fmt.Fprintf(&b, "  - %s: %s%s\n", t.Name, t.Description, writeFlag)
		}
		b.WriteString("\n")
	}

	if len(r.Resources) > 0 {
		fmt.Fprintf(&b, "Resources (%d):\n", len(r.Resources))
		for _, res := range r.Resources {
			fmt.Fprintf(&b, "  - %s (%s)\n", res.Name, res.URI)
		}
		b.WriteString("\n")
	}

	if len(r.Prompts) > 0 {
		fmt.Fprintf(&b, "Prompts (%d):\n", len(r.Prompts))
		for _, p := range r.Prompts {
			if len(p.Arguments) > 0 {
				fmt.Fprintf(&b, "  - %s(%s)\n", p.Name, strings.Join(p.Arguments, ", "))
			} else {
				fmt.Fprintf(&b, "  - %s\n", p.Name)
			}
		}
		b.WriteString("\n")
	}

	if len(r.Extensions) > 0 {
		fmt.Fprintf(&b, "Extensions (%d):\n", len(r.Extensions))
		for _, ext := range r.Extensions {
			fmt.Fprintf(&b, "  - %s\n", ext)
		}
		b.WriteString("\n")
	}

	if len(r.Metadata) > 0 {
		b.WriteString("Metadata:\n")
		for k, v := range r.Metadata {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}

	return b.String()
}

// FormatJSON returns the report serialized as indented JSON.
func (r *ContextReport) FormatJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
