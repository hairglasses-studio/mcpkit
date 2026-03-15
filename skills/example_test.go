package skills

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// ExampleNewSkillRegistry demonstrates registering a skill bundle and activating
// it on demand, which makes the bundled tools available in the DynamicRegistry.
func ExampleNewSkillRegistry() {
	dynReg := registry.NewDynamicRegistry()
	// Pre-register tools that the skill will manage.
	dynReg.AddTool(registry.ToolDefinition{
		Tool:    registry.Tool{Name: "analyze"},
		Handler: func(ctx context.Context, req registry.CallToolRequest) (*registry.CallToolResult, error) { return nil, nil },
	})

	sr := NewSkillRegistry(dynReg)
	sr.Register(Skill{
		Name:        "analysis",
		Description: "Tools for data analysis",
		Tools:       []string{"analyze"},
		Priority:    5,
	})

	// After registration the tool is stashed (not yet in the registry).
	_, inReg := dynReg.GetTool("analyze")
	fmt.Println("before activate:", inReg)

	// Activate the skill to expose its tools.
	_ = sr.Activate(context.Background(), "analysis")

	_, inReg = dynReg.GetTool("analyze")
	fmt.Println("after activate:", inReg)

	active := sr.ActiveSkills()
	fmt.Println("active skills:", active[0])
	// Output:
	// before activate: false
	// after activate: true
	// active skills: analysis
}
