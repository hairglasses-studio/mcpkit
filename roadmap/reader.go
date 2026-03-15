package roadmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadRoadmap reads and parses a Roadmap from a JSON file at the given path.
func LoadRoadmap(path string) (*Roadmap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("roadmap: load: %w", err)
	}
	var rm Roadmap
	if err := json.Unmarshal(data, &rm); err != nil {
		return nil, fmt.Errorf("roadmap: parse: %w", err)
	}
	return &rm, nil
}

// SaveRoadmap atomically writes a Roadmap to a JSON file (write tmp + rename).
func SaveRoadmap(path string, rm *Roadmap) error {
	data, err := json.MarshalIndent(rm, "", "  ")
	if err != nil {
		return fmt.Errorf("roadmap: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".roadmap-*.tmp")
	if err != nil {
		return fmt.Errorf("roadmap: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("roadmap: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("roadmap: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("roadmap: rename file: %w", err)
	}
	return nil
}

// RenderMarkdown generates a markdown string with XML-tagged sections for agent parsing.
// Each phase is wrapped in <roadmap-phase> tags and each item in <roadmap-item> tags.
// Tiers are wrapped in <roadmap-tier> tags.
func RenderMarkdown(rm *Roadmap) string {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(rm.Title)
	sb.WriteString("\n\n")

	if rm.Description != "" {
		sb.WriteString(rm.Description)
		sb.WriteString("\n\n")
	}

	for _, phase := range rm.Phases {
		fmt.Fprintf(&sb, "<%s id=%q status=%q name=%q>\n\n",
			TagRoadmapPhase, phase.ID, string(phase.Status), phase.Name)

		for _, item := range phase.Items {
			attrs := fmt.Sprintf("id=%q", item.ID)
			if item.Package != "" {
				attrs += fmt.Sprintf(" package=%q", item.Package)
			}
			attrs += fmt.Sprintf(" status=%q", string(item.Status))
			fmt.Fprintf(&sb, "<%s %s>\n%s\n</%s>\n\n",
				TagRoadmapItem, attrs, item.Description, TagRoadmapItem)
		}

		fmt.Fprintf(&sb, "</%s>\n\n", TagRoadmapPhase)
	}

	for _, tier := range rm.Tiers {
		fmt.Fprintf(&sb, "<%s id=%q name=%q>\n\n",
			TagRoadmapTier, tier.ID, tier.Name)

		for _, item := range tier.Items {
			attrs := fmt.Sprintf("id=%q", item.ID)
			if item.Package != "" {
				attrs += fmt.Sprintf(" package=%q", item.Package)
			}
			attrs += fmt.Sprintf(" status=%q", string(item.Status))
			fmt.Fprintf(&sb, "<%s %s>\n%s\n</%s>\n\n",
				TagRoadmapItem, attrs, item.Description, TagRoadmapItem)
		}

		fmt.Fprintf(&sb, "</%s>\n\n", TagRoadmapTier)
	}

	if rm.UpdatedAt != "" {
		fmt.Fprintf(&sb, "_Updated: %s_\n", rm.UpdatedAt)
	}

	return sb.String()
}
