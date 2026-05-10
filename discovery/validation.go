package discovery

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrValidationFailed is returned when publish metadata does not pass
// validation or schema-compliance checks.
var ErrValidationFailed = errors.New("discovery: validation failed")

// ValidationIssue describes one metadata validation finding.
type ValidationIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationReport captures schema-compliance results for server metadata.
type ValidationReport struct {
	Valid    bool              `json:"valid"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
}

// Err returns ErrValidationFailed when the report is invalid.
func (r ValidationReport) Err() error {
	if r.Valid {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrValidationFailed, r.Summary())
}

// Summary returns a compact human-readable validation summary.
func (r ValidationReport) Summary() string {
	if r.Valid {
		return "valid"
	}
	if len(r.Issues) == 0 {
		return "invalid"
	}
	parts := make([]string, 0, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Path == "" {
			parts = append(parts, issue.Message)
			continue
		}
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	return strings.Join(parts, "; ")
}

// ValidateServerMetadata checks the local MCP Registry/server-card contract
// before publishing. It intentionally validates only fields represented by
// ServerMetadata so callers can run it offline in CI.
func ValidateServerMetadata(meta ServerMetadata) ValidationReport {
	v := metadataValidator{}
	v.required("name", meta.Name)
	v.required("description", meta.Description)
	v.required("version", meta.Version)
	v.required("organization", meta.Organization)
	v.required("repository", meta.Repository)
	v.absoluteURL("repository", meta.Repository, false)
	v.absoluteURL("homepage", meta.Homepage, true)

	if len(meta.Transports) == 0 {
		v.issue("transports", "at least one transport is required")
	}
	for i, transport := range meta.Transports {
		path := fmt.Sprintf("transports[%d]", i)
		switch transport.Type {
		case "stdio":
			v.absoluteURL(path+".url", transport.URL, true)
		case "sse", "streamable-http":
			v.absoluteURL(path+".url", transport.URL, false)
		default:
			v.issue(path+".type", "must be one of stdio, sse, streamable-http")
		}
	}

	for i, tool := range meta.Tools {
		path := fmt.Sprintf("tools[%d]", i)
		v.required(path+".name", tool.Name)
		v.required(path+".description", tool.Description)
	}
	for i, resource := range meta.Resources {
		path := fmt.Sprintf("resources[%d]", i)
		v.required(path+".uri_template", resource.URITemplate)
		v.required(path+".name", resource.Name)
		v.required(path+".description", resource.Description)
	}
	for i, prompt := range meta.Prompts {
		path := fmt.Sprintf("prompts[%d]", i)
		v.required(path+".name", prompt.Name)
		v.required(path+".description", prompt.Description)
	}

	if meta.Auth != nil {
		switch meta.Auth.Type {
		case "none", "bearer":
		case "oauth2":
			v.absoluteURL("auth.token_url", meta.Auth.TokenURL, false)
		default:
			v.issue("auth.type", "must be one of none, bearer, oauth2")
		}
	}

	v.uniqueNonEmpty("tags", meta.Tags)
	v.uniqueNonEmpty("categories", meta.Categories)

	return ValidationReport{
		Valid:    len(v.issues) == 0,
		Issues:   v.issues,
		Warnings: v.warnings,
	}
}

type metadataValidator struct {
	issues   []ValidationIssue
	warnings []ValidationIssue
}

func (v *metadataValidator) required(path, value string) {
	if strings.TrimSpace(value) == "" {
		v.issue(path, "is required")
	}
}

func (v *metadataValidator) absoluteURL(path, value string, optional bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		if !optional {
			v.issue(path, "is required")
		}
		return
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		v.issue(path, "must be an absolute URL")
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		v.issue(path, "must use http or https")
	}
}

func (v *metadataValidator) uniqueNonEmpty(path string, values []string) {
	seen := map[string]struct{}{}
	for i, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			v.issue(fmt.Sprintf("%s[%d]", path, i), "must not be empty")
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			v.warning(fmt.Sprintf("%s[%d]", path, i), "duplicate value")
			continue
		}
		seen[key] = struct{}{}
	}
}

func (v *metadataValidator) issue(path, message string) {
	v.issues = append(v.issues, ValidationIssue{Path: path, Message: message})
}

func (v *metadataValidator) warning(path, message string) {
	v.warnings = append(v.warnings, ValidationIssue{Path: path, Message: message})
}
