// Package prompts provides a registry for MCP prompt templates.
//
// It mirrors the registry package pattern: thread-safe registration via
// [PromptRegistry], middleware chains applied to [PromptHandlerFunc] handlers,
// module-based organization via [PromptModule], and server integration via
// [PromptRegistry.RegisterWithServer]. List-changed notifications are
// supported when [Config.ListChanged] is enabled, and dynamic registration at
// runtime is available through [DynamicPromptRegistry].
//
// Example:
//
//	reg := prompts.New(prompts.Config{})
//	reg.RegisterPrompt(prompts.PromptDefinition{
//	    Prompt: mcp.Prompt{Name: "summarize", Description: "Summarize text"},
//	    Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
//	        text, _ := req.Params.Arguments["text"]
//	        return &mcp.GetPromptResult{Messages: []mcp.PromptMessage{
//	            {Role: mcp.RoleUser, Content: mcp.NewTextContent("Summarize: " + text)},
//	        }}, nil
//	    },
//	})
//	reg.RegisterWithServer(srv)
package prompts
