# Developer Experience (DX) Research for mcpkit

Research findings on SDK/toolkit DX best practices, competitive analysis, and recommendations.
Last updated: 2026-03-14.

---

## 1. Onboarding

### What Makes an SDK Easy to Start With

The strongest onboarding experiences share three traits: (1) a working example in under 5 minutes, (2) a single entry point that produces visible output, and (3) progressive disclosure of advanced features.

### Framework Comparison

| Framework | Time to First Tool | Approach | Weakness |
|-----------|-------------------|----------|----------|
| **Vercel AI SDK** | ~2 min | `generateText()` one-liner, streaming chat template | TypeScript-only |
| **OpenAI Agents SDK** | ~3 min | Minimal abstractions, simple agent creation | Locked to OpenAI ecosystem |
| **CrewAI** | ~5 min | Role-based mental model (Researcher, Writer) | Documentation gaps; "frustrating" onboarding per testing reports |
| **LangChain** | ~10 min | Composable chains, extensive integrations | "Thick platform" — forces you to learn the LangChain way; hard to debug |
| **Pydantic AI** | ~3 min | Type-safe, IDE auto-completion | Python-only |
| **mcp-go** | ~5 min | `NewMCPServer()` + `NewTool()` + handler | Minimal guides, no scaffolding CLI |

### Key Patterns

- **Playground/REPL**: Vercel provides an interactive academy course. LangChain has LangSmith playground. Neither mcp-go nor mcpkit offer a playground.
- **Scaffolding CLIs**: The `mcp new tool:calculate` command from MCP Tools (fka.dev) generates a complete project structure with transport config, TypeScript setup, and skeleton implementations. The .NET SDK provides `dotnet new mcpserver` project templates integrated into Visual Studio.
- **Example repos**: The Vercel AI SDK ships starter templates per framework. LangChain maintains a cookbook with 50+ recipes. mcp-go has a single examples directory.

### mcpkit Gap Analysis

- No `mcpkit init` or scaffolding CLI exists
- No interactive getting-started guide
- No example repository beyond test files
- mcptest provides good test infrastructure but is not positioned as an onboarding tool

### Recommendations

1. Create a `mcpkit init` CLI that generates a working MCP server project with one tool, one resource, and one prompt
2. Ship 5-7 standalone example servers (calculator, file browser, database query, API proxy, RAG pipeline, agent loop, multi-tool)
3. Add `Example` functions to key packages for godoc-rendered interactive documentation

---

## 2. API Design

### Go-Specific Patterns

**Functional Options (consensus winner for Go SDKs)**:
- 10+ years of community use; considered idiomatic Go
- Promotes readability, self-documenting code, backward-compatible extension
- Unexported struct fields + constructors = single path to initialization
- No `Build()` footgun (unlike builder pattern where forgetting `Build()` produces zero values)
- Namespace collision is the main downside — mitigated with typed option structs

**Builder Pattern pitfalls in Go**:
- Verbose; each step callable outside constructor context
- Does not protect against misuse (partial builds, wrong order)
- Error-prone compared to functional options

**Current mcpkit approach**: Uses a mix — `registry.Register()` with option-like ToolDefinition structs, `handler.TypedHandler` generics for type-safe params. This is solid but could benefit from more consistent functional options across packages.

### Error Handling Patterns

- Vercel AI SDK: Typed error objects with `.code` and `.message`
- Pydantic AI: Python exceptions with structured error data
- mcpkit: `handler.CodedErrorResult()` with error codes — good pattern, consistent with MCP protocol error semantics

### Recommendations

1. Standardize on functional options for all `New*()` constructors across packages
2. Add `With*()` option functions for common configurations (e.g., `WithMiddleware()`, `WithLogger()`, `WithTimeout()`)
3. Document the "pit of success" — make the easy path the correct path

---

## 3. Documentation

### What Developers Need (2025 Stack Overflow Survey, 49K+ respondents)

