//go:build !official_sdk

package workflow

import (
	"context"
	"maps"

	"github.com/hairglasses-studio/mcpkit/orchestrator"
	"github.com/hairglasses-studio/mcpkit/registry"
	"github.com/hairglasses-studio/mcpkit/sampling"
)

// FromStageFunc adapts an orchestrator.StageFunc into a NodeFunc.
// State.Data is passed as StageInput.Data; StageOutput.Data is merged back.
func FromStageFunc(stage orchestrator.StageFunc) NodeFunc {
	return func(ctx context.Context, state State) (State, error) {
		input := orchestrator.StageInput{
			Data:     state.Data,
			Metadata: state.Metadata,
		}
		output, err := stage(ctx, input)
		if err != nil {
			return state, err
		}
		result := state.Clone()
		maps.Copy(result.Data, output.Data)
		if output.Metadata != nil {
			maps.Copy(result.Metadata, output.Metadata)
		}
		return result, nil
	}
}

// FromToolHandler adapts a registry.ToolHandlerFunc into a NodeFunc.
// The tool name and State.Data are used to construct a CallToolRequest.
// The result text content is stored under "tool_result" in State.Data.
func FromToolHandler(name string, handler registry.ToolHandlerFunc) NodeFunc {
	return func(ctx context.Context, state State) (State, error) {
		args := make(map[string]any, len(state.Data))
		maps.Copy(args, state.Data)
		req := registry.CallToolRequest{}
		req.Params.Arguments = args
		req.Params.Name = name

		result, err := handler(ctx, req)
		if err != nil {
			return state, err
		}

		out := state.Clone()
		// Extract text from result
		if result != nil {
			for _, content := range result.Content {
				if text, ok := registry.ExtractTextContent(content); ok {
					out.Data["tool_result"] = text
					break
				}
			}
		}
		return out, nil
	}
}

// SamplingNode creates a NodeFunc that calls a SamplingClient.
// promptBuilder constructs the request from current state.
// The response text is stored under outputKey in State.Data.
func SamplingNode(client sampling.SamplingClient, promptBuilder func(State) sampling.CreateMessageRequest, outputKey string) NodeFunc {
	if outputKey == "" {
		outputKey = "llm_response"
	}
	return func(ctx context.Context, state State) (State, error) {
		req := promptBuilder(state)
		result, err := client.CreateMessage(ctx, req)
		if err != nil {
			return state, err
		}

		out := state.Clone()
		// Extract text content from result — SamplingMessage.Content is `any`,
		// so we type-assert to registry.TextContent (mcp.TextContent alias).
		if result != nil {
			if tc, ok := result.Content.(registry.TextContent); ok {
				out.Data[outputKey] = tc.Text
			}
		}
		return out, nil
	}
}
