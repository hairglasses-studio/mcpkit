//go:build !official_sdk

package dispatcher_test

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/dispatcher"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleNew() {
	d := dispatcher.New(dispatcher.Config{
		Workers:   2,
		QueueSize: 100,
	})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	s := d.Stats()
	fmt.Println(s.TotalWorkers)
	// Output:
	// 2
}

func ExampleDispatcher_Submit() {
	d := dispatcher.New(dispatcher.Config{Workers: 1})
	defer d.Shutdown(context.Background()) //nolint:errcheck

	handler := func(_ context.Context, _ registry.CallToolRequest) (*registry.CallToolResult, error) {
		return registry.MakeTextResult("hello from dispatcher"), nil
	}

	job := &dispatcher.Job{
		Name:     "greet",
		Ctx:      context.Background(),
		Handler:  handler,
		Priority: dispatcher.PriorityNormal,
	}

	result, err := d.Submit(context.Background(), job)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	text, _ := registry.ExtractTextContent(result.Content[0])
	fmt.Println(text)
	// Output:
	// hello from dispatcher
}