- 45% cite "almost right" AI-generated code as their top frustration
- Developers of every age value clear articles and good lists
- Younger engineers reach first for formats with quick feedback and human context
- Advanced/context-dependent questions remain unsolvable by AI — good docs still essential

### Go Documentation Best Practices

- **Godoc**: Parses comments directly from source. First sentence appears in package lists — must be concise and descriptive.
- **Testable Examples**: `Example` functions in `*_test.go` files are compiled, run by `go test`, AND rendered in package docs. This is Go's killer DX feature for SDKs.
- **Deprecation**: Paragraph starting with "Deprecated:" in doc comments. Godoc renders these with strikethrough.
- **Known Bugs**: `BUG(who)` comments surface in a "Bugs" section.

### Documentation Gaps That Frustrate Developers Most

1. **Missing "why"** — docs explain API surface but not when/why to use a feature
2. **No migration guides** — breaking changes without upgrade path
3. **Stale examples** — code that no longer compiles
4. **Missing error documentation** — what errors can a function return and why
5. **No architecture overview** — understanding how packages fit together

### mcpkit Gap Analysis

- CLAUDE.md serves as architecture overview (good)
- No godoc examples (`Example*` functions) in any package
- No cookbook/recipes
- No "when to use X vs Y" decision guides
- Package-level doc comments exist but are minimal

### Recommendations

1. Add `Example*` test functions to every public API — these become both tests and docs
2. Create a "Cookbook" section with recipe-pattern docs (e.g., "Add auth to any tool", "Rate-limit expensive tools", "Test tools with mcptest")
3. Add package-level doc comments explaining the "why" for each package
4. Write decision guides: "registry vs handler", "middleware ordering", "when to use resilience vs custom middleware"

---

## 4. CLI Tools

### Competing Framework CLIs

| Framework | CLI | Key Commands |
|-----------|-----|-------------|
| **MCP Tools** (fka.dev) | `mcp` | `mcp new tool:X`, `mcp new resource:X`, `mcp tools <server>` (introspection) |
| **.NET MCP SDK** | `dotnet new` | `dotnet new mcpserver` project template |
| **Python MCP SDK** | `mcp` | `mcp dev` (development server), `mcp install` (Claude Desktop integration) |
| **LangChain** | `langchain-cli` | `langchain app new`, `langchain serve` |
| **CrewAI** | `crewai` | `crewai create crew`, `crewai run`, `crewai train` |
| **Cobra** (Go CLI) | `cobra-cli` | `cobra-cli init`, `cobra-cli add <command>` |

### MCP Tools Scaffolding (Most Relevant)

The `mcp` CLI from fka.dev provides:
- `mcp new tool:calculate` — generates complete project structure
- `mcp new tool:X resource:Y prompt:Z` — multi-component scaffold
- `--transport=stdio|sse` — transport selection
- Template search path: `./templates/` > `~/.mcpt/templates/` > built-in
- Custom templates for advanced users

### Go-Specific Scaffolding

- **go-scaffold**: General-purpose, Helm-inspired template engine
- **goproj**: `goproj new` / `goproj init` — generates Makefile, Dockerfile, README, LICENSE
- **Autostrada**: Web app scaffold generator with framework integration
- **Cobra Generator**: `cobra-cli init` + `cobra-cli add` for CLI apps

### What a `mcpkit` CLI Could Provide

```
mcpkit init                          # Initialize new MCP server project
mcpkit add tool <name>               # Add tool scaffold to existing project
mcpkit add resource <name>           # Add resource scaffold
mcpkit add prompt <name>             # Add prompt scaffold
mcpkit add middleware <name>         # Add middleware scaffold
mcpkit test                          # Run tests with mcptest helpers
mcpkit inspect <server>              # Introspect running MCP server
mcpkit validate                      # Validate server against MCP spec
```

### Recommendations

1. Build `mcpkit init` as a standalone Go binary (using Cobra)
2. Support `mcpkit add tool/resource/prompt` for incremental scaffolding
3. Use Go embed for templates (no external template directory needed)
4. Generate idiomatic mcpkit code (registry, handler, middleware wiring)

---

