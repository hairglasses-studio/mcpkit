package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type skillAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type skillSpec struct {
	Name                   string       `json:"name"`
	ClaudeIncludeCanonical bool         `json:"claude_include_canonical,omitempty"`
	ExportPlugin           bool         `json:"export_plugin,omitempty"`
	ClaudeAliases          []skillAlias `json:"claude_aliases,omitempty"`
}

type frontDoorSpec struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Skill    string   `json:"skill"`
	Priority string   `json:"priority"`
	Summary  string   `json:"summary"`
	Aliases  []string `json:"aliases,omitempty"`
}

type surfaceConfig struct {
	Version    int             `json:"version"`
	Skills     []skillSpec     `json:"skills"`
	FrontDoors []frontDoorSpec `json:"front_doors,omitempty"`
}

type docFile struct {
	Path    string
	Content []byte
}

type frontDoorDoc struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Skill    string   `json:"skill"`
	Priority string   `json:"priority"`
	Summary  string   `json:"summary"`
	Aliases  []string `json:"aliases,omitempty"`
}

type surfaceDoc struct {
	Version    int            `json:"version"`
	Skills     []skillSpec    `json:"skills"`
	FrontDoors []frontDoorDoc `json:"front_doors"`
}

func main() {
	check := flag.Bool("check", false, "verify checked-in skill surfaces are up to date instead of rewriting them")
	flag.Parse()

	repoRoot := "."
	if *check {
		if err := checkOutputs(repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "genskillsurface: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := writeOutputs(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "genskillsurface: %v\n", err)
		os.Exit(1)
	}
}

func writeOutputs(repoRoot string) error {
	files, err := generateOutputs(repoRoot)
	if err != nil {
		return err
	}
	for _, f := range files {
		path := filepath.Join(repoRoot, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func checkOutputs(repoRoot string) error {
	files, err := generateOutputs(repoRoot)
	if err != nil {
		return err
	}
	for _, f := range files {
		path := filepath.Join(repoRoot, f.Path)
		actual, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !bytes.Equal(actual, f.Content) {
			return fmt.Errorf("checked-in skill surface drift detected in %s; run `go run ./tools/genskillsurface`", f.Path)
		}
	}
	return nil
}

func generateOutputs(repoRoot string) ([]docFile, error) {
	cfg, err := loadSurfaceConfig(repoRoot)
	if err != nil {
		return nil, err
	}

	files := make([]docFile, 0)
	for _, skill := range cfg.Skills {
		canonicalPath := filepath.Join(repoRoot, ".agents", "skills", skill.Name, "SKILL.md")
		frontMatter, body, err := splitFrontMatterFile(canonicalPath)
		if err != nil {
			return nil, err
		}

		files = append(files, docFile{
			Path: filepath.Join(".claude", "skills", skill.Name, "SKILL.md"),
			Content: buildMirror(skill.Name, frontMatter, body,
				fmt.Sprintf("Compatibility mirror of the canonical `%s` skill.", skill.Name)),
		})

		refFiles, err := collectReferences(repoRoot, skill.Name)
		if err != nil {
			return nil, err
		}
		files = append(files, refFiles...)

		for _, alias := range skill.ClaudeAliases {
			files = append(files, docFile{
				Path:    filepath.Join(".claude", "skills", alias.Name, "SKILL.md"),
				Content: buildAlias(alias, body, skill.Name),
			})
			aliasRefs, err := collectAliasReferences(repoRoot, skill.Name, alias.Name)
			if err != nil {
				return nil, err
			}
			files = append(files, aliasRefs...)
		}
	}

	frontDoors := buildFrontDoorDocs(cfg)
	md, err := renderFrontDoorMarkdown(cfg, frontDoors)
	if err != nil {
		return nil, err
	}
	files = append(files, docFile{
		Path:    filepath.Join("docs", "SKILL-FRONT-DOORS.md"),
		Content: []byte(md),
	})

	doc := surfaceDoc{
		Version:    cfg.Version,
		Skills:     cfg.Skills,
		FrontDoors: frontDoors,
	}
	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal front door json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')
	files = append(files, docFile{
		Path:    filepath.Join("docs", "skill-front-doors.json"),
		Content: jsonBytes,
	})

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func loadSurfaceConfig(repoRoot string) (surfaceConfig, error) {
	var cfg surfaceConfig
	data, err := os.ReadFile(filepath.Join(repoRoot, ".agents", "skills", "surface.yaml"))
	if err != nil {
		return cfg, fmt.Errorf("read surface config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse surface config: %w", err)
	}
	return cfg, nil
}

func splitFrontMatterFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return "", "", fmt.Errorf("%s: missing front matter", path)
	}
	rest := text[len("---\n"):]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", "", fmt.Errorf("%s: malformed front matter", path)
	}
	frontMatter := text[:len("---\n")+idx+len("\n---\n")]
	body := strings.TrimLeft(rest[idx+len("\n---\n"):], "\n")
	return frontMatter, body, nil
}

func buildMirror(skillName, frontMatter, body, note string) []byte {
	var b strings.Builder
	b.WriteString(frontMatter)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("<!-- GENERATED BY hg-skill-surface-sync.sh FROM .agents/skills/%s/SKILL.md; DO NOT EDIT -->\n\n", skillName))
	b.WriteString(note)
	b.WriteString("\n\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func buildAlias(alias skillAlias, body, canonicalName string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", alias.Name))
	b.WriteString(fmt.Sprintf("description: '%s'\n", alias.Description))
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("<!-- GENERATED BY hg-skill-surface-sync.sh FROM .agents/skills/%s/SKILL.md; DO NOT EDIT -->\n\n", canonicalName))
	b.WriteString(fmt.Sprintf("Compatibility alias for the canonical `%s` skill.\n\n", canonicalName))
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func collectReferences(repoRoot, skillName string) ([]docFile, error) {
	srcRoot := filepath.Join(repoRoot, ".agents", "skills", skillName, "references")
	return collectReferenceTree(srcRoot, filepath.Join(".claude", "skills", skillName, "references"))
}

