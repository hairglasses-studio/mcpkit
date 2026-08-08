package registry

import "testing"

// TestMakePrompt_ArgumentsRoundTrip exercises the pattern real consumers use
// (secretstudios-mcp's internal/surface/prompts.go newPrompt helper):
// MakePrompt(name, description, args...) where args comes from a variadic
// []PromptArgument. Runs unmodified under both build tags (no SDK-specific
// import) — mcp-go's Prompt.Arguments is a value slice and the official
// SDK's is a pointer slice, but MakePrompt papers over that difference so
// the resulting Prompt.Arguments always has len(args) entries with matching
// Name/Description/Required fields on both.
func TestMakePrompt_ArgumentsRoundTrip(t *testing.T) {
	args := []PromptArgument{
		{Name: "view", Description: "the view to render", Required: true},
		{Name: "lane", Description: "optional lane filter"},
	}
	p := MakePrompt("test_prompt", "a test prompt", args...)

	if p.Name != "test_prompt" {
		t.Errorf("Name = %q, want test_prompt", p.Name)
	}
	if p.Description != "a test prompt" {
		t.Errorf("Description = %q, want 'a test prompt'", p.Description)
	}
	if len(p.Arguments) != 2 {
		t.Fatalf("len(Arguments) = %d, want 2", len(p.Arguments))
	}

	// p.Arguments[i].Field works whether Arguments is a value slice
	// (mcp-go) or a pointer slice (official SDK) — Go auto-derefs field
	// access on a pointer element, so this indexing is itself portable.
	if p.Arguments[0].Name != "view" || p.Arguments[0].Description != "the view to render" || !p.Arguments[0].Required {
		t.Errorf("Arguments[0] = %+v", p.Arguments[0])
	}
	if p.Arguments[1].Name != "lane" || p.Arguments[1].Required {
		t.Errorf("Arguments[1] = %+v", p.Arguments[1])
	}
}

func TestMakePrompt_NoArguments(t *testing.T) {
	p := MakePrompt("simple", "no args here")
	if len(p.Arguments) != 0 {
		t.Errorf("len(Arguments) = %d, want 0", len(p.Arguments))
	}
}
