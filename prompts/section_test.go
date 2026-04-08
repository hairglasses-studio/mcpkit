//go:build !official_sdk

package prompts

import "testing"

func TestSectionedPrompt_Empty(t *testing.T) {
	sp := NewSectionedPrompt()
	prompt, offset := sp.Build()
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %q", prompt)
	}
	if offset != 0 {
		t.Fatalf("expected offset 0, got %d", offset)
	}
}

func TestSectionedPrompt_AllCacheable(t *testing.T) {
	sp := NewSectionedPrompt()
	sp.Add(Section{Name: "env", Content: "You are Claude.", Cacheable: true})
	sp.Add(Section{Name: "tools", Content: "Available tools: Read, Write.", Cacheable: true})
	prompt, offset := sp.Build()
	if offset != len(prompt) {
		t.Fatalf("all cacheable: expected offset=%d (len), got %d", len(prompt), offset)
	}
}

func TestSectionedPrompt_MixedCacheable(t *testing.T) {
	sp := NewSectionedPrompt()
	sp.Add(Section{Name: "env", Content: "You are Claude.", Cacheable: true})
	sp.Add(Section{Name: "cwd", Content: "CWD: /tmp", Cacheable: false})
	sp.Add(Section{Name: "task", Content: "Fix the bug.", Cacheable: false})
	prompt, offset := sp.Build()

	// Boundary should be right after "You are Claude." + newline separator
	expected := len("You are Claude.") + 1 // +1 for the \n before "CWD: /tmp"
	if offset != expected {
		t.Fatalf("expected boundary offset %d, got %d (prompt=%q)", expected, offset, prompt)
	}
	if len(prompt) == 0 {
		t.Fatal("expected non-empty prompt")
	}
}

func TestSectionedPrompt_AllNonCacheable(t *testing.T) {
	sp := NewSectionedPrompt()
	sp.Add(Section{Name: "dynamic", Content: "hello", Cacheable: false})
	_, offset := sp.Build()
	if offset != 0 {
		t.Fatalf("all non-cacheable: expected offset=0, got %d", offset)
	}
}

func TestSectionedPrompt_Sections(t *testing.T) {
	sp := NewSectionedPrompt()
	sp.Add(Section{Name: "a", Content: "A"})
	sp.Add(Section{Name: "b", Content: "B"})
	sections := sp.Sections()
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	// Verify it's a copy.
	sections[0].Name = "modified"
	if sp.sections[0].Name == "modified" {
		t.Fatal("Sections() should return a copy")
	}
}
