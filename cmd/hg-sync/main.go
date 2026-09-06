package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type repoEntry struct {
	Name              string `json:"name"`
	Path              string `json:"path"`
	Language          string `json:"language"`
	Lifecycle         string `json:"lifecycle"`
	GoWorkMember      bool   `json:"go_work_member"`
	Visibility        string `json:"visibility"`
	AutomationProfile string `json:"automation_profile"`
	CIProfile         string `json:"ci_profile"`
	ReviewProfile     string `json:"review_profile"`
	MCPSurfaceClass   string `json:"mcp_surface_class"`
	EnvProfile        string `json:"env_profile"`
	MirrorOf          string `json:"mirror_of"`
}

type manifest struct {
	Version int         `json:"version"`
	Repos   []repoEntry `json:"repos"`
}

type options struct {
	root                 string
	manifestPath         string
	versionFile          string
	repoPolicyPath       string
	dryRun               bool
	tidy                 bool
	workspaceCheck       bool
	includeCompatibility bool
	includeDeprecated    bool
	repoFilter           map[string]struct{}
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "hg-sync:", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	home, _ := os.UserHomeDir()
	defaultRoot := filepath.Join(home, "hairglasses-studio")

	var repos string
	var opts options
	flag.StringVar(&opts.root, "root", defaultRoot, "workspace root")
	flag.StringVar(&opts.manifestPath, "manifest", "", "manifest path (default <root>/workspace/manifest.json)")
	flag.StringVar(&opts.versionFile, "version-file", "", "go version file (default <root>/ralphglasses/dotfiles/make/go-version)")
	flag.StringVar(&opts.repoPolicyPath, "repo-policy", "", "repo policy path (default <root>/docs/inventory/repo-catalog.policy.json)")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "show what would change")
	flag.BoolVar(&opts.tidy, "tidy", false, "run go mod tidy after update")
	flag.BoolVar(&opts.workspaceCheck, "workspace-check", false, "run workspace inventory and script drift checks")
	flag.BoolVar(&opts.includeCompatibility, "include-compatibility", false, "include compatibility lifecycle repos")
	flag.BoolVar(&opts.includeDeprecated, "include-deprecated", false, "include deprecated lifecycle repos")
	flag.StringVar(&repos, "repos", "", "comma-separated repo names")
	flag.Parse()

	opts.repoFilter = map[string]struct{}{}
	for _, p := range strings.Split(repos, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			opts.repoFilter[p] = struct{}{}
		}
	}
	if opts.manifestPath == "" {
		opts.manifestPath = filepath.Join(opts.root, "workspace", "manifest.json")
	}
	if opts.versionFile == "" {
		opts.versionFile = filepath.Join(opts.root, "ralphglasses", "dotfiles", "make", "go-version")
	}
	if opts.repoPolicyPath == "" {
		opts.repoPolicyPath = filepath.Join(opts.root, "docs", "inventory", "repo-catalog.policy.json")
	}
	return opts
}

func run(opts options) error {
	if opts.workspaceCheck {
		return runWorkspaceCheck(opts)
	}

	targetVersionRaw, err := os.ReadFile(opts.versionFile)
	if err != nil {
		return fmt.Errorf("read version file: %w", err)
	}
	targetVersion := strings.TrimSpace(string(targetVersionRaw))
	if targetVersion == "" {
		return errors.New("empty go version in version file")
	}

	mf, err := loadManifest(opts.manifestPath)
	if err != nil {
		return err
	}
	repos := selectRepos(mf.Repos, opts)
	changed := 0
	for _, repo := range repos {
		repoPath := repo.Path
		if repoPath == "" {
			// manifest v1 has no path field; repos live at <root>/<name>.
			repoPath = repo.Name
		}
		if !filepath.IsAbs(repoPath) {
			repoPath = filepath.Join(opts.root, repoPath)
		}
		goModPath := filepath.Join(repoPath, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			continue
		}
		before, after, err := rewriteGoVersion(goModPath, targetVersion)
		if err != nil {
			return fmt.Errorf("%s: %w", repo.Name, err)
		}
		if before == after {
			fmt.Printf("%-24s %s (current)\n", repo.Name, before)
			continue
		}
		if opts.dryRun {
			fmt.Printf("%-24s %s -> %s (dry-run)\n", repo.Name, before, after)
			changed++
			continue
		}
		fmt.Printf("%-24s %s -> %s\n", repo.Name, before, after)
		changed++
		if opts.tidy {
			cmd := exec.Command("go", "mod", "tidy")
			cmd.Dir = repoPath
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: go mod tidy: %w", repo.Name, err)
			}
		}
	}
	if opts.dryRun {
		fmt.Printf("repos to update: %d\n", changed)
	} else {
		fmt.Printf("repos updated: %d\n", changed)
	}
	return nil
}

