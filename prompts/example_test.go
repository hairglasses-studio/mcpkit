//go:build !official_sdk

package prompts_test

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/hairglasses-studio/mcpkit/prompts"
)

// greetingModule is a minimal PromptModule used in examples.
type greetingModule struct{}

func (m *greetingModule) Name() string        { return "greetings" }
func (m *greetingModule) Description() string { return "Greeting prompt module" }
func (m *greetingModule) Prompts() []prompts.PromptDefinition {
	return []prompts.PromptDefinition{
		{
			Prompt:   mcp.NewPrompt("hello", mcp.WithPromptDescription("Say hello")),
			Category: "social",
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return mcp.NewGetPromptResult("Hello", []mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Say hello")),
				}), nil
			},
		},
		{
			Prompt:   mcp.NewPrompt("goodbye", mcp.WithPromptDescription("Say goodbye")),
			Category: "social",
			Handler: func(_ context.Context, _ mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return mcp.NewGetPromptResult("Goodbye", []mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("Say goodbye")),
				}), nil
			},
		},
	}
}

func ExampleNewPromptRegistry() {
	reg := prompts.NewPromptRegistry()
	reg.RegisterModule(&greetingModule{})

	fmt.Println(reg.PromptCount())
	fmt.Println(reg.ModuleCount())
	// Output:
	// 2
	// 1
}

func ExamplePromptRegistry_ListPrompts() {
	reg := prompts.NewPromptRegistry()
	reg.RegisterModule(&greetingModule{})

	names := reg.ListPrompts()
	for _, name := range names {
		fmt.Println(name)
	}
	// Output:
	// goodbye
	// hello
}

func ExamplePromptRegistry_GetPrompt() {
	reg := prompts.NewPromptRegistry()
	reg.RegisterModule(&greetingModule{})

	pd, ok := reg.GetPrompt("hello")
	fmt.Println(ok)
	fmt.Println(pd.Category)
	// Output:
	// true
	// social
}
