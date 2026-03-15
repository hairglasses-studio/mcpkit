//go:build !official_sdk

package handler_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleTextResult() {
	result := handler.TextResult("hello, world")

	text, _ := registry.ExtractTextContent(result.Content[0])
	fmt.Println(text)
	fmt.Println(result.IsError)
	// Output:
	// hello, world
	// false
}

func ExampleCodedErrorResult() {
	err := fmt.Errorf("item not found")
	result := handler.CodedErrorResult(handler.ErrNotFound, err)

	text, _ := registry.ExtractTextContent(result.Content[0])
	fmt.Println(text)
	fmt.Println(result.IsError)
	// Output:
	// [NOT_FOUND] item not found
	// true
}

func ExampleTypedHandler() {
	type Input struct {
		Query string `json:"query" jsonschema:"required,description=Search query"`
	}
	type Output struct {
		Count int `json:"count"`
	}

	td := handler.TypedHandler[Input, Output](
		"search",
		"Search for items",
		func(_ context.Context, in Input) (Output, error) {
			return Output{Count: len(in.Query)}, nil
		},
	)

	fmt.Println(td.Tool.Name)
	fmt.Println(td.Tool.Description)
	// Output:
	// search
	// Search for items
}
