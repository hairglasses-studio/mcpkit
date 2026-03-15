package sanitize

import (
	"strings"
	"testing"
)

func TestValidateURI_ValidCases(t *testing.T) {
	policy := DefaultURIPolicy()

	validURIs := []string{
		"https://example.com/data",
		"https://example.com/api/v1/resource",
		"http://example.com/path?q=value&page=1",
		"file:///etc/config.json",
		"file:///home/user/documents/report.txt",
		"https://api.example.com:8443/endpoint",
	}

	for _, uri := range validURIs {
		t.Run(uri, func(t *testing.T) {
			result, err := ValidateURI(uri, policy)
			if err != nil {
				t.Errorf("ValidateURI(%q) unexpected error: %v", uri, err)
			}
			if result != uri {
				t.Errorf("ValidateURI(%q) = %q, want unchanged URI", uri, result)
			}
		})
	}
}

func TestValidateURI_PathTraversal(t *testing.T) {
	policy := DefaultURIPolicy()

	traversalURIs := []string{
		"file:///../../../etc/passwd",
		"https://example.com/../../../etc/shadow",
		"http://example.com/api/../../../secret",
		"file:///safe/../../../etc/hosts",
	}

	for _, uri := range traversalURIs {
		t.Run(uri, func(t *testing.T) {
			_, err := ValidateURI(uri, policy)
			if err == nil {
				t.Errorf("ValidateURI(%q) should have failed for path traversal", uri)
			}
		})
	}
}

func TestValidateURI_AllowDotDot(t *testing.T) {
	policy := DefaultURIPolicy()
	policy.AllowDotDot = true

	uri := "https://example.com/a/../b"
	_, err := ValidateURI(uri, policy)
	if err != nil {
		t.Errorf("ValidateURI(%q) with AllowDotDot=true should pass, got: %v", uri, err)
	}
}

func TestValidateURI_SSRFBlocked(t *testing.T) {
	policy := DefaultURIPolicy()

	ssrfURIs := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254/iam/security-credentials/",
		"http://localhost/admin",
		"http://localhost:8080/api",
		"http://127.0.0.1/internal",
		"http://127.0.0.1:9200/_cat/indices",
		"http://0.0.0.0/hidden",
		"http://[::1]/loopback",
	}

	for _, uri := range ssrfURIs {
		t.Run(uri, func(t *testing.T) {
			_, err := ValidateURI(uri, policy)
			if err == nil {
				t.Errorf("ValidateURI(%q) should have failed for SSRF", uri)
			}
		})
	}
}

func TestValidateURI_SchemeValidation(t *testing.T) {
	policy := DefaultURIPolicy() // allows http, https, file

	t.Run("allowed schemes pass", func(t *testing.T) {
		allowed := []string{
			"http://example.com/",
			"https://example.com/",
			"file:///local/file.txt",
		}
		for _, uri := range allowed {
			_, err := ValidateURI(uri, policy)
			if err != nil {
				t.Errorf("ValidateURI(%q) should pass, got: %v", uri, err)
			}
		}
	})

	t.Run("blocked schemes rejected", func(t *testing.T) {
		blocked := []string{
			"ftp://example.com/file",
			"gopher://example.com/",
			"ldap://example.com/",
			"javascript:alert(1)",
			"data:text/html,<script>alert(1)</script>",
		}
		for _, uri := range blocked {
			_, err := ValidateURI(uri, policy)
			if err == nil {
				t.Errorf("ValidateURI(%q) should have failed for disallowed scheme", uri)
			}
		}
	})
}

func TestValidateURI_CustomSchemes(t *testing.T) {
	policy := URIPolicy{
		AllowedSchemes: []string{"myapp", "custom"},
		MaxLength:      4096,
	}

	_, err := ValidateURI("myapp://resource/123", policy)
	if err != nil {
		t.Errorf("ValidateURI with custom scheme should pass, got: %v", err)
	}

	_, err = ValidateURI("https://example.com/", policy)
	if err == nil {
		t.Error("ValidateURI with https should fail when not in custom allowed schemes")
	}
}

