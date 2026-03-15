package research_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/research"
)

func ExampleNewModule() {
	m := research.NewModule()

	fmt.Println(m.Name())
	fmt.Println(m.Description())
	// Output:
	// research
	// MCP ecosystem monitoring and viability assessment tools
}

func ExampleModule_Tools() {
	m := research.NewModule()

	tools := m.Tools()
	fmt.Println(len(tools))
	// Output:
	// 7
}
