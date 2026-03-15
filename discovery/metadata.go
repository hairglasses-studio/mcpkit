//go:build !official_sdk

package discovery

import (
	"sort"

	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/resources"
)

// MetadataConfig holds all inputs needed to build a ServerMetadata entry for
// publication to the MCP Registry. All registry fields are optional — pass nil
// to omit the corresponding section.
type MetadataConfig struct {
	// Required server identity fields.
	Name         string
	Description  string
	Version      string
	Organization string
	Repository   string

	// Tags are arbitrary labels attached to the server entry.
	Tags []string

	// Auth describes the authentication mechanism required by the server.
	Auth *AuthRequirement

	// Optional registries — nil values are handled gracefully.
	Tools     *registry.ToolRegistry
	Resources *resources.ResourceRegistry
	Prompts   *prompts.PromptRegistry

	// Transports lists the network endpoints exposed by the server.
	Transports []TransportInfo
}

// MetadataFromConfig builds a ServerMetadata from a MetadataConfig. It extracts
// tool, resource, and prompt summaries from the provided registries (if any),
// sorts them for deterministic output, and merges them with the server-level
// fields supplied in cfg.
func MetadataFromConfig(cfg MetadataConfig) ServerMetadata {
	meta := ServerMetadata{
		Name:         cfg.Name,
		Description:  cfg.Description,
		Version:      cfg.Version,
		Organization: cfg.Organization,
		Repository:   cfg.Repository,
		Tags:         cfg.Tags,
		Auth:         cfg.Auth,
		Transports:   cfg.Transports,
	}

	// Extract tool summaries.
	if cfg.Tools != nil {
		defs := cfg.Tools.GetAllToolDefinitions()
		sort.Slice(defs, func(i, j int) bool {
			return defs[i].Tool.Name < defs[j].Tool.Name
		})
		tools := make([]ToolSummary, 0, len(defs))
		for _, td := range defs {
			tools = append(tools, ToolSummary{
				Name:        td.Tool.Name,
				Description: td.Tool.Description,
			})
		}
		meta.Tools = tools
	}

	// Extract resource summaries (static resources only).
	if cfg.Resources != nil {
		rdefs := cfg.Resources.GetAllResourceDefinitions()
		sort.Slice(rdefs, func(i, j int) bool {
			return rdefs[i].Resource.URI < rdefs[j].Resource.URI
		})
		resSummaries := make([]ResourceSummary, 0, len(rdefs))
		for _, rd := range rdefs {
			resSummaries = append(resSummaries, ResourceSummary{
				URITemplate: rd.Resource.URI,
				Name:        rd.Resource.Name,
				Description: rd.Resource.Description,
			})
		}

		// Also extract template summaries.
		tdefs := cfg.Resources.GetAllTemplateDefinitions()
		sort.Slice(tdefs, func(i, j int) bool {
			return tdefs[i].Template.URITemplate.Raw() < tdefs[j].Template.URITemplate.Raw()
		})
		for _, td := range tdefs {
			resSummaries = append(resSummaries, ResourceSummary{
				URITemplate: td.Template.URITemplate.Raw(),
				Name:        td.Template.Name,
				Description: td.Template.Description,
			})
		}

		meta.Resources = resSummaries
	}

	// Extract prompt summaries.
	if cfg.Prompts != nil {
		pdefs := cfg.Prompts.GetAllPromptDefinitions()
		sort.Slice(pdefs, func(i, j int) bool {
			return pdefs[i].Prompt.Name < pdefs[j].Prompt.Name
		})
		promptSummaries := make([]PromptSummary, 0, len(pdefs))
		for _, pd := range pdefs {
			promptSummaries = append(promptSummaries, PromptSummary{
				Name:        pd.Prompt.Name,
				Description: pd.Prompt.Description,
			})
		}
		meta.Prompts = promptSummaries
	}

	return meta
}