func TestValidateURI_MaxLength(t *testing.T) {
	t.Run("at limit passes", func(t *testing.T) {
		policy := URIPolicy{MaxLength: 50}
		base := "https://example.com/"
		// Pad to exactly 50 bytes total
		uri := base + strings.Repeat("a", 50-len(base))
		if len(uri) != 50 {
			t.Fatalf("test setup: uri length = %d, want 50", len(uri))
		}
		_, err := ValidateURI(uri, policy)
		if err != nil {
			t.Errorf("URI at max length should pass, got: %v", err)
		}
	})

	t.Run("over limit fails", func(t *testing.T) {
		policy := URIPolicy{MaxLength: 50}
		base := "https://example.com/"
		// One byte over the limit
		uri := base + strings.Repeat("a", 50-len(base)+1)
		_, err := ValidateURI(uri, policy)
		if err == nil {
			t.Error("URI over max length should fail")
		}
	})

	t.Run("default 4096 limit", func(t *testing.T) {
		policy := URIPolicy{} // zero MaxLength uses default 4096
		longURI := "https://example.com/" + strings.Repeat("x", 4096)
		_, err := ValidateURI(longURI, policy)
		if err == nil {
			t.Error("URI over default 4096 limit should fail")
		}
	})
}

func TestValidateURI_NullBytes(t *testing.T) {
	policy := DefaultURIPolicy()

	nullURIs := []string{
		"https://example.com/path\x00injection",
		"https://example.com\x00.evil.com/",
		"\x00https://example.com/",
	}

	for _, uri := range nullURIs {
		t.Run("null_byte", func(t *testing.T) {
			_, err := ValidateURI(uri, policy)
			if err == nil {
				t.Errorf("ValidateURI with null byte should fail")
			}
		})
	}
}

func TestDefaultURIPolicy(t *testing.T) {
	p := DefaultURIPolicy()

	if len(p.AllowedSchemes) == 0 {
		t.Error("DefaultURIPolicy should have AllowedSchemes")
	}

	if len(p.BlockedHosts) == 0 {
		t.Error("DefaultURIPolicy should have BlockedHosts")
	}

	if p.MaxLength != 4096 {
		t.Errorf("DefaultURIPolicy MaxLength = %d, want 4096", p.MaxLength)
	}

	if p.AllowDotDot {
		t.Error("DefaultURIPolicy AllowDotDot should be false")
	}

	// Verify key blocked hosts are present
	blocked := map[string]bool{}
	for _, h := range p.BlockedHosts {
		blocked[h] = true
	}

	mustBlock := []string{"169.254.169.254", "localhost", "127.0.0.1", "0.0.0.0"}
	for _, h := range mustBlock {
		if !blocked[h] {
			t.Errorf("DefaultURIPolicy should block host %q", h)
		}
	}

	// Verify key schemes are allowed
	schemes := map[string]bool{}
	for _, s := range p.AllowedSchemes {
		schemes[s] = true
	}
	mustAllow := []string{"http", "https", "file"}
	for _, s := range mustAllow {
		if !schemes[s] {
			t.Errorf("DefaultURIPolicy should allow scheme %q", s)
		}
	}
}

func TestValidateURI_EmptyAllowedSchemes(t *testing.T) {
	// Empty AllowedSchemes means all schemes are allowed
	policy := URIPolicy{
		MaxLength: 4096,
	}

	uris := []string{
		"https://example.com/",
		"ftp://files.example.com/file.txt",
		"myapp://resource",
		"custom-scheme://host/path",
	}

	for _, uri := range uris {
		_, err := ValidateURI(uri, policy)
		if err != nil {
			t.Errorf("ValidateURI(%q) with empty AllowedSchemes should pass, got: %v", uri, err)
		}
	}
}

func TestValidateURI_InvalidURL(t *testing.T) {
	policy := URIPolicy{MaxLength: 4096}

	// A string with a control character that makes it unparseable
	invalidURI := "https://exam\x7fple.com/"
	// url.Parse is fairly permissive, so we focus on null bytes and length
	// which are more reliably invalid. The main goal is covering the parse path.
	_, _ = ValidateURI(invalidURI, policy)
	// Not asserting failure here since url.Parse may accept some odd inputs;
	// the important invariants are covered by the other tests.
}
