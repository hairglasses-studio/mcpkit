package registry

import "testing"

// TestPromptArguments_RoundTrip is the read-back mirror of
// TestMakePrompt_ArgumentsRoundTrip: build via MakePrompt, read back via
// PromptArguments, and confirm the values survive the round trip on both
// tags despite mcp-go's value slice vs the official SDK's pointer slice.
func TestPromptArguments_RoundTrip(t *testing.T) {
	p := MakePrompt("test_prompt", "a test prompt",
		PromptArgument{Name: "view", Description: "the view to render", Required: true},
		PromptArgument{Name: "lane", Description: "optional lane filter"},
	)

	got := PromptArguments(p)
	if len(got) != 2 {
		t.Fatalf("len(PromptArguments) = %d, want 2", len(got))
	}
	if got[0].Name != "view" || got[0].Description != "the view to render" || !got[0].Required {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "lane" || got[1].Required {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestPromptArguments_Nil(t *testing.T) {
	p := MakePrompt("no_args", "no arguments here")
	if got := PromptArguments(p); got != nil {
		t.Errorf("PromptArguments(no-arg prompt) = %v, want nil", got)
	}
}