func loadManifest(path string) (manifest, error) {
	var mf manifest
	raw, err := os.ReadFile(path)
	if err != nil {
		return mf, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(raw, &mf); err != nil {
		return mf, fmt.Errorf("parse manifest: %w", err)
	}
	return mf, nil
}

func selectRepos(repos []repoEntry, opts options) []repoEntry {
	out := make([]repoEntry, 0, len(repos))
	for _, r := range repos {
		if len(opts.repoFilter) > 0 {
			if _, ok := opts.repoFilter[r.Name]; !ok {
				continue
			}
		}
		if !opts.includeCompatibility && strings.EqualFold(r.Lifecycle, "compatibility") {
			continue
		}
		if !opts.includeDeprecated && strings.EqualFold(r.Lifecycle, "deprecated") {
			continue
		}
		if isGoRepo(r) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b repoEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func isGoRepo(r repoEntry) bool {
	if r.GoWorkMember {
		return true
	}
	return strings.Contains(strings.ToLower(r.Language), "go")
}

func rewriteGoVersion(path, target string) (string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read go.mod: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	before := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "go ") {
			parts := strings.Fields(trim)
			if len(parts) >= 2 {
				before = parts[1]
			}
			lines[i] = "go " + target
			break
		}
	}
	if before == "" {
		return "", "", errors.New("missing go directive")
	}
	if before == target {
		return before, target, nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return before, "", fmt.Errorf("write go.mod: %w", err)
	}
	return before, target, nil
}

type repoPolicy struct {
	RepoOverrides []repoPolicyOverride `json:"repo_overrides"`
}

type repoPolicyOverride struct {
	Name      string `json:"name"`
	CIProfile string `json:"ci_profile"`
}

func runWorkspaceCheck(opts options) error {
	mf, err := loadManifest(opts.manifestPath)
	if err != nil {
		return err
	}
	if err := validateManifestSchema(mf); err != nil {
		return err
	}
	if err := validateManifestEnums(mf.Repos); err != nil {
		return err
	}

	policy, err := loadRepoPolicy(opts.repoPolicyPath)
	if err != nil {
		return err
	}
	if err := validateRepoPaths(opts.root, mf.Repos); err != nil {
		return err
	}
	if err := validateMirrorRepos(opts.root, mf.Repos); err != nil {
		return err
	}
	if err := validateGoWorkMembership(opts.root, mf.Repos); err != nil {
		return err
	}
	if err := validateCIProfileDrift(mf.Repos, policy.RepoOverrides); err != nil {
		return err
	}
	if err := validateScriptManifestHelpers(opts.root); err != nil {
		return err
	}

	fmt.Println("Workspace manifest and shared automation checks passed")
	return nil
}

func validateManifestSchema(mf manifest) error {
	if mf.Version != 2 {
		return errors.New("workspace manifest must be version=2")
	}
	if len(mf.Repos) == 0 {
		return errors.New("workspace manifest must contain repos")
	}
	seen := map[string]struct{}{}
	for _, repo := range mf.Repos {
		if repo.Name == "" {
			return errors.New("workspace manifest contains repo with empty name")
		}
		if _, ok := seen[repo.Name]; ok {
			return errors.New("workspace manifest must define unique repo names")
		}
		seen[repo.Name] = struct{}{}
	}
	return nil
}

func validateManifestEnums(repos []repoEntry) error {
	visibilityAllowed := map[string]struct{}{"public": {}, "private": {}, "never_publish": {}}
	lifecycleAllowed := map[string]struct{}{"canonical": {}, "mirror": {}, "compatibility": {}, "deprecated": {}}
	automationAllowed := map[string]struct{}{"hub": {}, "application": {}, "standalone_mcp": {}, "tooling": {}, "private_ops": {}}
	ciAllowed := map[string]struct{}{"go_standard": {}, "go_custom": {}, "python_standard": {}, "node_standard": {}, "none": {}}
	reviewAllowed := map[string]struct{}{"public_ai_review": {}, "private_ai_review": {}, "none": {}}
	mcpClassAllowed := map[string]struct{}{"none": {}, "small": {}, "large": {}, "gateway": {}}
	envAllowed := map[string]struct{}{"root_inherit": {}, "root_inherit_oauth_exception": {}, "local_only": {}}

	for _, repo := range repos {
		if _, ok := visibilityAllowed[repo.Visibility]; !ok {
			return fmt.Errorf("workspace manifest contains invalid visibility for %s: %s", repo.Name, repo.Visibility)
		}
		if _, ok := lifecycleAllowed[repo.Lifecycle]; !ok {
			return fmt.Errorf("workspace manifest contains invalid lifecycle for %s: %s", repo.Name, repo.Lifecycle)
		}
		if _, ok := automationAllowed[repo.AutomationProfile]; !ok {
			return fmt.Errorf("workspace manifest contains invalid automation_profile for %s: %s", repo.Name, repo.AutomationProfile)
		}
		if _, ok := ciAllowed[repo.CIProfile]; !ok {
			return fmt.Errorf("workspace manifest contains invalid ci_profile for %s: %s", repo.Name, repo.CIProfile)
		}
		if _, ok := reviewAllowed[repo.ReviewProfile]; !ok {
			return fmt.Errorf("workspace manifest contains invalid review_profile for %s: %s", repo.Name, repo.ReviewProfile)
		}
		if _, ok := mcpClassAllowed[repo.MCPSurfaceClass]; !ok {
			return fmt.Errorf("workspace manifest contains invalid mcp_surface_class for %s: %s", repo.Name, repo.MCPSurfaceClass)
		}
		if _, ok := envAllowed[repo.EnvProfile]; !ok {
			return fmt.Errorf("workspace manifest contains invalid env_profile for %s: %s", repo.Name, repo.EnvProfile)
		}
	}
	return nil
}

func loadRepoPolicy(path string) (repoPolicy, error) {
	var policy repoPolicy
	raw, err := os.ReadFile(path)
	if err != nil {
		return policy, fmt.Errorf("docs repo catalog policy missing: %s", path)
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return policy, fmt.Errorf("docs repo catalog policy parse failed: %w", err)
	}
	if policy.RepoOverrides == nil {
		return policy, errors.New("docs repo catalog policy must define repo_overrides[]")
	}
	return policy, nil
}

func validateRepoPaths(root string, repos []repoEntry) error {
	for _, repo := range repos {
		repoPath := filepath.Join(root, repo.Name)
		if _, err := os.Stat(repoPath); err != nil {
			return fmt.Errorf("manifest repo missing on disk: %s", repo.Name)
		}
	}
	return nil
}

func validateMirrorRepos(root string, repos []repoEntry) error {
	for _, repo := range repos {
		if repo.Lifecycle != "mirror" {
			continue
		}
		if strings.TrimSpace(repo.MirrorOf) == "" {
			return fmt.Errorf("mirror repo missing mirror_of: %s", repo.Name)
		}
		source := filepath.Join(root, repo.MirrorOf)
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("mirror source missing for %s: %s", repo.Name, repo.MirrorOf)
		}
	}
	return nil
}

