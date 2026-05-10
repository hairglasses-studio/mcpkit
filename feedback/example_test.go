package feedback_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/feedback"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func ExampleModule() {
	sink := feedback.NewMemorySink()
	reg := registry.NewToolRegistry()
	reg.RegisterModule(feedback.NewModule(sink, feedback.WithSource("example-server")))

	fmt.Println(reg.ListTools()[0])
	fmt.Println(reg.ToolCount())
	// Output:
	// feedback_submit
	// 1
}
