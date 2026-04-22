package vuln

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hairglasses-studio/mcpkit/client"
)

const osvAPIBase = "https://api.osv.dev/v1"

// OSVClientConfig configures the OSV API client.
type OSVClientConfig struct {
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// BaseURL overrides the OSV API base URL (useful for testing).
	BaseURL string
}

// OSVClient queries the OSV API for Go module vulnerabilities.
type OSVClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewOSVClient creates an OSV API client with the given configuration.
func NewOSVClient(cfg ...OSVClientConfig) *OSVClient {
	c := &OSVClient{
		httpClient: client.Standard(),
		baseURL:    osvAPIBase,
	}
	if len(cfg) > 0 {
		if cfg[0].HTTPClient != nil {
			c.httpClient = cfg[0].HTTPClient
		}
		if cfg[0].BaseURL != "" {
			c.baseURL = strings.TrimRight(cfg[0].BaseURL, "/")
		}
	}
	return c
}

// osvQueryRequest is the OSV API v1/query request body.
type osvQueryRequest struct {
	Version string      `json:"version,omitempty"`
	Package osvQueryPkg `json:"package"`
}

type osvQueryPkg struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// Query queries the OSV API for vulnerabilities affecting the given Go module
// at the specified version. Pass an empty version to query all versions.
func (c *OSVClient) Query(ctx context.Context, modulePath, version string) (OSVQueryResult, error) {
	reqBody := osvQueryRequest{
		Version: version,
		Package: osvQueryPkg{
			Name:      modulePath,
			Ecosystem: "Go",
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return OSVQueryResult{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return OSVQueryResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OSVQueryResult{}, fmt.Errorf("OSV API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return OSVQueryResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return OSVQueryResult{}, fmt.Errorf("OSV API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var apiResp osvAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return OSVQueryResult{}, fmt.Errorf("parse response: %w", err)
	}

	result := OSVQueryResult{
		Module:  modulePath,
		Version: version,
	}

	for _, entry := range apiResp.Vulns {
		e := entry // copy for pointer use
		vuln := Vulnerability{
			ID:        entry.ID,
			Aliases:   entry.Aliases,
			Summary:   entry.Summary,
			Details:   entry.Details,
			Module:    modulePath,
			Version:   version,
			Severity:  classifySeverity(&e),
			Published: entry.Published,
			Modified:  entry.Modified,
		}
		for _, ref := range entry.References {
			vuln.References = append(vuln.References, Reference(ref))
		}
		if entry.DatabaseSpecific != nil {
			vuln.AdvisoryURL = entry.DatabaseSpecific.URL
		}
		// Extract fixed version from the first SEMVER range that has a fix.
		for _, aff := range entry.Affected {
			for _, r := range aff.Ranges {
				if r.Type != "SEMVER" {
					continue
				}
				for _, ev := range r.Events {
					if ev.Fixed != "" {
						vuln.FixedVersion = "v" + ev.Fixed
						break
					}
				}
				if vuln.FixedVersion != "" {
					break
				}
			}
			if vuln.FixedVersion != "" {
				break
			}
		}
		result.Vulnerabilities = append(result.Vulnerabilities, vuln)
	}

	result.Summary = buildOSVSummary(result)
	return result, nil
}

func buildOSVSummary(r OSVQueryResult) string {
	n := len(r.Vulnerabilities)
	if n == 0 {
		ver := r.Version
		if ver == "" {
			ver = "all versions"
		}
		return fmt.Sprintf("No vulnerabilities found for %s@%s.", r.Module, ver)
	}
	return fmt.Sprintf("%d vulnerabilit%s found for %s.", n, pluralSuffix(n), r.Module)
}
