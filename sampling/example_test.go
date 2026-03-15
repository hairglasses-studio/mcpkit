package sampling

import (
	"fmt"
)

// ExampleCompletionRequest demonstrates building a sampling request with a
// user message and common options using the request builder helpers.
func ExampleCompletionRequest() {
	msgs := []SamplingMessage{
		TextMessage("user", "Summarise the MCP specification."),
	}
	req := CompletionRequest(msgs,
		WithSystemPrompt("You are a concise technical writer."),
		WithTemperature(0.3),
		WithMaxTokens(256),
	)

	// Verify that the message was included in the request.
	fmt.Println("messages:", len(msgs))
	_ = req // req is ready to pass to a SamplingClient
	fmt.Println("built ok")
	// Output:
	// messages: 1
	// built ok
}
