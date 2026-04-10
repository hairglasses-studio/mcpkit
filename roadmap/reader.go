package roadmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LoadRoadmap reads and parses a Roadmap from a JSON or Markdown file at the given path.
// Markdown files are identified by .md or .markdown extensions and must contain XML-tagged sections.
func LoadRoadmap(path string) (*Roadmap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("roadmap: load: %w", err)
	}

	if strings.HasSuffix(strings.ToLower(path), ".md") || strings.HasSuffix(strings.ToLower(path), ".markdown") {
		return ParseMarkdown(string(data))
	}

	var rm Roadmap
	if err := json.Unmarshal(data, &rm); err != nil {
		return nil, fmt.Errorf("roadmap: parse: %w", err)
	}
	return &rm, nil
}

// ParseMarkdown extracts Roadmap data from a markdown string containing XML-tagged sections.
func ParseMarkdown(content string) (*Roadmap, error) {
	rm := &Roadmap{}

	// Extract title: # Title
	reTitle := regexp.MustCompile(`(?m)^#\s+(.*)$`)
	if match := reTitle.FindStringSubmatch(content); len(match) > 1 {
		rm.Title = strings.TrimSpace(match[1])
	}

	// Extract updated_at: _Updated: 2026-03-15_
	reUpdated := regexp.MustCompile(`_Updated:\s*([^_]*)_`)
	if match := reUpdated.FindStringSubmatch(content); len(match) > 1 {
		rm.UpdatedAt = strings.TrimSpace(match[1])
	}

	// Extract phases
	phaseRegex := regexp.MustCompile(`(?s)<roadmap-phase\s+([^>]+)>(.*?)</roadmap-phase>`)
	phaseMatches := phaseRegex.FindAllStringSubmatch(content, -1)
	for _, match := range phaseMatches {
		attrs := parseAttributes(match[1])
		phase := Phase{
			ID:     attrs["id"],
			Name:   attrs["name"],
			Status: PhaseStatus(attrs["status"]),
		}
		phase.Items = parseItems(match[2])
		rm.Phases = append(rm.Phases, phase)
	}

	// Extract tiers
	tierRegex := regexp.MustCompile(`(?s)<roadmap-tier\s+([^>]+)>(.*?)</roadmap-tier>`)
	tierMatches := tierRegex.FindAllStringSubmatch(content, -1)
	for _, match := range tierMatches {
		attrs := parseAttributes(match[1])
		tier := Tier{
			ID:   attrs["id"],
			Name: attrs["name"],
		}
		tier.Items = parseItems(match[2])
		rm.Tiers = append(rm.Tiers, tier)
	}

	return rm, nil
}

func parseAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(attrStr, -1)
	for _, m := range matches {
		attrs[m[1]] = m[2]
	}
	return attrs
}

func parseItems(body string) []WorkItem {
	var items []WorkItem
	itemRegex := regexp.MustCompile(`(?s)<roadmap-item\s+([^>]+)>(.*?)</roadmap-item>`)
	matches := itemRegex.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		attrs := parseAttributes(m[1])
		item := WorkItem{
			ID:          attrs["id"],
			Package:     attrs["package"],
			Status:      ItemStatus(attrs["status"]),
			Description: strings.TrimSpace(m[2]),
		}
		if deps, ok := attrs["depends_on"]; ok && deps != "" {
			item.DependsOn = strings.Split(deps, ",")
		}
		if priority, ok := attrs["priority"]; ok && priority != "" {
			item.Priority = priority
		}
		items = append(items, item)
	}
	return items
}

// SaveRoadmap atomically writes a Roadmap to a JSON file (write tmp + rename).
func SaveRoadmap(path string, rm *Roadmap) error {
	var data []byte
	var err error

	if strings.HasSuffix(strings.ToLower(path), ".md") || strings.HasSuffix(strings.ToLower(path), ".markdown") {
		data = []byte(RenderMarkdown(rm))
	} else {
		data, err = json.MarshalIndent(rm, "", "  ")
		if err != nil {
			return fmt.Errorf("roadmap: marshal: %w", err)
		}
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
			if len(item.DependsOn) > 0 {
				attrs += fmt.Sprintf(" depends_on=%q", strings.Join(item.DependsOn, ","))
			}
			if item.Priority != "" {
				attrs += fmt.Sprintf(" priority=%q", item.Priority)
			}
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
			if len(item.DependsOn) > 0 {
				attrs += fmt.Sprintf(" depends_on=%q", strings.Join(item.DependsOn, ","))
			}
			if item.Priority != "" {
				attrs += fmt.Sprintf(" priority=%q", item.Priority)
			}
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

