package vuln

import "time"

// Severity is a coarse vulnerability severity classification derived from
// the OSV database_specific review status and CVSS scoring where available.
type Severity string

const (
	// SeverityUnknown is used when severity cannot be determined.
	SeverityUnknown Severity = "UNKNOWN"
	// SeverityLow indicates a low-severity vulnerability.
	SeverityLow Severity = "LOW"
	// SeverityMedium indicates a medium-severity vulnerability.
	SeverityMedium Severity = "MEDIUM"
	// SeverityHigh indicates a high-severity vulnerability.
	SeverityHigh Severity = "HIGH"
	// SeverityCritical indicates a critical-severity vulnerability.
	SeverityCritical Severity = "CRITICAL"
)

// Vulnerability describes a single known vulnerability affecting a Go module.
type Vulnerability struct {
	// ID is the Go vulnerability database ID (e.g. GO-2024-1234).
	ID string `json:"id"`

	// Aliases contains CVE and GHSA identifiers for this vulnerability.
	Aliases []string `json:"aliases,omitempty"`

	// Summary is a short human-readable description.
	Summary string `json:"summary"`

	// Details provides additional information about the vulnerability.
	Details string `json:"details,omitempty"`

	// Module is the affected Go module path.
	Module string `json:"module"`

	// Version is the currently used version of the affected module.
	Version string `json:"version,omitempty"`

	// FixedVersion is the version in which the vulnerability was fixed.
	// Empty if no fix is available.
	FixedVersion string `json:"fixed_version,omitempty"`

	// Severity is a coarse severity classification.
	Severity Severity `json:"severity"`

	// Published is when the vulnerability was first published.
	Published time.Time `json:"published,omitempty"`

	// Modified is when the vulnerability record was last modified.
	Modified time.Time `json:"modified,omitempty"`

	// AdvisoryURL links to the Go advisory page for this vulnerability.
	AdvisoryURL string `json:"advisory_url,omitempty"`

	// References contains additional URLs (ADVISORY, FIX, REPORT, etc.).
	References []Reference `json:"references,omitempty"`

	// Called indicates whether the vulnerable symbol is reachable from
	// the scanned source (symbol-level finding). False means the vulnerable
	// module is imported but the specific symbol is not called.
	Called bool `json:"called"`
}

// Reference is an external link associated with a vulnerability.
type Reference struct {
	// Type classifies the reference (e.g. "ADVISORY", "FIX", "REPORT").
	Type string `json:"type"`
	// URL is the fully-qualified reference URL.
	URL string `json:"url"`
}

// ScanResult is the output of a govulncheck scan.
type ScanResult struct {
	// Dir is the directory that was scanned.
	Dir string `json:"dir"`

	// GoVersion is the Go toolchain version used for the scan.
	GoVersion string `json:"go_version,omitempty"`

	// ScannerVersion is the govulncheck version used.
	ScannerVersion string `json:"scanner_version,omitempty"`

	// DBLastModified is when the vulnerability database was last updated.
	DBLastModified *time.Time `json:"db_last_modified,omitempty"`

	// Vulnerabilities is the list of discovered vulnerabilities.
	// Empty when the module is clean.
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`

	// Summary is a human-readable scan summary.
	Summary string `json:"summary"`
}

// OSVQueryResult is the output of an OSV API query for a specific module version.
type OSVQueryResult struct {
	// Module is the queried module path.
	Module string `json:"module"`

	// Version is the queried version.
	Version string `json:"version,omitempty"`

	// Vulnerabilities is the list of known vulnerabilities for this module version.
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`

	// Summary is a human-readable result summary.
	Summary string `json:"summary"`
}

// osvAPIResponse is the raw JSON response from the OSV API v1/query endpoint.
type osvAPIResponse struct {
	Vulns []osvEntry `json:"vulns"`
}

// osvEntry is the minimal OSV entry fields we parse from the API response.
type osvEntry struct {
	ID        string     `json:"id"`
	Summary   string     `json:"summary"`
	Details   string     `json:"details"`
	Aliases   []string   `json:"aliases"`
	Published time.Time  `json:"published"`
	Modified  time.Time  `json:"modified"`
	Affected  []affected `json:"affected"`
	References []osvRef  `json:"references"`
	DatabaseSpecific *struct {
		URL string `json:"url"`
	} `json:"database_specific"`
}

type affected struct {
	Pkg    osvPackage `json:"package"`
	Ranges []osvRange `json:"ranges"`
}

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

type osvRange struct {
	Type   string      `json:"type"`
	Events []osvEvent  `json:"events"`
}

type osvEvent struct {
	Introduced string `json:"introduced,omitempty"`
	Fixed      string `json:"fixed,omitempty"`
}

type osvRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
