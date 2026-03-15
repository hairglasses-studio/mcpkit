package handoff

import (
	"fmt"
)

// ExampleNewHandoffManager demonstrates creating a HandoffManager, registering
// agents, and listing them in alphabetical order.
func ExampleNewHandoffManager() {
	mgr := NewHandoffManager()

	_ = mgr.Register(AgentRef{Name: "researcher", Description: "Performs research tasks", Skills: []string{"research", "summarize"}})
	_ = mgr.Register(AgentRef{Name: "coder", Description: "Writes and reviews code", Skills: []string{"coding"}})

	agents := mgr.ListAgents()
	for _, a := range agents {
		fmt.Println(a.Name)
	}
	// Output:
	// coder
	// researcher
}
