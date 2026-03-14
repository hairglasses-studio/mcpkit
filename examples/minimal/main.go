// Command minimal demonstrates the simplest possible mcpkit MCP server.
//
// It registers a single tool using TypedHandler and serves over stdio.
//
// Usage:
//
//	go run ./examples/minimal
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// --- Input/Output types ---

type GreetInput struct {
	Name     string `json:"name" jsonschema:"required,description=Name to greet"`
	Language string `json:"language,omitempty" jsonschema:"description=Greeting language (en or es),enum=en,enum=es"`
}

type GreetOutput struct {
	Message string `json:"message"`
}

type WordCountInput struct {
	Text string `json:"text" jsonschema:"required,description=Text to count words in"`
}

type WordCountOutput struct {
	Words      int `json:"words"`
	Characters int `json:"characters"`
}

// --- Module ---

type UtilModule struct{}

func (m *UtilModule) Name() string        { return "util" }
func (m *UtilModule) Description() string { return "Utility tools" }
func (m *UtilModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		handler.TypedHandler[GreetInput, GreetOutput](
			"greet",
			"Greet a user by name. Supports English and Spanish.",
			func(_ context.Context, input GreetInput) (GreetOutput, error) {
				lang := input.Language
				if lang == "" {
					lang = "en"
				}
				var msg string
				switch lang {
				case "es":
					msg = fmt.Sprintf("¡Hola, %s!", input.Name)
				default:
					msg = fmt.Sprintf("Hello, %s!", input.Name)
				}
				return GreetOutput{Message: msg}, nil
			},
		),
		handler.TypedHandler[WordCountInput, WordCountOutput](
			"word_count",
			"Count words and characters in text.",
			func(_ context.Context, input WordCountInput) (WordCountOutput, error) {
				words := 0
				if input.Text != "" {
					words = len(strings.Fields(input.Text))
				}
				return WordCountOutput{
					Words:      words,
					Characters: len(input.Text),
				}, nil
			},
		),
	}
}

func main() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&UtilModule{})

	s := registry.NewMCPServer("minimal-example", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