## 5. Testing DX

### Industry State of the Art

**Vercel AI SDK** (best-in-class for testing DX):
- `MockLanguageModelV3` — controls model output deterministically
- `MockEmbeddingModelV3` — mock embeddings
- `simulateReadableStream` — configurable stream simulation with delays
- `mockValues` — iterates over arrays of canned responses
- `mockId` — generates incrementing IDs
- Test without calling real LLM providers — eliminates cost, latency, non-determinism

**Sentry AI Testing Framework**:
- Parallel execution support
- Orchestrators for test coordination
- Type definitions and validation logic
- Agent-specific test cases

### mcpkit's mcptest Package

Current capabilities:
- `mcptest.NewServer()` — spins up test MCP server
- Assertion helpers in `mcptest/assert.go`
- HTTP pool for integration testing
- Sampling test helpers

### Gap Analysis vs Alternatives

| Capability | Vercel AI SDK | mcpkit/mcptest | Gap? |
|-----------|--------------|----------------|------|
| Mock model responses | Yes | No (not applicable — server-side) | N/A |
| Test server creation | N/A | Yes | -- |
| Assertion helpers | Basic | Yes | -- |
| Stream simulation | Yes | No | Yes |
| Snapshot testing | No | No | Both |
| Integration test helpers | Limited | Yes (HTTP pool) | -- |
| Sampling mock | N/A | Yes | -- |
| Benchmark helpers | No | No | Both |

### Recommendations

1. Add snapshot testing support (golden file comparison for tool outputs)
2. Add stream simulation helpers for streaming transport testing
3. Add benchmark helpers for middleware performance testing
4. Document testing patterns in a "Testing Cookbook"
5. Consider `mcptest.AssertToolOutput()` for structured output validation

---

## 6. IDE Integration

### Go Tooling Landscape

**Gopls** (official Go language server):
- Provides code completion, diagnostics, signature help, hover docs, refactoring
- Automatic — no SDK-specific setup needed
- Long-running process shares type-checker data across features
- Works with VS Code, GoLand, Neovim, Emacs, Sublime Text

### What SDK Authors Can Do for Better IDE DX

1. **Strong typing** — generics and typed parameters enable better autocomplete. mcpkit's `TypedHandler` already does this well.
2. **Doc comments** — gopls renders these inline. Every exported symbol should have a comment.
3. **Consistent naming** — predictable names enable fuzzy completion. `With*` for options, `New*` for constructors, `Must*` for panic-on-error variants.
4. **Interface compliance** — `var _ Interface = (*Struct)(nil)` compile-time checks help IDE error detection.
5. **Sentinel errors** — named errors enable IDE-assisted error handling.

### mcpkit Strengths

- `TypedHandler` generics give excellent autocomplete for parameter extraction
- Middleware signature is consistent and discoverable
- `handler.Get*Param()` functions are well-named and predictable

### Recommendations

1. Ensure every exported symbol has a doc comment (gopls renders these)
2. Add `var _ Interface = (*Struct)(nil)` checks where applicable
3. Use named sentinel errors instead of ad-hoc `fmt.Errorf`
4. Consider adding Go code generation for tool definitions from OpenAPI/JSON Schema

---

## 7. Migration Paths

### Best Practices from Mature SDKs

**Vercel AI SDK** (exemplary migration approach):
- Published migration guides for every major version (3.4->4.0, 4.x->5.0, 5.x->6.0)
- Provides codemods: automated AST-based transformations that rename functions, update parameters, remove deprecated code
- Warning logger in SDK 6 outputs deprecation warnings at runtime
- Recommendation: migrate between consecutive major versions to avoid conflicts
- Pin experimental API versions exactly (no `^` or `~` ranges)

**Codemod Best Practices** (Martin Fowler):
- Codemods use Abstract Syntax Trees (AST) for precise, consistent transformations
- Should be test-driven: write test cases before implementing transforms
- Use alongside linters for code standardization
- Break complex transforms into smaller composable units
- Always require human review of results
- "Release a tool alongside your update that refactors their code for them"

