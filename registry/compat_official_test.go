//go:build official_sdk

// compat_official_test.go exercises the small official_sdk-only compat
// additions in compat_official.go that don't already have coverage
// elsewhere: NewTextContent (call-site parity with mcp-go's aliased
// mcp.NewTextContent) and MakeResourceAnnotations (Priority's *float64 vs
// float64 asymmetry). See compat_test.go for the mcp-go-side counterpart
// tests.
package registry

import "testing"

func TestNewTextContent(t *testing.T) {
	c := NewTextContent("hello")
	text, ok := ExtractTextContent(c)
	if !ok {
		t.Fatalf("NewTextContent result is not text content: %T", c)
	}
	if text != "hello" {
		t.Errorf("text = %q, want %q", text, "hello")
	}
}

func TestNewTextContent_MatchesMakeTextContent(t *testing.T) {
	a, aok := ExtractTextContent(NewTextContent("x"))
	b, bok := ExtractTextContent(MakeTextContent("x"))
	if !aok || !bok || a != b {
		t.Errorf("NewTextContent/MakeTextContent diverged: (%q,%v) vs (%q,%v)", a, aok, b, bok)
	}
}

func TestMakeResourceAnnotations_Official(t *testing.T) {
	a := MakeResourceAnnotations([]Role{RoleUser}, 0.5)
	if a == nil {
		t.Fatal("MakeResourceAnnotations returned nil")
	}
	if len(a.Audience) != 1 || a.Audience[0] != RoleUser {
		t.Errorf("Audience = %v, want [user]", a.Audience)
	}
	if a.Priority != 0.5 {
		t.Errorf("Priority = %v, want 0.5", a.Priority)
	}
}