func collectAliasReferences(repoRoot, canonicalSkill, alias string) ([]docFile, error) {
	srcRoot := filepath.Join(repoRoot, ".agents", "skills", canonicalSkill, "references")
	return collectReferenceTree(srcRoot, filepath.Join(".claude", "skills", alias, "references"))
}

func collectReferenceTree(srcRoot, dstRoot string) ([]docFile, error) {
	if _, err := os.Stat(srcRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", srcRoot, err)
	}
	var files []docFile
	err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		files = append(files, docFile{
			Path:    filepath.Join(dstRoot, rel),
			Content: data,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", srcRoot, err)
	}
	return files, nil
}

func buildFrontDoorDocs(cfg surfaceConfig) []frontDoorDoc {
	docs := make([]frontDoorDoc, 0, len(cfg.FrontDoors))
	for _, fd := range cfg.FrontDoors {
		doc := frontDoorDoc{
			ID:       fd.ID,
			Title:    fd.Title,
			Skill:    fd.Skill,
			Priority: fd.Priority,
			Summary:  fd.Summary,
			Aliases:  append([]string(nil), fd.Aliases...),
		}
		docs = append(docs, doc)
	}
	return docs
}

func renderFrontDoorMarkdown(cfg surfaceConfig, frontDoors []frontDoorDoc) (string, error) {
	var b strings.Builder
	b.WriteString("# mcpkit Skill Front Doors\n\n")
	b.WriteString("Generated from `.agents/skills/surface.yaml` and the canonical skill sources.\n\n")
	b.WriteString("## Priority Order\n\n")
	b.WriteString("| Priority | Front door | Skill | Aliases | Summary |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, fd := range frontDoors {
		aliases := strings.Join(fd.Aliases, ", ")
		if aliases == "" {
			aliases = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s | %s |\n",
			fd.Priority, fd.Title, fd.Skill, aliases, fd.Summary))
	}

	b.WriteString("\n## Skill Surface\n\n")
	b.WriteString("| Skill | Claude aliases |\n")
	b.WriteString("| --- | --- |\n")
	for _, skill := range cfg.Skills {
		aliases := make([]string, 0, len(skill.ClaudeAliases))
		for _, alias := range skill.ClaudeAliases {
			aliases = append(aliases, fmt.Sprintf("`%s`", alias.Name))
		}
		if len(aliases) == 0 {
			aliases = append(aliases, "-")
		}
		b.WriteString(fmt.Sprintf("| `%s` | %s |\n", skill.Name, strings.Join(aliases, ", ")))
	}

	return b.String(), nil
}
