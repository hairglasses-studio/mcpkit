# mcpkit Skill Front Doors

Generated from `.agents/skills/surface.yaml` and the canonical skill sources.

## Priority Order

| Priority | Front door | Skill | Aliases | Summary |
| --- | --- | --- | --- | --- |
| high | Framework Mapping | `mcpkit` | fix-issue | Inspect package boundaries, dependency layers, and downstream impact before proposing framework changes. |
| high | Tool Scaffolding | `mcpkit` | mcp-tool-scaffold, new-tool | Route new tool and handler work through the canonical module, handler, and registration patterns. |
| medium | Package Testing | `mcpkit` | test-package | Start with the narrowest package-level checks, then expand to broader framework validation only when justified. |
| medium | Issue Work | `mcpkit` | fix-issue | Separate the fix plan, compatibility risk, and verification scope before changing framework behavior. |
| medium | Go API Reference | `mcpkit-go` | - | Use the typed Go reference skill when the work is about handler signatures, registries, middleware, or package contracts. |
| low | Go Conventions Review | `go-conventions` | - | Use the shared Go conventions skill for repo-wide review, style, and implementation hygiene. |

## Skill Surface

| Skill | Claude aliases |
| --- | --- |
| `mcpkit` | `fix-issue`, `mcp-tool-scaffold`, `new-tool`, `test-package` |
| `mcpkit-go` | - |
| `go-conventions` | - |