### Go-Specific Migration Strategies

- **Module versioning**: Go modules enforce semver; `/v2` import paths for major versions
- **Deprecation comments**: `Deprecated:` in doc comments, tooling respects these
- **Compatibility shims**: Keep old function signatures as wrappers around new implementations
- **Build tags**: mcpkit already uses `//go:build !official_sdk` for dual-SDK support — this pattern extends to version migration

### mcpkit Considerations

- Currently pre-1.0, so breaking changes are expected
- Dual-SDK build tags demonstrate good migration infrastructure
- `registry/compat.go` aliases provide a migration buffer
- No formal deprecation policy or migration guides exist yet

### Recommendations

1. Establish a deprecation policy: deprecated APIs live for 2 minor versions before removal
2. Use `Deprecated:` doc comments consistently
3. When reaching v1.0, commit to Go module semver (`/v2` for breaking changes)
4. Consider a `mcpkit migrate` CLI command for future major version transitions
5. Maintain a CHANGELOG.md with breaking changes clearly marked

---

## 8. Community Building

### What Drives SDK Adoption

**Platform strategy** (from DevRel research):
- GitHub for async collaboration, issue tracking, code-linked discussion
- Discord/Slack for real-time Q&A, onboarding help, community energy
- Stack Overflow / Reddit for credibility and discoverability

**Growth signals from competing frameworks**:
- LangChain: 87K GitHub stars, 50K+ production apps, 45% job posting growth Q4 2025
- CrewAI: 15K stars, 300% job posting growth Q4 2025, 5x faster growth rate than LangChain
- mcp-go: Active GitHub Discussions, growing community

**What works**:
- Short workshops and template drops for product adoption
- Structured roadmap sessions for feedback
- Contribution guides with clear "good first issue" labels
- Plugin/extension ecosystem that lets community contribute value

### Plugin Ecosystem Architecture

Successful ecosystems share traits:
- Well-defined extension points (interfaces, not concrete types)
- Registry/marketplace for discovery
- Template/starter projects for plugin authors
- Clear versioning contracts between core and plugins

mcpkit's middleware signature is already an excellent extension point. The `registry.Middleware` type is a clean interface for community contributions.

### Recommendations

1. Create a GitHub Discussions space for mcpkit (not just mcp-go)
2. Add a `CONTRIBUTING.md` with clear guidelines
3. Label issues with "good first issue" and "help wanted"
4. Create a `contrib/` or `community/` directory for community middleware
5. Consider a middleware registry (similar to Express.js middleware ecosystem)

---

## 9. Bootstrap / Init Patterns

### Convention-Over-Configuration in Go

Go projects follow strong conventions:
- `cmd/` for entry points
- `internal/` for private packages
- `pkg/` for public library code (debated but common)
- `Makefile` for build commands
- Standard `go.mod` / `go.sum`

### What `mcpkit init` Should Generate

```
my-server/
  cmd/
    server/
      main.go              # Entry point with server setup
  internal/
    tools/
      hello.go             # Example tool with TypedHandler
      hello_test.go         # Test using mcptest
    resources/
      config.go            # Example resource
    prompts/
      greeting.go          # Example prompt
    middleware/
      logging.go           # Example middleware
  go.mod                   # Module with mcpkit dependency
  go.sum
  Makefile                 # build, test, vet, run targets
  .gitignore
  CLAUDE.md               # Agent-friendly project context
```

### Template Variations

| Template | Use Case | Includes |
|----------|----------|----------|
| `minimal` | Single tool, stdio transport | 1 tool, main.go |
| `standard` | Full server with tools/resources/prompts | All primitives, middleware, tests |
| `api` | HTTP-based MCP server | SSE/streamable HTTP, health checks, auth |
| `agent` | Ralph loop-based agent | Ralph config, sampling, finops budget |

### Interactive Init Flow

```
$ mcpkit init
? Project name: my-mcp-server
? Template: [minimal / standard / api / agent]
? Transport: [stdio / sse / streamable-http]
? Include auth? [y/N]
? Include observability? [y/N]

Created my-mcp-server/
  Run: cd my-mcp-server && go run ./cmd/server
```