func validateGoWorkMembership(root string, repos []repoEntry) error {
	goWorkPath := filepath.Join(root, "go.work")
	entries, err := parseGoWorkEntries(goWorkPath)
	if err != nil {
		return err
	}
	manifestGo := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repo.GoWorkMember {
			manifestGo = append(manifestGo, repo.Name)
		}
	}
	sort.Strings(manifestGo)

	missingFromManifest := diffSorted(entries, manifestGo)
	missingFromGoWork := diffSorted(manifestGo, entries)
	if len(missingFromManifest) > 0 {
		return fmt.Errorf("go.work contains repos missing from manifest: %s", strings.Join(missingFromManifest, ", "))
	}
	if len(missingFromGoWork) > 0 {
		return fmt.Errorf("manifest go_work_member=true repos missing from go.work: %s", strings.Join(missingFromGoWork, ", "))
	}
	return nil
}

func parseGoWorkEntries(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read go.work: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	inUse := false
	entries := make([]string, 0, 32)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if !inUse && strings.HasPrefix(trim, "use") && strings.Contains(trim, "(") {
			inUse = true
			continue
		}
		if inUse && trim == ")" {
			break
		}
		if inUse {
			trim = strings.TrimPrefix(trim, "./")
			if trim != "" {
				entries = append(entries, trim)
			}
		}
	}
	sort.Strings(entries)
	return entries, nil
}

func diffSorted(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, v := range right {
		rightSet[v] = struct{}{}
	}
	out := make([]string, 0)
	for _, v := range left {
		if _, ok := rightSet[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func validateCIProfileDrift(repos []repoEntry, overrides []repoPolicyOverride) error {
	manifestProfiles := map[string]string{}
	for _, repo := range repos {
		manifestProfiles[repo.Name] = repo.CIProfile
	}
	drift := make([]string, 0)
	for _, override := range overrides {
		if override.Name == "" || override.CIProfile == "" {
			continue
		}
		manifestProfile, ok := manifestProfiles[override.Name]
		if !ok {
			continue
		}
		if manifestProfile != override.CIProfile {
			drift = append(drift, fmt.Sprintf("%s (manifest=%s, policy=%s)", override.Name, manifestProfile, override.CIProfile))
		}
	}
	if len(drift) > 0 {
		sort.Strings(drift)
		return fmt.Errorf("docs repo catalog policy ci_profile drift detected: %s", strings.Join(drift, "; "))
	}
	return nil
}

func validateScriptManifestHelpers(root string) error {
	scriptsDir := filepath.Join(root, "dotfiles", "scripts")
	scripts := []string{
		"hg-go-sync.sh",
		"hg-new-repo.sh",
		"hg-onboard-repo.sh",
		"hg-repo-profile-sync.sh",
		"hg-workflow-sync.sh",
		"sync-standalone-mcp-repos.sh",
	}
	for _, script := range scripts {
		path := filepath.Join(scriptsDir, script)
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("shared script missing: %s", path)
			}
			return fmt.Errorf("read shared script %s: %w", path, err)
		}
		content := string(raw)
		if strings.Contains(content, `source "$SCRIPT_DIR/lib/hg-workspace.sh"`) {
			continue
		}
		if strings.Contains(content, "mcpkit/cmd/hg-sync/main.go") {
			continue
		}
		return fmt.Errorf("shared script must use manifest helper or delegated hg-sync wrapper: %s", path)
	}
	return nil
}
