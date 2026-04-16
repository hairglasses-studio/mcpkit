package vuln_test

import (
	"fmt"

	"github.com/hairglasses-studio/mcpkit/vuln"
)

func ExampleNewModule() {
	m := vuln.NewModule()

	fmt.Println(m.Name())
	fmt.Println(m.Description())
	// Output:
	// vuln
	// Go module security scanning tools: govulncheck integration and OSV API queries
}

func ExampleModule_Tools() {
	m := vuln.NewModule()

	tools := m.Tools()
	fmt.Println(len(tools))
	// Output:
	// 2
}

func ExampleNewScanner() {
	s := vuln.NewScanner()
	fmt.Println(s != nil)
	// Output:
	// true
}

func ExampleNewOSVClient() {
	c := vuln.NewOSVClient()
	fmt.Println(c != nil)
	// Output:
	// true
}