### Recommendations

1. Start with `minimal` and `standard` templates
2. Use `go:embed` for templates (single binary distribution)
3. Generate working tests from day one
4. Include a `Makefile` with `make check` matching mcpkit conventions
5. Generate `CLAUDE.md` for AI-assisted development context

---

## Key Takeaways

### Highest-Impact DX Investments for mcpkit

1. **`mcpkit init` CLI** — Biggest gap vs competitors. Every major framework has scaffolding. This is table stakes for adoption.

2. **Godoc Example functions** — Go's unique DX advantage. Examples that are simultaneously documentation, tests, and runnable code. Zero cost to maintain because they break if the API changes.

3. **Cookbook/recipes** — Bridges the gap between API reference and real-world usage. LangChain's cookbook (50+ recipes) is a major adoption driver.

4. **Consistent functional options** — Standardize `With*()` options across all `New*()` constructors. Makes the SDK feel cohesive and predictable.

5. **Community infrastructure** — GitHub Discussions, CONTRIBUTING.md, "good first issue" labels. Low effort, high signal for potential adopters evaluating the project.

### Lower Priority but Valuable

6. Snapshot testing in mcptest
7. Deprecation policy documentation
8. Plugin/middleware registry
9. `mcpkit validate` for spec compliance checking
10. Migration guide infrastructure (for eventual v1.0)

---

## Sources

- [DX Developer Experience Guide (getdx.com)](https://getdx.com/blog/developer-experience/)
- [Ultimate Guide to Developer Experience (Common Room)](https://www.commonroom.io/resources/ultimate-guide-to-developer-experience/)
- [14 AI Agent Frameworks Compared (Softcery)](https://softcery.com/lab/top-14-ai-agent-frameworks-of-2025-a-founders-guide-to-building-smarter-systems)
- [LangChain vs CrewAI (Second Talent)](https://www.secondtalent.com/resources/langchain-vs-crewai/)
- [10 Years of Functional Options in Go (ByteSizeGo)](https://www.bytesizego.com/blog/10-years-functional-options-golang)
- [Understanding the Options Pattern in Go (DEV Community)](https://dev.to/kittipat1413/understanding-the-options-pattern-in-go-390c)
- [MCP Project Scaffolding with MCP Tools (fka.dev)](https://blog.fka.dev/blog/2025-04-03-project-scaffolding-mcp-tools/)
- [Build an MCP Server (modelcontextprotocol.io)](https://modelcontextprotocol.io/docs/develop/build-server)
- [AI SDK Testing (ai-sdk.dev)](https://ai-sdk.dev/docs/ai-sdk-core/testing)
- [Vercel AI SDK Introduction](https://ai-sdk.dev/docs/introduction)
- [AI SDK Migration Guides (ai-sdk.dev)](https://ai-sdk.dev/docs/migration-guides/migration-guide-6-0)
- [Codemods for API Refactoring (Martin Fowler)](https://martinfowler.com/articles/codemods-api-refactoring.html)
- [2025 Stack Overflow Developer Survey](https://survey.stackoverflow.co/2025/)
- [Developers Remain Willing but Reluctant to Use AI (Stack Overflow Blog)](https://stackoverflow.blog/2025/12/29/developers-remain-willing-but-reluctant-to-use-ai-the-2025-developer-survey-results-are-here/)
- [Gopls: The Language Server for Go](https://go.dev/gopls/)
- [Godoc: Documenting Go Code](https://go.dev/blog/godoc)
- [Developer Relations: Building Community (dasroot.net)](https://dasroot.net/posts/2026/02/developer-relations-building-community/)
- [mcp-go Getting Started](https://mcp-go.dev/getting-started/)
- [Go Scaffold CLI (GitHub)](https://github.com/go-scaffold/go-scaffold)
- [JetBrains Developer Ecosystem 2025](https://devecosystem-2025.jetbrains.com/tools-and-trends)
