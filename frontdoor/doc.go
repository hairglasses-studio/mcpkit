// Package frontdoor ships an opinionated, discovery-first tool surface for
// downstream MCP servers built on mcpkit.
//
// A front-door module mounts four tools on top of an existing
// registry.ToolRegistry:
//
//   - tool_catalog  — paginated listing of every registered tool with minimal
//     metadata (name, description, category, tags, write/deprecated flags).
//   - tool_search   — fuzzy search over tool name, tags, category, and
//     description, powered by registry.SearchTools.
//   - tool_schema   — full input/output schema and metadata for a single tool.
//   - server_health — lifecycle status, uptime, and tool-inventory counts.
//
// The goal is to give downstream servers a consistent explorer UX without
// re-implementing discovery plumbing on top of every new repo's registry.
// Large-surface servers (50+ tools) can register frontdoor early and then
// defer the rest of their catalog with registry.RegisterDeferredModule.
//
// # Usage
//
//	reg := registry.NewToolRegistry()
//	// ... register your own modules ...
//	reg.RegisterModule(frontdoor.New(reg,
//	    frontdoor.WithPrefix("myapp_"),
//	    frontdoor.WithHealthChecker(checker),
//	))
//
// With WithPrefix("myapp_"), the tools are exposed as myapp_tool_catalog,
// myapp_tool_search, myapp_tool_schema, and myapp_server_health. When the
// prefix is empty the tools keep their bare names.
//
// # Health integration
//
// If WithHealthChecker is passed, server_health mirrors the checker's
// lifecycle status and uptime. Otherwise it reports "ok" with the registry's
// current tool and module counts.
package frontdoor
